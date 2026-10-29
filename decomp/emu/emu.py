"""Run the original Pool of Radiance (START.EXE + GAME.OVR) in Unicorn, real mode.

The machine: START.bin relocated at LOAD, every overlay of GAME.OVR resident at a
fixed segment with its stub entries patched to JMP FAR (what the Borland overlay
manager does on load, minus the unloading). DOS/BIOS interrupts are served in
Python; the hardware-facing START primitives (prims.py) are intercepted, traced
and skipped. See README.md.
"""
import atexit, json, os, shutil, struct, sys, hashlib, tempfile
from unicorn import *
from unicorn.x86_const import *

HERE = os.path.dirname(os.path.abspath(__file__))
DECOMP = os.path.dirname(HERE)
OUT = os.path.join(DECOMP, 'out')
ROOT = os.path.abspath(os.path.join(DECOMP, '..'))   # repo root
# the DOS install, split between the files the port reads and the rest
GAME_DIRS = [os.path.join(ROOT, 'data', 'dos'), os.path.join(ROOT, 'original')]
SAVES = os.path.join(ROOT, 'data', 'save')

LOAD = 0x1000            # START.bin load segment (PSP = LOAD-0x10)
PSP = LOAD - 0x10
TOPMEM = 0xA000          # PSP:2, end of DOS memory
OVSEG = 0xB000           # overlays from here (~0x3050 paragraphs); A000 stays the EGA window
BIOSSEG = 0xF000         # interrupt stubs: F000:n*4 = INT n; IRET
DS_REL = 0x0C7C          # data segment, relative to the image
ENV = 0x0F00             # environment segment

def seg(rel):            # image-relative segment -> absolute
    return LOAD + rel


class Stop(Exception):
    pass


