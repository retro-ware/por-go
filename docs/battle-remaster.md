# Battle map remaster: plan

F10 swaps the combat screen between the original and a 3D tabletop view.
The game state never changes. The remaster only reads a published snapshot,
the same seam the first-person remaster uses (`engine.View`).

## Rendering

Ebiten is gone. It is replaced by `gpu/`, built on glfw and the gogpu/wgpu
fork (`../gogpu-wgpu`, github.com/memmaker/wgpu). `gpu/` has the same surface
as the Ebiten API we used: images, DrawTriangles, WGSL shaders, input, and the
game loop. It adds `DrawMesh`, which draws depth-tested 3D triangles.

## Modules

```
gpu               window, input, 2D batches, WGSL shaders, DrawMesh
engine            View.Battle: ground grid, tiles, units, cursor
present/battle    3D view: camera (orbit), scene meshes, action bar
  scene.go        pure: map mesh (blocks in tile 3, the plain stone), boxes, matrices
  extrude.go      sprite -> Minecraft-style slab (copied from VoxelLib)
  bar.go          the action bar: a button per menu item, clicks send its hotkey
```

## Milestones

- [x] 1. Ebiten -> gpu (wgpu) swap, app-wide (por, Blobber, editor, web build)
- [x] 2. Engine seam: View.Battle plus a test
- [x] 3. 3D map: wall blocks with the original tiles, the floor
- [x] 4. Figurines: extruded sprites on pedestals (blue for the party, red for enemies)
- [x] 5. XCOM camera (orbit with [ and ], follows the active unit) and F10 switching
- [x] 6. Action bar of icons at the bottom centre (clicks send the menu hotkeys)
- [x] 7. Tabletop look: lit meshes (sun, sky, specular), the DungCom stone derived at runtime, floor grain and square edges, corner occlusion, blob shadows, a board on a wooden table
- [x] 8. Feature parity: effects (hits, missiles, spells) with point lights and sparks, deaths, move/aim marks, the cursor, clicks on squares, zoom (- and =)
- [x] 9. Playtest: dungeon and wilderness fights, DOS and Amiga art, aim, square clicks, effects, deaths, F10 mid-fight (seeded runs pixel-identical)
- [x] 10. XCOM controls: W A S D pan (held, relative to the heading; lets go when the camera gets a new thing to watch), Q/E turn, the wheel zooms, Home recentres, 1..9 and 0 pick the bar buttons (numbered), Tab/Shift-Tab cycle targets, Enter attacks the aimed one, Backspace ends the turn (Done, Quit); arrows stay with the game (manual aim, keypad steps)
- [x] 11. XCOM 2 flow: on the main combat menu a click on a square in reach walks there and hands back (the reach drawn in blue), a click on a foe aims at it and attacks it when it can; the bar drops Move and calls Aim Attack; hovering says what a click does (Move here, Attack NAME HP AC)
- [x] 12. XCOM 2, the rest: a clicked foe is attacked (a melee fighter walks up first); foes a click attacks framed red, spell areas orange; one's own figure clicked opens the sheet; bar: Attack, Items, Cast, Turn (undead), Quick, Guard, Wait, Bandage, End (Done+Quit), the aim bar only Free (manual), Attack, Cancel (Tab cycles); the active character's portrait card bottom left; button click areas follow the bar when the hint is wider than the buttons
