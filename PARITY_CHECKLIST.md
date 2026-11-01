# Function parity checklist

Routine-by-routine check that each Go port behaves like the original, not
just that it exists. Tick a box only after comparing the Go code with the
listing (`python3 decomp/tools/view.py ovrNN LO HI`), using the Ghidra
decompile as a guide to its structure (see `.claude/skills/ghidra/SKILL.md`). Check:
branches and conditions (signed vs unsigned), loop bounds, every far call
(target and argument order), DS globals read and written, return value, and
quirks kept on purpose.

## Trace-proven scenarios

Retired: the port is a modernized build only. Classic mode (extras off,
the original's exact behaviour) was removed on 2026-10-10, so the Go
engine no longer replays the DOS game's traces; the last commit where all
35 scenarios passed is 651acd2. What follows is the record up to there.

Proof of parity was mechanical: `decomp/emu/both.sh NAME KEYS...` ran the
original under the emulator and the Go engine (then in Classic mode)
on the same key script and seed, and `compare.py` had to report
"identical". A scenario was ticked only when it did.

- [x] `load`: `enter l a` — init, title, credits, party menu, load save A, first 3D view
- [x] `walk`, `area`, `look`, `search`, `view`, `encamp`, `create` (full character creation incl. rolls and portrait) (see decomp/emu/scenarios.txt)
- [x] Later areas from save fixtures (`NAME@FIX` in scenarios.txt, decomp/emu/saves/README.md):
      `area1`..`area29`, one scenario per dungeon/city ECL block (all but 0, 19, 25-27), each
      running its first event square and a few steps; `finale` (Tyranthraxus vote, Quick fight,
      ProgramEight, back to Phlan); `pyramid` (NEWECL to the wilderness, EndgameHook(1));
      `recovered` (LoadGame's EndgameHook(0)). Fixed on the way: ProgramEight's finalAnim
      (frame 1 drawn unmasked first, the loop flips through ovr29:0000) and recolourFrame
      (EGA path: matches against the unchanged picture, random mask per 8-pixel cell).
      Pinned: ReadVar's unset result word for unknown vars (ovr08:0E12, area10's `[C059]`).

- [x] Random play: `decomp/emu/fuzzall.sh FROM TO N` (or `par.sh FROM TO N JOBS`)
      drives both sides with seeded random key scripts (fuzz.py). Seeds 1-1000
      at 1000 keys all pass in one run on the final code (2026-10-09),
      reaching shops, the temple, trading, resting, memorizing, the icon
      editor and combat (placement, moves, attacks, spells and wands, deaths).
      Reads of uninitialised memory in the original are pinned in emu.py
      `_pin_ub` to the values Go models: the INT 41h vector at 0000:0104,
      TradeMoney bp-0x131, AddCharacter bp-0x16/-0x14, o14_3558 bp-0xDD,
      o13_1E77 bp-2, o13_24D9/o13_268A bp-1, opClock bp-1, fxRestore bp-0xC,
      o17_382D bp-0x1A, and opWho's nil selection (Esc then Enter), which
      keeps the selection the op started with.

Run all: `decomp/emu/all.sh`. Records compared: every hooked primitive, BIOS
ReadKey and KeyPressed (repeats collapsed), plus the final game, area, aux,
ECL and party records byte for byte (far pointers masked).

Status so far:
- [x] Existence: every overlay routine found by Ghidra, the prologue scan
      and xref.tsv is named in the Go code (audit 2026-10-09).
- [ ] Behaviour: the per-routine checks below.
- [ ] START.EXE: routines Ghidra finds in START that the Go code does not
      name yet are listed at the end; each is either ported, replaced by Go
      (runtime, DOS, drivers) or not needed.

## ovr02
- [ ] ovr02:0000 `WaitSeconds` (engine.go)
- [ ] ovr02:0206 `Credits` (title.go)
- [ ] ovr02:0626 `Title` (title.go)

## ovr04
- [ ] ovr04:0001 `opExit` (eclops.go)
- [ ] ovr04:00F4 `opGoto` (eclops.go)
- [ ] ovr04:0111 `opGosub` (eclops.go)
- [ ] ovr04:0126 `opCompare` (eclops.go)
- [ ] ovr04:01A3 `opArith` (eclops.go)
- [ ] ovr04:0234 `opRandom` (eclops.go)
- [ ] ovr04:0292 `opSave` (eclops.go)
- [ ] ovr04:02E0 `opLoadCharacter` (eclops.go)
- [ ] ovr04:03B4 `opSetupMonster` (eclops.go)
- [ ] ovr04:0450 `opLoadMonster` (eclops.go)
- [ ] ovr04:0817 `opApproach` (eclops.go)
- [ ] ovr04:0856 `opPicture` (eclops.go)
- [ ] ovr04:0951 `opInputNumber` (eclops.go)
- [ ] ovr04:0994 `opInputString` (eclops.go)
- [ ] ovr04:0A09 `opPrint` (eclops.go)
- [ ] ovr04:0A9F `opReturn` (eclops.go)
- [ ] ovr04:0AED `opCompareAnd` (eclops.go)
- [ ] ovr04:0B52 `opIf` (eclops.go)
- [ ] ovr04:0BD1 `opNewECL` (eclops.go)
- [ ] ovr04:0C74 `opLoadFiles` (eclops.go)
- [ ] ovr04:0DEF `opAndOr` (eclops.go)
- [ ] ovr04:0E5E `opGetTable` (eclops.go)
- [ ] ovr04:0EBC `opSaveTable` (eclops.go)
- [ ] ovr04:0F08 `opVMenu` (eclops.go)
- [ ] ovr04:10D1 `opHMenu` (eclops.go)
- [ ] ovr04:125D `opClearMonsters` (eclops.go)
- [ ] ovr04:12C6 `opPartyStrength` (eclops.go)
- [ ] ovr04:1435 `opCheckParty` (eclops.go)
- [ ] ovr04:1629 `opPartySurprise` (eclops.go)
- [ ] ovr04:16C9 `opSurprise` (eclops.go)
- [ ] ovr04:179B `opCombat` (eclops.go)
- [ ] ovr04:18F8 `opOnGoto` (eclops2.go)
- [ ] ovr04:19B0 `opTreasure` (eclops2.go)
- [ ] ovr04:1DB1 `opRob` (eclops2.go)
- [ ] ovr04:1EF8 `opEncounterMenu` (eclops2.go)
- [ ] ovr04:25CB `opParlay` (eclops2.go)
- [ ] ovr04:266A `opFindItem` (eclops2.go)
- [ ] ovr04:271F `opDelay` (eclops2.go)
- [ ] ovr04:2775 `opDamage` (eclops2.go)
- [ ] ovr04:2AA2 `opRedrawView` (eclops2.go)
- [ ] ovr04:2AC1 `opClock` (eclops2.go)
- [ ] ovr04:2AEB `opNewLine` (eclops2.go)
- [ ] ovr04:2B14 `opClearBox` (eclops2.go)
- [ ] ovr04:2B33 `opWho` (eclops2.go)
- [ ] ovr04:2B7E `opAddNPC` (eclops2.go)
- [ ] ovr04:2C03 `opSpell` (eclops2.go)
- [ ] ovr04:2CCF `opCall` (eclops2.go)
- [ ] ovr04:2DD3 `Encamp` (game.go)
- [ ] ovr04:2E10 `opProgram` (eclops2.go)
- [ ] ovr04:2E7D `opProtection` (eclops2.go)
- [ ] ovr04:3171 `RunECL` (eclops.go)
- [ ] ovr04:31B4 `AreaChange` (game.go)
- [ ] ovr04:32F8 `GameLoop` (game.go)

## ovr05
- [ ] ovr05:0013 `templeNotNeeded` (temple.go)
- [ ] ovr05:00BF `templePay` (temple.go)
- [ ] ovr05:0249 `cureBlindness` (temple.go)
- [ ] ovr05:02ED `cureDisease` (temple.go)
- [ ] ovr05:0401 `cureWounds` (temple.go)
- [ ] ovr05:0510 `raiseDead` (temple.go)
- [ ] ovr05:0751 `neutralizePoison` (temple.go)
- [ ] ovr05:083A `removeCurse` (temple.go)
- [ ] ovr05:0928 `stoneToFlesh` (temple.go)
- [ ] ovr05:09D7 `templeHeal` (temple.go)
- [ ] ovr05:0D09 `Temple` (temple.go)

## ovr06
- [ ] ovr06:0000 `computeExp` (aftercombat.go)
- [ ] ovr06:0355 `giveExp` (aftercombat.go)
- [ ] ovr06:04C9 `combatResult` (aftercombat.go)
- [ ] ovr06:0916 `showResult` (aftercombat.go)
- [ ] ovr06:0B14 `lootList` (aftercombat.go)
- [ ] ovr06:0C06 `lootTake` (aftercombat.go)
- [ ] ovr06:0D38 `takeMenu` (aftercombat.go)
- [ ] ovr06:0ECD `lootMenu` (aftercombat.go)
- [ ] ovr06:11AC `cleanupCombatants` (aftercombat.go)
- [ ] ovr06:12E8 `hiredShare` (aftercombat.go)
- [ ] ovr06:1527 `AfterCombat` (aftercombat.go)

## ovr07
- [ ] ovr07:0030 `shopList` (shop.go)
- [ ] ovr07:0239 `buyItem` (shop.go)
- [ ] ovr07:0361 `shopBuy` (shop.go)
- [ ] ovr07:053E `Shop` (shop.go)

## ovr08
- [ ] ovr08:0000 `FetchArgs` (ecl.go)
- [ ] ovr08:0136 `ArgValue` (ecl.go)
- [ ] ovr08:01B6 `InitAreaScript` (ecl.go)
- [ ] ovr08:0341 `LoadArea` (ecl.go)
- [ ] ovr08:0482 `FreeSteps` (ecl.go)
- [ ] ovr08:0511 `ShowPortrait` (anim.go)
- [ ] ovr08:054E `ShowEncounter` (anim.go)
- [ ] ovr08:06FB `addrKind` (ecl.go)
- [ ] ovr08:07BC `readSpecialGlobal` (ecl.go)
- [ ] ovr08:0A74 `writeSpecialGlobal` (ecl.go)
- [ ] ovr08:0C77 `WriteVar` (ecl.go)
- [ ] ovr08:0E0C `ReadVar` (ecl.go)
- [ ] ovr08:0F53 `WriteStringVar` (ecl.go)
- [ ] ovr08:1153 `decodePackedString` (ecl.go)
- [ ] ovr08:12FB `readStringVar` (ecl.go)
- [ ] ovr08:156D `extractHotkeys` (ecl.go)
- [ ] ovr08:1684 `MenuInput` (ecl.go)
- [ ] ovr08:18EB `CompareWords` (ecl.go)
- [ ] ovr08:1951 `Gosub` (ecl.go)
- [ ] ovr08:19DB `StepForward` (ecl.go)
- [ ] ovr08:1A6F `DuelSetup` (story.go)
- [ ] ovr08:1CC3 `ScaleExp` (ecl.go)
- [ ] ovr08:1DF0 `ItemSave` (ecl.go)
- [ ] ovr08:1EA6 `opArgCount` (ecl.go)
- [ ] ovr08:1FB1 `PartyHpRange` (ecl.go)
- [ ] ovr08:20CC `HitMessage` (ecl.go)

## ovr09
- [ ] ovr09:0000 `combatEnd` (combat.go)
- [ ] ovr09:007A `Combat` (combat.go)
- [ ] ovr09:0131 `nextActor` (combat.go)
- [ ] ovr09:01F6 `takeTurn` (combat.go)
- [ ] ovr09:031A `playerTurn` (combat.go)
- [ ] ovr09:0666 `combatMenu` (combat.go)
- [ ] ovr09:0880 `endRound` (combat.go)
- [ ] ovr09:09DF `moveAttack` (combat.go)
- [ ] ovr09:0D45 `attackTarget` (combat.go)
- [ ] ovr09:0E45 `otherMenu` (combat.go)
- [ ] ovr09:0FFF `bandage` (combat.go)
- [ ] ovr09:10F6 `speedMenu` (combat.go)
- [ ] ovr09:122B `quickOne` (combat.go)

## ovr10
- [ ] ovr10:000F `o10_000F` (combatai.go)
- [ ] ovr10:0203 `aiTryCast` (combatai.go)
- [ ] ovr10:02EB `canCast` (combatai.go)
- [ ] ovr10:03E4 `aiCastFromItem` (combatai.go)
- [ ] ovr10:0545 `aiCastKnown` (combatai.go)
- [ ] ovr10:0675 `aiCanStep` (combatai.go)
- [ ] ovr10:07F2 `aiMoveStep` (combatai.go)
- [ ] ovr10:0B3C `aiAttackLoop` (combatai.go)
- [ ] ovr10:0F4F `aiGiveUp` (combatai.go)
- [ ] ovr10:0FC1 `aiKeys` (combatai.go)
- [ ] ovr10:10D7 `aiFleeCheck` (combatai.go)
- [ ] ovr10:128D `weaponScore` (combatai.go)
- [ ] ovr10:13CA `aiPickWeapon` (combatai.go)

## ovr11
- [ ] ovr11:0000 `floorClutter` (combatsetup.go)
- [ ] ovr11:012B `cellSide` (combatsetup.go)
- [ ] ovr11:01AD `cellEdge` (combatsetup.go)
- [ ] ovr11:0810 `dungeonMap` (combatsetup.go)
- [ ] ovr11:08A4 `terrainFlags` (combatsetup.go)
- [ ] ovr11:0BDE `wildRoad` (combatsetup.go)
- [ ] ovr11:0D13 `wildWater` (combatsetup.go)
- [ ] ovr11:0E9E `wildPlants` (combatsetup.go)
- [ ] ovr11:11B4 `wildMap` (combatsetup.go)
- [ ] ovr11:1244 `combatMapSetup` (combatsetup.go)
- [ ] ovr11:12E1 `combatBlocks` (combatsetup.go)
- [ ] ovr11:1403 `formationEdge` (combatsetup.go)
- [ ] ovr11:1434 `placeAt` (combatsetup.go)
- [ ] ovr11:1577 `placeOne` (combatsetup.go)
- [ ] ovr11:19F5 `placeCombatants` (combatsetup.go)
- [ ] ovr11:1E31 `combatSetup` (combatsetup.go)

## ovr12
- [ ] ovr12:002D `Init` (init.go)

## ovr13
- [ ] ovr13:0000 `blockAffect` (affects1.go)
- [ ] ovr13:001F `addAffectNow` (affects1.go)
- [ ] ovr13:0058 `killWithMessage` (affects1.go)
- [ ] ovr13:010D `o13_010D` (affects1.go)
- [ ] ovr13:011D `o13_011D` (affects1.go)
- [ ] ovr13:013B `o13_013B` (affects1.go)
- [ ] ovr13:017F `o13_017F` (affects1.go)
- [ ] ovr13:01C3 `o13_01C3` (affects1.go)
- [ ] ovr13:0233 `o13_0233` (affects1.go)
- [ ] ovr13:0377 `o13_0377` (affects1.go)
- [ ] ovr13:03AC `o13_03AC` (affects1.go)
- [ ] ovr13:03E1 `o13_03E1` (affects1.go)
- [ ] ovr13:0400 `o13_0400` (affects1.go)
- [ ] ovr13:04BA `o13_04BA` (affects1.go)
- [ ] ovr13:05D6 `o13_05D6` (affects1.go)
- [ ] ovr13:05EB `o13_05EB` (affects1.go)
- [ ] ovr13:0647 `o13_0647` (affects1.go)
- [ ] ovr13:064E `o13_064E` (affects1.go)
- [ ] ovr13:0679 `o13_0679` (affects1.go)
- [ ] ovr13:06E4 `o13_06E4` (affects1.go)
- [ ] ovr13:070F `o13_070F` (affects1.go)
- [ ] ovr13:0775 `o13_0775` (affects1.go)
- [ ] ovr13:07E0 `o13_07E0` (affects1.go)
- [ ] ovr13:0918 `o13_0918` (affects1.go)
- [ ] ovr13:0947 `o13_0947` (affects1.go)
- [ ] ovr13:09C0 `o13_09C0` (affects1.go)
- [ ] ovr13:0A3D `o13_0A3D` (affects1.go)
- [ ] ovr13:0A65 `o13_0A65` (affects1.go)
- [ ] ovr13:0B1B `o13_0B1B` (affects1.go)
- [ ] ovr13:0BAC `o13_0BAC` (affects1.go)
- [ ] ovr13:0BCF `o13_0BCF` (affects1.go)
- [ ] ovr13:0C06 `o13_0C06` (affects1.go)
- [ ] ovr13:0C15 `o13_0C15` (affects1.go)
- [ ] ovr13:0C26 `o13_0C26` (affects1.go)
- [ ] ovr13:0C4B `o13_0C4B` (affects1.go)
- [ ] ovr13:0CC3 `o13_0CC3` (affects1.go)
- [ ] ovr13:0FAC `avoidDistantAttack` (affects1.go)
- [ ] ovr13:101D `readiedWeapon` (affects1.go)
- [ ] ovr13:1086 `o13_1086` (affects1.go)
- [ ] ovr13:10C1 `o13_10C1` (affects1.go)
- [ ] ovr13:10DE `o13_10DE` (affects1.go)
- [ ] ovr13:1168 `o13_1168` (affects1.go)
- [ ] ovr13:11F9 `o13_11F9` (affects1.go)
- [ ] ovr13:1276 `o13_1276` (affects1.go)
- [ ] ovr13:12B2 `o13_12B2` (affects1.go)
- [ ] ovr13:12FB `o13_12FB` (affects1.go)
- [ ] ovr13:133F `o13_133F` (affects1.go)
- [ ] ovr13:1355 `o13_1355` (affects1.go)
- [ ] ovr13:135C `o13_135C` (affects1.go)
- [ ] ovr13:1363 `o13_1363` (affects1.go)
- [ ] ovr13:1399 `o13_1399` (affects1.go)
- [ ] ovr13:1452 `o13_1452` (affects1.go)
- [ ] ovr13:1472 `o13_1472` (affects1.go)
- [ ] ovr13:1490 `o13_1490` (affects1.go)
- [ ] ovr13:14EB `o13_14EB` (affects1.go)
- [ ] ovr13:1545 `poisonAttack` (affects1.go)
- [ ] ovr13:15EB `paralyseTarget` (affects1.go)
- [ ] ovr13:165D `o13_165D` (affects1.go)
- [ ] ovr13:1671 `o13_1671` (affects1.go)
- [ ] ovr13:1685 `o13_1685` (affects1.go)
- [ ] ovr13:1699 `o13_1699` (affects1.go)
- [ ] ovr13:16BB `o13_16BB` (affects1.go)
- [ ] ovr13:16E6 `o13_16E6` (affects1.go)
- [ ] ovr13:170B `o13_170B` (affects1.go)
- [ ] ovr13:171F `o13_171F` (affects1.go)
- [ ] ovr13:1730 `o13_1730` (affects1.go)
- [ ] ovr13:176F `o13_176F` (affects1.go)
- [ ] ovr13:18B5 `o13_18B5` (affects1.go)
- [ ] ovr13:1935 `o13_1935` (affects1.go)
- [ ] ovr13:19D9 `o13_19D9` (affects1.go)
- [ ] ovr13:1A19 `o13_1A19` (affects1.go)
- [ ] ovr13:1A4C `o13_1A4C` (affects1.go)
- [ ] ovr13:1AAA `o13_1AAA` (affects1.go)
- [ ] ovr13:1BA8 `o13_1BA8` (affects1.go)
- [ ] ovr13:1CAD `o13_1CAD` (affects2.go)
- [ ] ovr13:1E77 `o13_1E77` (affects2.go)
- [ ] ovr13:2002 `bestClass` (affects2.go)
- [ ] ovr13:2122 `drainLevels` (affects2.go)
- [ ] ovr13:23A4 `o13_23A4` (affects2.go)
- [ ] ovr13:23C2 `o13_23C2` (affects2.go)
- [ ] ovr13:23EC `o13_23EC` (affects2.go)
- [ ] ovr13:2490 `o13_2490` (affects2.go)
- [ ] ovr13:24D9 `o13_24D9` (affects2.go)
- [ ] ovr13:2556 `o13_2556` (affects2.go)
- [ ] ovr13:25A2 `o13_25A2` (affects2.go)
- [ ] ovr13:25BC `o13_25BC` (affects2.go)
- [ ] ovr13:2610 `o13_2610` (affects2.go)
- [ ] ovr13:264B `o13_264B` (affects2.go)
- [ ] ovr13:268A `o13_268A` (affects2.go)
- [ ] ovr13:2700 `o13_2700` (affects2.go)
- [ ] ovr13:2730 `o13_2730` (affects2.go)
- [ ] ovr13:27D9 `o13_27D9` (affects2.go)
- [ ] ovr13:2814 `o13_2814` (affects2.go)
- [ ] ovr13:2866 `o13_2866` (affects2.go)
- [ ] ovr13:28BF `o13_28BF` (affects2.go)
- [ ] ovr13:2901 `o13_2901` (affects2.go)
- [ ] ovr13:2915 `resistRoll` (affects2.go)
- [ ] ovr13:2976 `o13_2976` (affects2.go)
- [ ] ovr13:2984 `o13_2984` (affects2.go)
- [ ] ovr13:2992 `o13_2992` (affects2.go)
- [ ] ovr13:29B6 `o13_29B6` (affects2.go)
- [ ] ovr13:29CB `o13_29CB` (affects2.go)
- [ ] ovr13:29D9 `o13_29D9` (affects2.go)
- [ ] ovr13:29F0 `o13_29F0` (affects2.go)
- [ ] ovr13:2A11 `o13_2A11` (affects2.go)
- [ ] ovr13:2A28 `o13_2A28` (affects2.go)
- [ ] ovr13:2A6D `o13_2A6D` (affects2.go)
- [ ] ovr13:2A87 `o13_2A87` (affects2.go)
- [ ] ovr13:2AE6 `o13_2AE6` (affects2.go)
- [ ] ovr13:2B20 `o13_2B20` (affects2.go)
- [ ] ovr13:2B63 `o13_2B63` (affects2.go)
- [ ] ovr13:2B7D `o13_2B7D` (affects2.go)
- [ ] ovr13:2BC8 `o13_2BC8` (affects2.go)
- [ ] ovr13:2C1C `o13_2C1C` (affects2.go)
- [ ] ovr13:2D38 `o13_2D38` (affects2.go)
- [ ] ovr13:2D88 `o13_2D88` (affects2.go)
- [ ] ovr13:2DE4 `o13_2DE4` (affects2.go)
- [ ] ovr13:2E08 `o13_2E08` (affects2.go)
- [ ] ovr13:2E37 `o13_2E37` (affects2.go)
- [ ] ovr13:2EDA `o13_2EDA` (affects2.go)
- [ ] ovr13:2F57 `o13_2F57` (affects2.go)
- [ ] ovr13:30A5 `o13_30A5` (affects2.go)
- [ ] ovr13:3134 `o13_3134` (affects2.go)
- [ ] ovr13:317A `o13_317A` (affects2.go)
- [ ] ovr13:319B `o13_319B` (affects2.go)
- [ ] ovr13:31A2 `o13_31A2` (affects2.go)

## ovr14
- [ ] ovr14:0000 `turnSetup` (combatturn1.go)
- [ ] ovr14:0123 `o14_0123` (combatturn1.go)
- [ ] ovr14:0191 `o14_0191` (combatturn1.go)
- [ ] ovr14:02F9 `o14_02F9` (combatturn1.go)
- [ ] ovr14:0627 `o14_0627` (combatturn1.go)
- [ ] ovr14:0712 `o14_0712` (combatturn1.go)
- [ ] ovr14:08B2 `o14_08B2` (combatturn1.go)
- [ ] ovr14:0C6A `o14_0C6A` (combatturn1.go)
- [ ] ovr14:0D23 `o14_0D23` (combatturn1.go)
- [ ] ovr14:0E52 `o14_0E52` (combatturn1.go)
- [ ] ovr14:0E82 `o14_0E82` (combatturn1.go)
- [ ] ovr14:1085 `o14_1085` (combatturn1.go)
- [ ] ovr14:116A `o14_116A` (combatturn1.go)
- [ ] ovr14:1351 `o14_1351` (combatturn1.go)
- [ ] ovr14:1406 `o14_1406` (combatturn1.go)
- [ ] ovr14:17F6 `o14_17F6` (combatturn1.go)
- [ ] ovr14:1884 `o14_1884` (combatturn1.go)
- [ ] ovr14:1B1E `o14_1B1E` (combatturn1.go)
- [ ] ovr14:1BF8 `o14_1BF8` (combatturn2.go)
- [ ] ovr14:1E16 `chooseSpellTarget` (combatturn2.go)
- [ ] ovr14:20C3 `o14_20C3` (combatturn2.go)
- [ ] ovr14:2414 `o14_2414` (combatturn2.go)
- [ ] ovr14:2574 `rangedFacing` (combatturn2.go)
- [ ] ovr14:263D `o14_263D` (combatturn2.go)
- [ ] ovr14:26B0 `attackAnimation` (combatturn2.go)
- [ ] ovr14:2879 `o14_2879` (combatturn2.go)
- [ ] ovr14:2908 `o14_2908` (combatturn2.go)
- [ ] ovr14:2998 `o14_2998` (combatturn2.go)
- [ ] ovr14:2AA0 `aimPrompt` (combatturn2.go)
- [ ] ovr14:2C3D `aimConfirm` (combatturn2.go)
- [ ] ovr14:2DFA `aimManual` (combatturn2.go)
- [ ] ovr14:3558 `o14_3558` (combatturn2.go)
- [ ] ovr14:37E4 `o14_37E4` (combatturn2.go)
- [ ] ovr14:39D2 `o14_39D2` (combatturn2.go)
- [ ] ovr14:3A86 `o14_3A86` (combatturn2.go)
- [ ] ovr14:3B93 `o14_3B93` (combatturn2.go)
- [ ] ovr14:3C6D `o14_3C6D` (combatturn2.go)

## ovr15
- [ ] ovr15:0000 `setDoor` (game.go)
- [ ] ovr15:0236 `partyHasSkill` (game.go)
- [ ] ovr15:0294 `bashDoor` (game.go)
- [ ] ovr15:04DF `pickLock` (game.go)
- [ ] ovr15:05AB `spellSlot` (game.go)
- [ ] ovr15:05EE `findCaster` (game.go)
- [ ] ovr15:0648 `knock` (game.go)
- [ ] ovr15:068D `checkEdge` (game.go)
- [ ] ovr15:073B `stepAhead` (game.go)
- [ ] ovr15:0818 `mapMove` (game.go)
- [ ] ovr15:0989 `CommandInput` (game.go)
- [ ] ovr15:0D6D `Move` (game.go)

## ovr16
- [ ] ovr16:0000 `studyTime` (camp.go)
- [ ] ovr16:0159 `clearMemorize` (camp.go)
- [ ] ovr16:019C `clearScribe` (camp.go)
- [ ] ovr16:0214 `clearStudy` (camp.go)
- [ ] ovr16:025C `slotsLeft` (camp.go)
- [ ] ovr16:036C `canStudy` (camp.go)
- [ ] ovr16:04A1 `CastMenu` (camp.go)
- [ ] ovr16:05A4 `memorizeInfo` (camp.go)
- [ ] ovr16:077A `restMenu` (camp.go)
- [ ] ovr16:0824 `sortSpells` (camp.go)
- [ ] ovr16:090A `memorizeMenu` (camp.go)
- [ ] ovr16:0B5B `scribeMenu` (camp.go)
- [ ] ovr16:0FBC `affectNames` (camp.go)
- [ ] ovr16:13F1 `magicMenu` (camp.go)
- [ ] ovr16:14A9 `movePrev` (camp.go)
- [ ] ovr16:1601 `moveNext` (camp.go)
- [ ] ovr16:1780 `orderMenu` (camp.go)
- [ ] ovr16:18E5 `dropMenu` (camp.go)
- [ ] ovr16:1A97 `campSpeedMenu` (camp.go)
- [ ] ovr16:1C59 `alterMenu` (camp.go)
- [ ] ovr16:1E6F `CampMenu` (camp.go)

## ovr17
- [ ] ovr17:0000 `freeCharacter` (create.go)
- [ ] ovr17:0161 `PartyMenu` (party.go)
- [ ] ovr17:056C `CreateCharacter` (create.go)
- [ ] ovr17:1F37 `DropCharacter` (create.go)
- [ ] ovr17:2033 `modifyDraw` (train.go)
- [ ] ovr17:209E `ModifyCharacter` (train.go)
- [ ] ovr17:29AC `TrainCharacter` (train.go)
- [ ] ovr17:306D `AddCharacter` (train.go)
- [ ] ovr17:3291 `RemoveSelected` (party.go)
- [ ] ovr17:33E7 `o17_33E7` (train.go)
- [ ] ovr17:34DF `iconPane` (train.go)
- [ ] ovr17:35BE `iconCopy` (train.go)
- [ ] ovr17:382D `o17_382D` (train.go)
- [ ] ovr17:3F31 `o17_3F31` (train.go)
- [ ] ovr17:3FB3 `minHitPoints` (train.go)
- [ ] ovr17:408C `maxHitPoints` (train.go)
- [ ] ovr17:4235 `o17_4235` (train.go)
- [ ] ovr17:42ED `chargeGold` (train.go)

## ovr18
- [ ] ovr18:0010 `BuildCharList` (files.go)
- [ ] ovr18:01B8 `WriteCharList` (files.go)
- [ ] ovr18:0482 `BuildCharIcon` (cmap.go)
- [ ] ovr18:069E `SaveCharacter` (files.go)
- [ ] ovr18:0AEC `LoadCharacter` (files.go)
- [ ] ovr18:0EC4 `LoadMonster` (files.go)
- [ ] ovr18:1262 `AddMonsterToParty` (files.go)
- [ ] ovr18:1446 `AddToParty` (files.go)
- [ ] ovr18:1602 `LoadGame` (files.go)
- [ ] ovr18:1D58 `SaveGame` (files.go)
- [x] ovr18:22D5 `SaveAreaState`: dropped (disk-swap prompt; all files are in one folder)
- [ ] ovr18:23E0 `o18_23E0` (story.go)

## ovr19
- [ ] ovr19:02A4 `ProgramEight` (story.go) — trace-proven by `finale`
- [ ] ovr19:06B3 `EndgameHook` (story.go) — trace-proven by `pyramid`, `recovered`
- [ ] ovr19:08F6 `o19_08F6` (story.go)

## ovr20
- [ ] ovr20:004A `CharacterSheet` (view.go)
- [ ] ovr20:050D `showMoney` (view.go)
- [ ] ovr20:0608 `showCombatStats` (view.go)
- [ ] ovr20:0901 `showStat` (view.go)
- [ ] ovr20:0AB3 `ViewCharacter` (view.go)
- [ ] ovr20:0D70 `canLoseItem` (view.go)
- [ ] ovr20:0EF8 `ItemsMenu` (view.go)
- [ ] ovr20:14D0 `readyItem` (view.go)
- [ ] ovr20:1734 `tradeItem` (view.go)
- [ ] ovr20:17ED `halveItem` (view.go)
- [ ] ovr20:189D `joinItems` (view.go)
- [ ] ovr20:1A8A `useItem` (view.go)
- [ ] ovr20:1CEE `sellItem` (view.go)
- [ ] ovr20:1F52 `identifyItem` (view.go)
- [ ] ovr20:2156 `TradeMoney` (view.go)
- [ ] ovr20:24A7 `DropMoney` (view.go)
- [ ] ovr20:275B `overloaded` (view.go)
- [ ] ovr20:27DF `partyKeypad` (party.go)
- [ ] ovr20:28BE `charMoneyValue` (money.go)
- [ ] ovr20:299C `SpellList` (view.go)

## ovr21
- [ ] ovr21:0000 `tickEffects` (game.go)
- [ ] ovr21:02F2 `normalizeClock` (game.go)
- [ ] ovr21:03A3 `normalizeRest` (camp.go)
- [ ] ovr21:03D5 `AdvanceClock` (game.go)
- [ ] ovr21:047A `subRest` (camp.go)
- [ ] ovr21:0565 `twoDigits` (camp.go)
- [ ] ovr21:05F0 `showRest` (camp.go)
- [ ] ovr21:0721 `restSetup` (camp.go)
- [ ] ovr21:0871 `healTick` (camp.go)
- [ ] ovr21:0925 `memorizeOne` (camp.go)
- [ ] ovr21:09E7 `scribeOne` (camp.go)
- [ ] ovr21:0B0A `memTick` (camp.go)
- [ ] ovr21:0BB2 `hourTick` (camp.go)
- [ ] ovr21:0C9A `Rest` (camp.go)

## ovr22
- [ ] ovr22:0000 `maxWeight` (money.go)
- [ ] ovr22:0020 `subWeight` (money.go)
- [ ] ovr22:0054 `overweight` (money.go)
- [ ] ovr22:00A8 `poolValue` (money.go)
- [ ] ovr22:0130 `setCharMoney` (money.go)
- [ ] ovr22:0185 `setPoolMoney` (money.go)
- [ ] ovr22:0202 `addPlatinum` (money.go)
- [ ] ovr22:0294 `inputAmount` (money.go)
- [ ] ovr22:044C `giveMoney` (money.go)
- [ ] ovr22:0508 `PoolMoney` (money.go)
- [ ] ovr22:063A `ShareMoney` (money.go)
- [ ] ovr22:0A01 `dropMoney` (money.go)
- [ ] ovr22:0A88 `takeMoney` (money.go)
- [ ] ovr22:0B93 `coinFromText` (money.go)
- [ ] ovr22:0CEB `TakeMoneyMenu` (money.go)
- [ ] ovr22:0F7E `treasureFlags` (money.go)
- [ ] ovr22:0FD2 `randomItemClass` (money.go)
- [ ] ovr22:1010 `RandomItem` (randitem.go)
- [ ] ovr22:1295 `randomItemWeight` (randitem.go)
- [ ] ovr22:19B2 `Appraise` (randitem.go)

## ovr23
- [ ] ovr23:008A `spellMenu` (spells1.go)
- [ ] ovr23:028D `addSpellLine` (spells1.go)
- [ ] ovr23:044E `addScrollSpells` (spells1.go)
- [ ] ovr23:051C `addAllScrolls` (spells1.go)
- [ ] ovr23:059C `buildSpellList` (spells1.go)
- [ ] ovr23:0729 `o23_0729` (spells1.go)
- [ ] ovr23:07D6 `o23_07D6` (spells1.go)
- [ ] ovr23:08C3 `o23_08C3` (spells1.go)
- [ ] ovr23:0A91 `spellTargetSetup` (spells1.go)
- [ ] ovr23:0C20 `CastSpell` (spells1.go)
- [ ] ovr23:0F4F `o23_0F4F` (spells1.go)
- [ ] ovr23:100F `o23_100F` (spells2.go)
- [ ] ovr23:1040 `o23_1040` (spells2.go)
- [ ] ovr23:106B `o23_106B` (spells2.go)
- [ ] ovr23:10A7 `o23_10A7` (spells2.go)
- [ ] ovr23:10E9 `o23_10E9` (spells2.go)
- [ ] ovr23:1123 `o23_1123` (spells2.go)
- [ ] ovr23:1162 `o23_1162` (spells2.go)
- [ ] ovr23:1190 `o23_1190` (spells2.go)
- [ ] ovr23:11DD `o23_11DD` (spells2.go)
- [ ] ovr23:12A5 `o23_12A5` (spells2.go)
- [ ] ovr23:136E `o23_136E` (spells2.go)
- [ ] ovr23:13D8 `o23_13D8` (spells2.go)
- [ ] ovr23:1439 `o23_1439` (spells2.go)
- [ ] ovr23:1499 `o23_1499` (spells2.go)
- [ ] ovr23:14C7 `o23_14C7` (spells2.go)
- [ ] ovr23:151B `o23_151B` (spells2.go)
- [ ] ovr23:1658 `o23_1658` (spells2.go)
- [ ] ovr23:17E9 `o23_17E9` (spells2.go)
- [ ] ovr23:1822 `o23_1822` (spells2.go)
- [ ] ovr23:185B `o23_185B` (spells2.go)
- [ ] ovr23:190F `o23_190F` (spells2.go)
- [ ] ovr23:19C3 `o23_19C3` (spells2.go)
- [ ] ovr23:1A16 `o23_1A16` (spells2.go)
- [ ] ovr23:1A4F `o23_1A4F` (spells2.go)
- [ ] ovr23:1A8A `o23_1A8A` (spells2.go)
- [ ] ovr23:1ACC `o23_1ACC` (spells2.go)
- [ ] ovr23:1B11 `o23_1B11` (spells2.go)
- [ ] ovr23:1F41 `o23_1F41` (spells2.go)
- [ ] ovr23:206F `o23_206F` (spells2.go)
- [ ] ovr23:2217 `o23_2217` (spells2.go)
- [ ] ovr23:225D `o23_225D` (spells2.go)
- [ ] ovr23:228A `cureDiseaseAffects` (spells2.go)
- [ ] ovr23:232F `o23_232F` (spells2.go)
- [ ] ovr23:234C `o23_234C` (spells2.go)
- [ ] ovr23:2385 `o23_2385` (spells2.go)
- [ ] ovr23:24D6 `o23_24D6` (spells2.go)
- [ ] ovr23:2540 `spellRemoveCurse` (spells1.go)
- [ ] ovr23:2604 `fxCursed` (spells1.go)
- [ ] ovr23:263D `fxBlinking` (spells1.go)
- [ ] ovr23:266B `fxFireball` (spells1.go)
- [ ] ovr23:2764 `removeAffectFromSide` (spells1.go)
- [ ] ovr23:2892 `fxHaste` (spells1.go)
- [ ] ovr23:28BC `strikeSquare` (spells1.go)
- [ ] ovr23:295B `boltPath` (spells1.go)
- [ ] ovr23:2BB7 `fxLightning` (spells1.go)
- [ ] ovr23:2C09 `fxSlow` (spells1.go)
- [ ] ovr23:2C43 `fxRestore` (spells1.go)
- [ ] ovr23:2E02 `fxSpeedy` (spells1.go)
- [ ] ovr23:2E4D `fxCureParalysis` (spells1.go)
- [ ] ovr23:2EE5 `fxStronger` (spells1.go)
- [ ] ovr23:2F4D `fxBolt` (spells1.go)
- [ ] ovr23:2F99 `fxParalyzed` (spells1.go)
- [ ] ovr23:2FD0 `fxHeal2d4` (spells1.go)
- [ ] ovr23:302A `fxInvisible` (spells1.go)
- [ ] ovr23:3058 `fxDamage2d4` (spells1.go)
- [ ] ovr23:3092 `fxNone` (spells1.go)
- [ ] ovr23:30A2 `fxReading` (spells1.go)
- [ ] ovr23:30D9 `o23_30D9` (spells1.go)
- [ ] ovr23:323D `isScroll` (spells1.go)
- [ ] ovr23:328A `scrollUsed` (spells1.go)
- [ ] ovr23:331A `o23_331A` (spells1.go)
- [ ] ovr23:3413 `spellTableInit` (spells1.go)
- [ ] ovr23:37B0 `o23_37B0` (spells1.go)

## ovr24
- [ ] ovr24:0000 `RecomputeCharacter` (charcalc.go)
- [ ] ovr24:01AF `wisdomSlots` (charcalc.go)
- [ ] ovr24:0255 `savingThrows` (charcalc.go)
- [ ] ovr24:0329 `thiefSkills` (charcalc.go)

## ovr25
- [ ] ovr25:0000 `callAffect` (rules.go)
- [ ] ovr25:0026 `RemoveAffect` (rules.go)
- [ ] ovr25:0160 `checkAffect` (rules.go)
- [ ] ovr25:02FA `affectGroup` (rules.go)
- [ ] ovr25:0AE7 `stinkingCloud` (rules.go)
- [ ] ovr25:0C4F `RollHit` (rules.go)
- [ ] ovr25:0CB7 `AttackHits` (rules.go)
- [ ] ovr25:0D63 `SavingThrow` (rules.go)
- [ ] ovr25:0DE3 `Dice` (engine.go)
- [ ] ovr25:0E2E `RollDice` (rules.go)
- [ ] ovr25:0E52 `AddAffect` (rules.go)
- [ ] ovr25:0F06 `KillCharacter` (rules.go)
- [ ] ovr25:0FD2 `removeInvisibility` (rules.go)
- [ ] ovr25:100A `removeCombatAffects` (rules.go)
- [ ] ovr25:1041 `removeAffects4A` (rules.go)
- [ ] ovr25:1081 `CureAffect` (rules.go)
- [ ] ovr25:10E4 `strValue` (rules.go)
- [ ] ovr25:110D `strDecode` (rules.go)
- [ ] ovr25:115C `SetStrength` (rules.go)
- [ ] ovr25:1344 `Damage` (rules.go)
- [ ] ovr25:1656 `AddAffectMsg` (rules.go)
- [ ] ovr25:175D `Heal` (rules.go)
- [ ] ovr25:1869 `Revive` (rules.go)

## ovr26
- [ ] ovr26:0000 `nth` (listmenu.go)
- [ ] ovr26:00CB `menuItems` (menu.go)
- [ ] ovr26:01DA `drawMenu` (menu.go)
- [ ] ovr26:03BD `combatInput` (combatturn2.go)
- [ ] ovr26:06FB `ClearRow24` (input.go)
- [ ] ovr26:07A4 `drawRows` (listmenu.go)
- [ ] ovr26:08D7 `firstNonSpace` (listmenu.go)
- [ ] ovr26:0969 `lmDrawRow` (listmenu.go)
- [ ] ovr26:0B03 `lmSkipHeaders` (listmenu.go)
- [ ] ovr26:0C33 `lmPageScroll` (listmenu.go)
- [ ] ovr26:0CC5 `lmMoveCursor` (listmenu.go)
- [ ] ovr26:0D9A `ListMenu` (listmenu.go)
- [ ] ovr26:10D1 `YesNo` (menu.go)
- [ ] ovr26:1140 `newList` (listmenu.go)

## ovr27
- [ ] ovr27:0000 `drawAreaMapParty` (areamap.go)
- [ ] ovr27:00C7 `skyFg` (screen.go)
- [ ] ovr27:00FB `skyBand` (screen.go)
- [ ] ovr27:012F `skyGround` (screen.go)
- [ ] ovr27:016D `Redraw` (screen.go)

## ovr28
- [ ] ovr28:0000 `DrawAreaMap` (areamap.go)
- [ ] ovr28:0207 `areaMapInit` (areamap.go)

## ovr29
- [ ] ovr29:0000 `drawPicAt` (anim.go)
- [ ] ovr29:00A2 `LoadAnimPic` (anim.go)
- [ ] ovr29:048F `FreeAnimPic` (anim.go)
- [ ] ovr29:04F9 `SheetPortrait` (view.go)
- [ ] ovr29:05D9 `LoadPortrait` (anim.go)
- [ ] ovr29:0697 `DrawPortrait` (anim.go)
- [ ] ovr29:0708 `DrawAnimFrame` (anim.go)

## ovr30
- [ ] ovr30:0000 `DrawOverheadMap` (geo.go)
- [ ] ovr30:0155 `SetSkyColours` (geo.go)
- [ ] ovr30:0174 `drawSky` (geo.go)
- [ ] ovr30:0211 `drawWallPart` (geo.go)
- [ ] ovr30:031F `onMap` (geo.go)
- [ ] ovr30:0350 `DoorAt` (geo.go)
- [ ] ovr30:047B `WallAt` (geo.go)
- [ ] ovr30:057A `MapCell` (geo.go)
- [ ] ovr30:05FD `Draw3DView` (geo.go)
- [ ] ovr30:0DE8 `wallSlots` (geo.go)
- [ ] ovr30:0E08 `LoadWalldef` (geo.go)
- [ ] ovr30:10C7 `LoadGeo` (geo.go)

## ovr31
- [ ] ovr31:0000 `sign` (combatmap.go)
- [ ] ovr31:002E `nearSort` (cmap.go)
- [ ] ovr31:0422 `lineOfSight` (cmap.go)
- [ ] ovr31:057B `inArc` (cmap.go)
- [ ] ovr31:0909 `buildNear` (cmap.go)

## ovr32
- [ ] ovr32:0000 `footprint` (cmap.go)
- [ ] ovr32:0068 `updateScreenPos` (cmap.go)
- [ ] ovr32:0105 `combatViewEnter` (cmap.go)
- [ ] ovr32:0127 `drawCursor` (cmap.go)
- [ ] ovr32:02D5 `eraseCursor` (cmap.go)
- [ ] ovr32:03A0 `combatRedrawMap` (cmap.go)
- [ ] ovr32:04BE `mapAt` (cmap.go)
- [ ] ovr32:052B `combatClearIcon` (cmap.go)
- [ ] ovr32:06E9 `combatInView` (cmap.go)
- [ ] ovr32:071A `combatOnScreen` (cmap.go)
- [ ] ovr32:07D4 `recentre` (cmap.go)
- [ ] ovr32:0996 `combatCentre` (cmap.go)
- [ ] ovr32:0AF8 `combatTurn` (cmap.go)
- [ ] ovr32:0C37 `nearCombatants` (cmap.go)
- [ ] ovr32:0C5F `combatIndex` (cmap.go)
- [ ] ovr32:0CB9 `groundAt` (cmap.go)
- [ ] ovr32:0E11 `combatRemoveFromMap` (cmap.go)
- [ ] ovr32:10A7 `combatSquareFree` (cmap.go)
- [ ] ovr32:1298 `combatRedrawChar` (cmap.go)

## ovr33
- [ ] ovr33:0022 `LoadBackpacIcons` (cmap.go)
- [ ] ovr33:00F1 `drawTile` (cmap.go)
- [ ] ovr33:014E `freeIcon` (cmap.go)
- [ ] ovr33:01C8 `LoadIconPic` (cmap.go)
- [ ] ovr33:04FC `drawIcon` (cmap.go)

## ovr34
- [ ] ovr34:0004 `debugValues` (affects2.go)
- [ ] ovr34:00F7 `o34_00F7` (story.go)

## ovr35
- [ ] ovr35:003B `LoadSymbolSet` (pic.go)
- [ ] ovr35:015D `Put8x8Symbol` (classic3d_test.go classicWallPart)

## START 00BA
- [ ] 00BA:0000 `weaponStats` (charcalc.go)
- [ ] 00BA:01FB `armourMove` (charcalc.go)
- [ ] 00BA:0282 `itemAC` (charcalc.go)
- [ ] 00BA:03A0 `encumbrance` (charcalc.go)
- [ ] 00BA:0442 `ItemName` (items.go)
- [ ] 00BA:0767 `DisplayPartyList` (party.go)
- [ ] 00BA:08EF `displayAC` (char.go)
- [ ] 00BA:0973 `displayHP` (char.go)
- [ ] 00BA:09E3 `combatPanel` (combat.go)
- [ ] 00BA:0B73 `hasAffect286B` (charcalc.go)
- [ ] 00BA:0BB8 `ApplyItems` (charcalc.go)
- [ ] 00BA:1037 `IntToStr` (char.go)
- [ ] 00BA:10E6 `dexAC` (charcalc.go)
- [ ] 00BA:1168 `dexMissile` (charcalc.go)
- [ ] 00BA:11EA `strIndex` (charcalc.go)
- [ ] 00BA:1282 `strHit` (charcalc.go)
- [ ] 00BA:1325 `strDamage` (charcalc.go)
- [ ] 00BA:13AB `strCarry` (charcalc.go)
- [ ] 00BA:148B `forgetSpell` (rules.go)
- [ ] 00BA:1509 `RemoveItem` (items.go)
- [ ] 00BA:15F7 `AddItemCopy` (items.go)
- [ ] 00BA:1693 `BottomMessage` (items.go)
- [ ] 00BA:16EC `CharMessage` (rules.go)
- [ ] 00BA:17E3 `ClearMessageArea` (rules.go)
- [ ] 00BA:1816 `displayName` (char.go)
- [ ] 00BA:1892 `iconToSprite` (cmap.go)
- [ ] 00BA:19DE `hitColours` (cmap.go)
- [ ] 00BA:1A29 `flyMissile` (combatmap.go)
- [ ] 00BA:1FEB `hitMessage` (rules.go)
- [ ] 00BA:2199 `FindAffect` (rules.go)
- [ ] 00BA:2228 `ApplyDamage` (rules.go)
- [ ] 00BA:2348 `healedMessage` (rules.go)
- [ ] 00BA:23B7 `isParty` (combatmap.go)
- [ ] 00BA:23DB `countSides` (rules.go)
- [ ] 00BA:2433 `nearEnemies` (combatmap.go)
- [ ] 00BA:255B `rangeTo` (combatmap.go)
- [ ] 00BA:2637 `clearCombatInfo` (rules.go)
- [ ] 00BA:2686 `guard` (combat.go)
- [ ] 00BA:26C2 `casterLevel` (combatmap.go)
- [ ] 00BA:272A `DrawMainFrame` (screen.go)
- [ ] 00BA:27D9 `DrawMainScreen` (screen.go)
- [ ] 00BA:290B `StatusLine` (screen.go)
- [ ] 00BA:2B5E `areaMapIcons` (areamap.go)
- [ ] 00BA:2B88 `combatUseHook` (combat.go)
- [ ] 00BA:2C48 `SelectCharacter` (party.go)
- [ ] 00BA:2E03 `rangedWeapon` (combat.go)
- [ ] 00BA:2E46 `throwable` (combat.go)
- [ ] 00BA:2E8B `ammoFor` (combat.go)
- [ ] 00BA:2F57 `atRaceLimit` (charcalc.go)
- [ ] 00BA:36FE `AreaMapScreen` (the port was unused and went)

## START 03F1
- [ ] 03F1:0000 `DrawBorder` (text.go)
- [ ] 03F1:0137 `ClearCells` (text.go)
- [ ] 03F1:02E8 `DrawBoxTitled` (text.go)

## START 0464
- [ ] 0464:0111 `LoadPicMasked` (pic.go)
- [ ] 0464:08D4 `DrawPictureMasked` (pic.go)
- [ ] 0464:0D71 `restoreUnder` (cmap.go)
- [ ] 0464:1951 `DrawPicture` (pic.go)
- [ ] 0464:1B52 `Flip` (combatmap.go)
- [ ] 0464:1E5C `recolour` (story.go recolourFrame) — EGA path ported from the listing; CGA/Tandy paths not
- [ ] 0464:2632 `FillRect` (geo.go)

## START 0709
- [ ] 0709:01DF `DisplayChar` (text.go)
- [ ] 0709:05EB `DisplaySpaces` (input.go)
- [ ] 0709:0626 `DisplayString` (text.go)
- [ ] 0709:06A7 `printSeg` (text.go)
- [ ] 0709:0768 `DisplayTextBlock` (text.go)
- [ ] 0709:09E3 `InputString` (input.go)
- [ ] 0709:0B66 `InputNumber` (text.go)
- [ ] 0709:0BF5 `MessageRow24b` (files.go)
- [ ] 0709:0C3F `Ticks` (engine.go)
- [ ] 0709:0CAB `ClearScreen` (view.go)
- [ ] 0709:0D68 `DisplayMessageWait` (input.go)
- [ ] 0709:0DBE `MsgDelay` (combatmap.go)
- [ ] 0709:0DD3 `ClearBox` (text.go)

## START 0802
- [ ] 0802:0356 `removeSubstr` (files.go)

## START 0899
- [ ] 0899:0000 `Exit` (engine.go)
- [ ] 0899:0059 `ReadKeyByte` (input.go)
- [ ] 0899:0122 `flushKey` (combatsetup.go)

## START 08AE
- [ ] 08AE:00DC `PlaySound` (sound.go)
- [ ] 08AE:017B `SoundReset` (sound.go)
- [ ] 08AE:01A5 `soundWait` (sound.go)
- [ ] 08AE:1477 `tick` (sound.go)
- [ ] 08AE:157A `step` (sound.go)
- [ ] 08AE:160B `fetch` (sound.go)
- [ ] 08AE:1742 `start` (sound.go)

## START 0A40
- [ ] 0A40:029C `Delay` (engine.go)

## START routines not named in the Go code yet
Found by Ghidra in START.bin (overlay stubs below 00BA and the 0AF8 runtime left out).
- [ ] 00BA:10AD
- [ ] 00BA:309E
- [ ] 00BA:3125
- [ ] 00BA:312A
- [ ] 00BA:3235
- [ ] 00BA:323F
- [ ] 00BA:324E
- [ ] 00BA:32A8
- [ ] 00BA:32AD
- [ ] 00BA:32BC
- [ ] 00BA:32C6
- [ ] 00BA:32CB
- [ ] 03F1:0367
- [ ] 0428:002A
- [ ] 0464:0000
- [ ] 0464:007B
- [ ] 0464:00CF
- [ ] 0464:00FF
- [ ] 0464:02A5
- [ ] 0464:1484
- [ ] 0464:1509
- [ ] 0464:1596
- [ ] 0464:15C7
- [ ] 0464:2A0D
- [ ] 0464:2A4A
- [ ] 0709:0000
- [ ] 0709:0071
- [ ] 0709:00A0
- [ ] 0709:00B7
- [ ] 0709:0F82
- [ ] 0802:0020
- [ ] 0802:0040
- [ ] 0802:0105
- [ ] 0802:0175
- [ ] 0802:0193
- [ ] 0802:025D
- [ ] 0802:050F
- [ ] 0802:06E7
- [ ] 0802:0968
- [ ] 0899:013C
- [ ] 08AE:0000
- [ ] 08AE:01D2
- [ ] 08AE:13B1
- [ ] 08AE:17AE
- [ ] 08AE:181A
- [ ] 08AE:1830
- [ ] 08AE:184B
- [ ] 08AE:18AC
- [ ] 08AE:18CE
- [ ] 08AE:18E2
- [ ] 08AE:1907
- [ ] 0A40:0097
- [ ] 0A40:00E5
- [ ] 0A40:0143
- [ ] 0A40:02BD
- [ ] 0A40:030F
- [ ] 0A40:0394
- [ ] 0A40:0477
- [ ] 0A40:047E
- [ ] 0A40:04D7
- [ ] 0A40:04F8
- [ ] 0A40:04FF
- [ ] 0A40:060B
- [ ] 0A40:07C3
- [ ] 0A40:0812
- [ ] 0A40:086E
- [ ] 0A40:087B