class Machine:
    def __init__(self, seed=0x12345678, keys=(), scratch=None, verbose=False):
        self.seed = seed & 0xFFFFFFFF
        self.keys = list(keys)          # scripted keys: words (scan<<8|ascii)
        self.trace = []                 # (addr/name, args...)
        self.verbose = verbose
        self.ticks = 0                  # virtual time, milliseconds
        self.idle_polls = 0             # KeyPressed polls with an empty queue since the last key
        self.stop_reason = None
        self.files = {}                 # DOS handle -> python file
        self.next_handle = 5
        self.dta = None
        self.find = None
        if not scratch:  # per process, so parallel runs keep their own SAVE/
            scratch = tempfile.mkdtemp(prefix='porscratch')
            atexit.register(shutil.rmtree, scratch, True)
        self.scratch = scratch
        self.ovr = {o['n']: o for o in json.load(open(os.path.join(OUT, 'overlays.json')))}
        self.ovseg = {}                 # overlay n -> absolute segment of its code
        self.stubseg = {}               # absolute stub segment -> overlay n
        self._prep_scratch()
        mu = self.mu = Uc(UC_ARCH_X86, UC_MODE_16)
        mu.mem_map(0, 0x110000)
        self._load()
        import prims, fb
        self.fb = fb.FB()
        self.prims = prims.install(self)
        mu.hook_add(UC_HOOK_INTR, self._intr)
        self._pin_ub()
        mu.hook_add(UC_HOOK_INSN, self._in, None, 1, 0, UC_X86_INS_IN)
        mu.hook_add(UC_HOOK_INSN, self._out, None, 1, 0, UC_X86_INS_OUT)
        mu.hook_add(UC_HOOK_MEM_READ, self._bios_tick, None, 0x46C, 0x46F)
        mu.hook_add(UC_HOOK_MEM_INVALID, self._badmem)

    # ---------------------------------------------------------------- memory
    def rb(self, a): return self.mu.mem_read(a, 1)[0]
    def rw(self, a): return struct.unpack('<H', self.mu.mem_read(a, 2))[0]
    def rd(self, a): return struct.unpack('<I', self.mu.mem_read(a, 4))[0]
    def wb(self, a, v): self.mu.mem_write(a, bytes([v & 0xFF]))
    def ww(self, a, v): self.mu.mem_write(a, struct.pack('<H', v & 0xFFFF))
    def wd(self, a, v): self.mu.mem_write(a, struct.pack('<I', v & 0xFFFFFFFF))
    def far(self, p): return ((p >> 16) << 4) + (p & 0xFFFF)   # dword far ptr -> linear
    def pstr(self, a):
        n = self.rb(a)
        return bytes(self.mu.mem_read(a + 1, n)).decode('latin1')
    def cstr(self, a):
        b = bytearray()
        while True:
            c = self.rb(a + len(b))
            if c == 0: return b.decode('latin1')
            b.append(c)
    @property
    def ds(self): return seg(DS_REL)
    def dsa(self, ofs): return (self.ds << 4) + ofs
    def reg(self, r): return self.mu.reg_read(r)
    def setreg(self, r, v): self.mu.reg_write(r, v)

    def _prep_scratch(self):
        # writes never touch the repo: SAVE/ is copied fresh for every run,
        # and the game folder is put together from GAME_DIRS (links)
        if os.path.exists(self.scratch):
            shutil.rmtree(self.scratch)
        shutil.copytree(SAVES, os.path.join(self.scratch, 'SAVE'), ignore=shutil.ignore_patterns('saves'))
        self.game = os.path.join(self.scratch, 'game')
        os.makedirs(self.game)
        for d in GAME_DIRS:
            for f in os.listdir(d):
                os.symlink(os.path.join(d, f), os.path.join(self.game, f))
        fix = os.environ.get('POR_SAVES')  # scenario fixture: its files replace those of SAVE/
        if fix:
            for f in os.listdir(fix):
                shutil.copy(os.path.join(fix, f), os.path.join(self.scratch, 'SAVE', f))

    def _load(self):
        mu = self.mu
        img = bytearray(open(os.path.join(OUT, 'START.bin'), 'rb').read())
        for ln in open(os.path.join(OUT, 'START.bin.relocs')):
            if ln.strip():
                r = int(ln, 16)
                struct.pack_into('<H', img, r, (struct.unpack_from('<H', img, r)[0] + LOAD) & 0xFFFF)
        mu.mem_write(LOAD << 4, bytes(img))
        # overlays resident at fixed segments; stubs patched to JMP FAR
        s = OVSEG
        for n, o in sorted(self.ovr.items()):
            code = bytearray(open(os.path.join(OUT, 'ovr', 'ovr%02d.bin' % n), 'rb').read())
            for f in o['fixups']:
                struct.pack_into('<H', code, f, (struct.unpack_from('<H', code, f)[0] + LOAD) & 0xFFFF)
            mu.mem_write(s << 4, bytes(code))
            st = seg(o['seg'])
            self.ovseg[n] = s
            self.stubseg[st] = n
            self.ww((st << 4) + 0x10, s)                  # stub header: load segment
            for i, e in enumerate(o['entries']):
                a = (st << 4) + 0x20 + 5 * i
                mu.mem_write(a, b'\xEA' + struct.pack('<HH', e, s))
            s += (len(code) + 15) >> 4
        assert s <= BIOSSEG, hex(s)
        # IVT -> F000:n*4 = INT n; IRET (Intr() in the DOS unit jumps through the IVT)
        for n in range(256):
            self.ww(n * 4, n * 4)
            self.ww(n * 4 + 2, BIOSSEG)
            mu.mem_write((BIOSSEG << 4) + n * 4, bytes([0xCD, n, 0xCF, 0x90]))
        # BIOS data area: EGA, 80 columns, 25 rows
        self.ww(0x410, 0x0021); self.ww(0x413, 640); self.wb(0x449, 0x0D); self.ww(0x44A, 40)
        self.ww(0x463, 0x3D4); self.wb(0x484, 24); self.wb(0x487, 0x60); self.wb(0x488, 0x09)
        # PSP + environment
        p = PSP << 4
        mu.mem_write(p, b'\xCD\x20'); self.ww(p + 2, TOPMEM); self.ww(p + 0x2C, ENV)
        mu.mem_write(p + 0x80, b'\x00\x0D')
        mu.mem_write(ENV << 4, b'COMSPEC=C:\\COMMAND.COM\x00PATH=C:\\\x00\x00\x01\x00C:\\POOLRAD\\START.EXE\x00')
        self.mcb_top = TOPMEM
        mu.reg_write(UC_X86_REG_DS, PSP); mu.reg_write(UC_X86_REG_ES, PSP)
        mu.reg_write(UC_X86_REG_SS, seg(0x14F2)); mu.reg_write(UC_X86_REG_SP, 0x4000)
        mu.reg_write(UC_X86_REG_CS, seg(0)); mu.reg_write(UC_X86_REG_IP, 6)

    def _pin_ub(self):
        """Pin the original's undefined behaviour to the values the Go port models.
        INT 41h vector (0000:0104) = nil: SelectCharacter's 'O' with who = nil
        reads nil^.next there. TradeMoney's bp-0x131 is never set on the Esc
        path; its stack residue becomes 1 on entry (ovr20:2159). opWho's nil
        selection keeps the old one."""
        self.ww(0x104, 0); self.ww(0x106, 0)
        a = self.lin('ovr20:2159')
        def tm(mu, addr, size, ud):
            self.wb((self.reg(UC_X86_REG_SS) << 4) + self.reg(UC_X86_REG_BP) - 0x131, 1)
        self.mu.hook_add(UC_HOOK_CODE, tm, None, a, a)
        # opWho (ovr04:2B33) passes DS:5D92 itself as SelectCharacter's var:
        # Esc then Enter stores nil, and every later read of the selected
        # character goes through nil into the interrupt vectors. Go keeps the
        # selection the op started with.
        # AddCharacter's count (bp-0x16) is only set by the first add; Esc
        # or Add on a '* ' entry before that tests the stack residue.
        a = self.lin('ovr17:3074')
        def ac(mu, addr, size, ud):
            self.wb((self.reg(UC_X86_REG_SS) << 4) + self.reg(UC_X86_REG_BP) - 0x16, 0)
        self.mu.hook_add(UC_HOOK_CODE, ac, None, a, a)
        # o14_3558's step (bp-0xDD) is only set by N/P/M/T/C; any other key
        # first steps by the stack residue.
        a = self.lin('ovr14:355F')
        def ai(mu, addr, size, ud):
            self.wb((self.reg(UC_X86_REG_SS) << 4) + self.reg(UC_X86_REG_BP) - 0xDD, 0)
        self.mu.hook_add(UC_HOOK_CODE, ai, None, a, a)
        # Locals read before any write (see the Go comments): pinned to the
        # value Go assumes. (hook, bp offset, size, value)
        for name, off, n, v in [
                ('ovr13:1E7D', 2, 2, 0),     # o13_1E77 range (lineOfSight limit)
                ('ovr13:24DF', 1, 1, 0),     # o13_24D9 level bonus
                ('ovr13:2690', 1, 1, 0),     # o13_268A level bonus
                ('ovr04:2AC7', 1, 1, 1),     # opClock operand number
                ('ovr23:2C49', 0xC, 1, 0),   # fxRestore best class
                ('ovr17:3074', 0x14, 1, 0),  # AddCharacter ListMenu show-exit
                ('ovr17:3834', 0x1A, 1, 0),  # o17_382D parent
                ('ovr08:0E12', 2, 2, 0)]:    # ReadVar result of 0x35F, C04B+5.. (never set)
            a = self.lin(name)
            def pin(mu, addr, size, ud, off=off, n=n, v=v):
                l = (self.reg(UC_X86_REG_SS) << 4) + self.reg(UC_X86_REG_BP) - off
                (self.ww if n == 2 else self.wb)(l, v)
            self.mu.hook_add(UC_HOOK_CODE, pin, None, a, a)
        saved = [0]
        def who_in(mu, addr, size, ud): saved[0] = self.who_saved = self.rd(self.dsa(0x5D92))
        def who_out(mu, addr, size, ud):
            if self.rd(self.dsa(0x5D92)) == 0: self.wd(self.dsa(0x5D92), saved[0])
            self.who_saved = 0
        a, b = self.lin('ovr04:2B33'), self.lin('ovr04:2B7A')
        self.mu.hook_add(UC_HOOK_CODE, who_in, None, a, a)
        self.mu.hook_add(UC_HOOK_CODE, who_out, None, b, b)

    # ------------------------------------------------------------- addresses
    def lin(self, name):
        """'0464:0D71' (image-relative START address) or 'ovr13:0233' -> linear."""
        a, b = name.split(':')
        if a.lower().startswith('ovr'):
            return (self.ovseg[int(a[3:])] << 4) + int(b, 16)
        return (seg(int(a, 16)) << 4) + int(b, 16)

    def where(self, cs=None, ip=None):
        cs = self.reg(UC_X86_REG_CS) if cs is None else cs
        ip = self.reg(UC_X86_REG_IP) if ip is None else ip
        for n, s in self.ovseg.items():
            if s <= cs and (cs - s) * 16 + ip < self.ovr[n]['code'] + 16:
                return 'ovr%02d:%04X' % (n, (cs - s) * 16 + ip)
        if LOAD <= cs < LOAD + 0x1000:
            return '%04X:%04X' % (cs - LOAD, ip)
        return '%04X:%04X(abs)' % (cs, ip)

    def caller(self, near=False):
        """Return address of the routine being entered (top of stack) as a name."""
        sp = (self.reg(UC_X86_REG_SS) << 4) + self.reg(UC_X86_REG_SP)
        if near:
            return self.where(self.reg(UC_X86_REG_CS), self.rw(sp))
        return self.where(self.rw(sp + 2), self.rw(sp))

    # -------------------------------------------------------------- tracing
    png_at = {}                         # trace length -> PNG path (run.py --png-at)

    def emit(self, *rec):
        self.trace.append(rec)
        if len(self.trace) in self.png_at:
            self.fb.png(self.png_at[len(self.trace)])
        if self.verbose:
            print(rec, file=sys.stderr)

    def stop(self, why):
        self.stop_reason = why
        self.mu.emu_stop()

    # ------------------------------------------------------------- hardware
    def _in(self, mu, port, size, ud):
        if port == 0x3DA:                     # input status: toggle retrace bits
            self._retrace = getattr(self, '_retrace', 0) ^ 0x09
            return self._retrace
        if port == 0x61:
            return 0
        return 0

    def _out(self, mu, port, size, value, ud):
        pass

    def _bios_tick(self, mu, access, addr, size, value, ud):
        # each read of 0040:006C moves the BIOS tick on (Crt's Delay calibration
        # waits for a change); the value itself is virtual time.
        if addr == 0x46C:
            self.bios_tick = getattr(self, 'bios_tick', 0) + 1
            self.ww(0x46C, self.bios_tick & 0xFFFF)

    def _badmem(self, mu, access, addr, size, value, ud):
        self.stop('bad memory access %x at %s' % (addr, self.where()))
        return False

    # ----------------------------------------------------------- interrupts
    def _intr(self, mu, intno, ud):
        ah = self.reg(UC_X86_REG_AH)
        h = getattr(self, '_int%02x' % intno, None)
        if h is None:
            if intno == 0x3F:
                return self._int3f()
            self.emit('int', '%02X' % intno, ah, self.where())
            return
        h(ah)

    def _int3f(self):
        # stub entry not patched (should not happen): resolve like the overlay manager
        cs, ip = self.reg(UC_X86_REG_CS), self.reg(UC_X86_REG_IP)
        n = self.stubseg.get(cs)
        if n is None:
            return self.stop('INT 3F outside a stub at %s' % self.where())
        ofs = self.rw((cs << 4) + ip)
        self.setreg(UC_X86_REG_CS, self.ovseg[n]); self.setreg(UC_X86_REG_IP, ofs)

    def _flags(self, cf=None, zf=None):
        # the INT hook does not push flags: our stub's IRET pops the caller's FLAGS
        # (Intr path), and a direct INT instruction resumes with live flags. Patch both.
        f = self.reg(UC_X86_REG_EFLAGS)
        sp = (self.reg(UC_X86_REG_SS) << 4) + self.reg(UC_X86_REG_SP)
        def setf(v, bit, val):
            return (v | bit) if val else (v & ~bit)
        if cf is not None: f = setf(f, 1, cf)
        if zf is not None: f = setf(f, 0x40, zf)
        self.setreg(UC_X86_REG_EFLAGS, f)
        cs, ip = self.reg(UC_X86_REG_CS), self.reg(UC_X86_REG_IP)
        if cs == BIOSSEG:                     # inside the IVT stub: fix the pushed FLAGS too
            pf = self.rw(sp + 4)
            if cf is not None: pf = setf(pf, 1, cf)
            if zf is not None: pf = setf(pf, 0x40, zf)
            self.ww(sp + 4, pf)

    def _int10(self, ah):
        al = self.reg(UC_X86_REG_AL)
        if ah == 0x0F:
            self.setreg(UC_X86_REG_AX, 0x2800 | getattr(self, 'vmode', 0x0D)); self.setreg(UC_X86_REG_BH, 0)
        elif ah == 0x00:
            self.vmode = al & 0x7F; self.emit('int10', 'setmode', al)
        elif ah == 0x12:                      # EGA info: 256K, colour
            self.setreg(UC_X86_REG_BX, 0x0003); self.setreg(UC_X86_REG_CX, 0x0009)
        elif ah == 0x1A:
            self.setreg(UC_X86_REG_AL, 0)     # no VGA display combination
        elif ah == 0x10:
            self.emit('int10', 'palette', al, self.reg(UC_X86_REG_BX))
        else:
            self.emit('int10', '%02X' % ah, al)

    def _int11(self, ah): self.setreg(UC_X86_REG_AX, 0x0021)
    def _int12(self, ah): self.setreg(UC_X86_REG_AX, 640)

    def _int16(self, ah):
        # Key arrival model (the Go trace mode does the same): a scripted key becomes
        # visible to KeyPressed only on the poll after one that saw none, so the
        # game's "drain the buffer" loops (0899:0059) cannot swallow typed-ahead keys.
        # A blocking read takes the next key at once.
        if ah in (0x00, 0x10):
            if not self.keys:
                return self.stop('key script exhausted (ReadKey) at %s' % self.where())
            k = self.keys.pop(0)
            self.key_ready = False
            self.idle_polls = 0
            self.emit('INT16', 'ReadKey', k)
            self.setreg(UC_X86_REG_AX, k)
        elif ah in (0x01, 0x11):
            if self.keys and self.key_ready:
                self.emit('INT16', 'KeyPressed', 1)
                self.setreg(UC_X86_REG_AX, self.keys[0]); self._flags(zf=False)
                return
            self.emit('INT16', 'KeyPressed', 0)
            if self.keys:
                self.key_ready = True
            else:
                self.idle_polls += 1
                if self.idle_polls > self.max_idle:
                    return self.stop('key script exhausted (KeyPressed polled %d times) at %s' % (self.idle_polls, self.where()))
            self._flags(zf=True)
        elif ah == 0x02:
            self.setreg(UC_X86_REG_AL, 0)
    key_ready = False
    max_idle = 200

    def _int1a(self, ah):
        if ah == 0:
            t = self.ticks * 182 // 10000
            self.setreg(UC_X86_REG_CX, t >> 16); self.setreg(UC_X86_REG_DX, t & 0xFFFF); self.setreg(UC_X86_REG_AL, 0)

    def _int33(self, ah):
        self.setreg(UC_X86_REG_AX, 0)         # no mouse

    # ---------------------------------------------------------------- DOS
    def _path(self, name, write=False):
        """Map a DOS path onto the assembled game folder; SAVE\\ and every write go to the scratch copy."""
        n = name.replace('/', '\\')
        if len(n) > 1 and n[1] == ':':
            n = n[2:]
        parts = [p for p in n.split('\\') if p and p != '.']
        if parts and parts[0].upper() == 'POOLRAD':
            parts = parts[1:]
        if parts and parts[0].upper() == 'SAVE':
            base = self.scratch
        elif write:
            base = self.scratch
        else:
            base = self.game
        d = base
        for i, p in enumerate(parts):
            hit = None
            if os.path.isdir(d):
                for f in os.listdir(d):
                    if f.upper() == p.upper():
                        hit = f; break
            d = os.path.join(d, hit or p.upper())
        return d

    def _dos_ok(self, ax=None):
        if ax is not None: self.setreg(UC_X86_REG_AX, ax)
        self._flags(cf=False)

    def _dos_err(self, code):
        self.setreg(UC_X86_REG_AX, code); self._flags(cf=True)

    def _int21(self, ah):
        r = self.reg
        al = r(UC_X86_REG_AL)
        dsdx = (r(UC_X86_REG_DS) << 4) + r(UC_X86_REG_DX)
        if ah == 0x30:
            self.setreg(UC_X86_REG_AX, 0x1E03); self.setreg(UC_X86_REG_BX, 0)
        elif ah == 0x35:
            self.setreg(UC_X86_REG_BX, self.rw(al * 4)); self.setreg(UC_X86_REG_ES, self.rw(al * 4 + 2))
        elif ah == 0x25:
            self.ww(al * 4, r(UC_X86_REG_DX)); self.ww(al * 4 + 2, r(UC_X86_REG_DS))
        elif ah == 0x62 or ah == 0x51:
            self.setreg(UC_X86_REG_BX, PSP)
        elif ah == 0x19:
            self.setreg(UC_X86_REG_AL, 2)
        elif ah == 0x0E:
            self.setreg(UC_X86_REG_AL, 3)
        elif ah == 0x47:
            self.mu.mem_write((r(UC_X86_REG_DS) << 4) + r(UC_X86_REG_SI), b'POOLRAD\x00'); self._dos_ok()
        elif ah == 0x3B:
            self._dos_ok()
        elif ah == 0x2A:
            self.setreg(UC_X86_REG_CX, 1989); self.setreg(UC_X86_REG_DX, 0x0101); self.setreg(UC_X86_REG_AL, 0)
        elif ah == 0x2C:
            cs = self.ticks // 10
            self.setreg(UC_X86_REG_CX, ((cs // 360000) % 24) << 8 | (cs // 6000) % 60)
            self.setreg(UC_X86_REG_DX, ((cs // 100) % 60) << 8 | cs % 100)
        elif ah == 0x1A:
            self.dta = dsdx
        elif ah == 0x2F:
            if self.dta is None: self.dta = (PSP << 4) + 0x80
            self.setreg(UC_X86_REG_ES, self.dta >> 4); self.setreg(UC_X86_REG_BX, self.dta & 0xF)
        elif ah == 0x44:
            h = r(UC_X86_REG_BX)
            if al == 0:
                self.setreg(UC_X86_REG_DX, 0x80D3 if h < 5 else 0x0002); self._dos_ok()
            else:
                self._dos_ok(0)
        elif ah in (0x3D, 0x3C, 0x5B):
            name = self.cstr(dsdx)
            write = ah != 0x3D or (al & 3) != 0
            p = self._path(name, write)
            try:
                if ah == 0x3D:
                    if write and not os.path.exists(p) and os.path.exists(self._path(name)):
                        shutil.copy(self._path(name), p)
                    f = open(p, 'r+b' if write else 'rb')
                else:
                    os.makedirs(os.path.dirname(p), exist_ok=True)
                    f = open(p, 'w+b')
            except OSError:
                self.emit('dos', 'open', name, -2)
                return self._dos_err(2)
            h = self.next_handle; self.next_handle += 1
            self.files[h] = f
            self.emit('dos', 'open' if ah == 0x3D else 'create', name, h)
            self._dos_ok(h)
        elif ah == 0x3E:
            f = self.files.pop(r(UC_X86_REG_BX), None)
            if f: f.close(); self.emit('dos', 'close', r(UC_X86_REG_BX))
            self._dos_ok()
        elif ah == 0x3F:
            h, n = r(UC_X86_REG_BX), r(UC_X86_REG_CX)
            f = self.files.get(h)
            if f is None:
                return self._dos_ok(0) if h == 0 else self._dos_err(6)
            pos = f.tell()
            d = f.read(n)
            self.mu.mem_write(dsdx, d)
            self.emit('dos', 'read', h, pos, n, len(d))
            self._dos_ok(len(d))
        elif ah == 0x40:
            h, n = r(UC_X86_REG_BX), r(UC_X86_REG_CX)
            d = bytes(self.mu.mem_read(dsdx, n))
            if h in (1, 2):
                self.emit('stdout', d.decode('latin1'))
                return self._dos_ok(n)
            f = self.files.get(h)
            if f is None:
                return self._dos_err(6)
            self.emit('dos', 'write', h, f.tell(), n, hashlib.md5(d).hexdigest()[:8])
            if n == 0: f.truncate()
            else: f.write(d)
            self._dos_ok(n)
        elif ah == 0x42:
            f = self.files.get(r(UC_X86_REG_BX))
            if f is None:
                return self._dos_err(6)
            off = (r(UC_X86_REG_CX) << 16) | r(UC_X86_REG_DX)
            if off & 0x80000000: off -= 1 << 32
            f.seek(off, al)
            p = f.tell()
            self.setreg(UC_X86_REG_DX, p >> 16); self._dos_ok(p & 0xFFFF)
        elif ah in (0x4E, 0x4F):
            if ah == 0x4E:
                import fnmatch
                pat = self.cstr(dsdx)
                p = self._path(pat)
                d, mask = os.path.dirname(p), os.path.basename(p)
                found = []
                for src in {d, self._path(pat, True) and os.path.dirname(self._path(pat, True))}:
                    if os.path.isdir(src):
                        found += [os.path.join(src, f) for f in sorted(os.listdir(src)) if fnmatch.fnmatch(f.upper(), mask.upper())]
                seen, self.find = set(), []
                for f in found:
                    if os.path.basename(f).upper() not in seen and os.path.isfile(f):
                        seen.add(os.path.basename(f).upper()); self.find.append(f)
                self.emit('dos', 'findfirst', pat, len(self.find))
            if not self.find:
                return self._dos_err(0x12)
            f = self.find.pop(0)
            if self.dta is None: self.dta = (PSP << 4) + 0x80
            nm = os.path.basename(f).upper().encode()[:12] + b'\x00'
            self.wb(self.dta + 0x15, 0x20); self.ww(self.dta + 0x16, 0); self.ww(self.dta + 0x18, 0x1321)
            self.wd(self.dta + 0x1A, os.path.getsize(f))
            self.mu.mem_write(self.dta + 0x1E, nm.ljust(13, b'\x00'))
            self._dos_ok(0)
        elif ah == 0x41:
            p = self._path(self.cstr(dsdx), True)
            self.emit('dos', 'delete', self.cstr(dsdx))
            if os.path.exists(p): os.remove(p); self._dos_ok()
            else: self._dos_err(2)
        elif ah == 0x56:
            a, b = self.cstr(dsdx), self.cstr((r(UC_X86_REG_ES) << 4) + r(UC_X86_REG_DI))
            self.emit('dos', 'rename', a, b)
            try: os.rename(self._path(a, True), self._path(b, True)); self._dos_ok()
            except OSError: self._dos_err(2)
        elif ah in (0x39, 0x3A):              # mkdir / rmdir, in the scratch tree
            p = self._path(self.cstr(dsdx), True)
            self.emit('dos', 'mkdir' if ah == 0x39 else 'rmdir', self.cstr(dsdx))
            try:
                os.mkdir(p) if ah == 0x39 else os.rmdir(p)
                self._dos_ok()
            except OSError:
                self._dos_err(5)
        elif ah == 0x4A:                      # resize: grant up to TOPMEM
            es, want = r(UC_X86_REG_ES), r(UC_X86_REG_BX)
            if es + want > TOPMEM:
                self.setreg(UC_X86_REG_BX, TOPMEM - es); return self._dos_err(8)
            self._dos_ok()
        elif ah == 0x48:
            self.setreg(UC_X86_REG_BX, 0); self._dos_err(8)   # no free DOS memory (TP heap is inside the PSP block)
        elif ah == 0x49:
            self._dos_ok()
        elif ah == 0x4C:
            ds = self.ds << 4
            self.emit('exit', al, '%04X:%04X' % (self.rw(ds + 0x4420), self.rw(ds + 0x441E)))
            self.stop('exit %d (ErrorAddr %04X:%04X)' % (al, self.rw(ds + 0x4420), self.rw(ds + 0x441E)))
        elif ah in (0x0B,):
            self.setreg(UC_X86_REG_AL, 0)
        else:
            self.emit('dos', 'unhandled', '%02X' % ah, al, self.where())

    # ------------------------------------------------------------------ run
    def run(self, max_insns=0):
        cs, ip = self.reg(UC_X86_REG_CS), self.reg(UC_X86_REG_IP)
        try:
            self.mu.emu_start((cs << 4) + ip, 0xFFFFFFFF, 0, max_insns)
        except UcError as e:
            self.stop_reason = self.stop_reason or 'UcError %s at %s' % (e, self.where())
        if self.stop_reason is None:
            self.stop_reason = 'instruction limit %d at %s' % (max_insns, self.where())
        return self.stop_reason

    # ---------------------------------------------------------------- dumps
    def regs(self):
        names = ['AX', 'BX', 'CX', 'DX', 'SI', 'DI', 'BP', 'SP', 'CS', 'DS', 'ES', 'SS', 'IP', 'FLAGS']
        return {n: self.reg(getattr(sys.modules['unicorn.x86_const'], 'UC_X86_REG_' + ('EFLAGS' if n == 'FLAGS' else n))) for n in names}

    DS_SIZE = (0x14F2 - DS_REL) * 16      # DS:0000..DS:8760, the stack segment follows

    def ds_bytes(self):
        return bytes(self.mu.mem_read(self.ds << 4, self.DS_SIZE))

    def party(self, limit=64):
        """Character records (0x11D bytes): list head DS:5D96, next pointer at +0x104."""
        out, p = [], self.rd(self.dsa(0x5D96))
        while p and p != 0xFFFFFFFF and len(out) < limit:
            out.append(p)
            p = self.rd(self.far(p) + 0x104)
        return out

    def regions(self):
        """DS (whole segment), the heap blocks the game state points at, the party."""
        d = self.ds << 4
        out = {'DS': self.ds_bytes().hex()}
        blocks = [(n, self.rd(d + ptr), size) for n, ptr, size in REGIONS]
        # a run cut off inside opWho's prompt after Esc: the pin's selection
        blocks = [(n, p or (getattr(self, 'who_saved', 0) if n == 'selected_char' else 0), z) for n, p, z in blocks]
        blocks += [('char%d' % i, p, 0x11D) for i, p in enumerate(self.party())]
        for name, p, size in blocks:
            if p and p != 0xFFFFFFFF:
                out[name] = {'ptr': '%04X:%04X' % (p >> 16, p & 0xFFFF), 'data': bytes(self.mu.mem_read(self.far(p), size)).hex()}
        return out


# far pointers in DS to heap records worth dumping: (name, DS offset, size)
REGIONS = [
    ('game', 0x49D6, 0x800),     # e.game  (Go: Init)
    ('area', 0x49D2, 0x800),
    ('aux', 0x49DA, 0x400),
    ('ecl', 0x49DE, 0x1E00),
    ('selected_char', 0x5D92, 0x11D),
]


def json_safe(rec):
    return [x if isinstance(x, (int, str, float, type(None))) else (list(x) if isinstance(x, (list, tuple)) else str(x)) for x in rec]
