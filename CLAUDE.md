<!-- claude-reflect: learned rules (saved from the band; edit freely) -->
## Learned rules
- After completing each milestone, update the checklist and commit your changes.
- Do not use raw memory blocks or offsets in this codebase; represent every value as a typed variable.
- Never scale images, textures, or other assets without preserving their aspect ratio.
- Do not implement map edge wrapping; game maps never wrap at their edges, so treat wrapping as a bug.
- Every screen fills the whole window or screen: no black bars or borders on any side, in windowed mode, in fullscreen and after any resize. The 40x25 page is the lower bound; larger windows give more cells and every screen lays itself out to them (winRight, winBottom, promptRow). Windowed mode is locked to the 16:10 aspect of the title images. Fullscreen keeps the screen's own aspect: UI pages adapt to it, and original pictures (title images, backdrops) are the one exception, shown centred with bars rather than cropped or stretched.
<!-- end claude-reflect -->
