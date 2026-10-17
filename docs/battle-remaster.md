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
```

## Milestones

- [x] 1. Ebiten -> gpu (wgpu) swap, app-wide (por, Blobber, editor, web build)
- [x] 2. Engine seam: View.Battle plus a test
- [x] 3. 3D map: wall blocks with the original tiles, the floor
- [x] 4. Figurines: extruded sprites on pedestals (blue for the party, red for enemies)
- [x] 5. XCOM camera (orbit with [ and ], follows the active unit) and F10 switching
- [ ] 6. Action bar of icons at the bottom centre (clicks send the menu hotkeys)
