# gfx - Draw Calls

Draw calls occur any time that you *draw* an object to a canvas. Understanding *draw calls* is critical to diagnosing performance of 3D applications.

 * [What are they?](#what-are-they)
 * [Performance trade-off](#performance-trade-off)
 * [Example - Voxels](#example-voxels)
 * [Example - Sprites](#example-sprites)
 * [Batchers](#batchers)

# What are they?

Using the gfx package a draw call is done through the `gfx.Canvas` interface, like so:

`canvas.Draw(image.Rect(0,0,0,0), obj, cam)`

Where `obj` is a `gfx.Object` and `cam` is a `gfx.Camera`.

In literal OpenGL code the above draw operation would equate to either `glDrawArrays` or `glDrawElements` (depending on whether or not the mesh is indexed). Technically each `canvas.Draw` operations makes `N` draw calls, where `N` is the number of `gfx.Mesh` that the `gfx.Object` being drawn contains.

# Performance trade-off

The performance of draw calls depends on many factors. As a simple example lets say that we want to draw *10,000 triangles*:

 * *Fast*: 1 draw call, 10,000 triangles/call.
 * *Also Fast*: 2 draw calls, 5,000 triangles/call.
 * *Slow*: 100 draw calls, 100 triangles/call.

You might be thinking: *"Well, just draw the entire scene in one draw call!"*. (If you did want to do this, you would just have one `gfx.Object` with one giant `gfx.Mesh`). It's not so simple though -- for one thing a single draw call is restricted to:

 * One mesh -- Since every mesh equates 1:1 to a single draw call.
  * Modifying a mesh requires that modification to be uploaded to the GPU again, making dynamic modifications slow.
 * One shader.
  * Super-shaders (i.e. all-in-one) are slow and expensive due to their excessive use of branching (`if` statements, etc).
 * One set of textures.
  * Requires special care to handle properly in shaders.
 * One *graphics state*.
  * Single alpha mode.
  * Single stencil state.
  * etc.

Not to mention the fact that with a single mesh *you can't control the draw order of triangles independantly*. To recap on what we have learned thus far:

 * Draw calls *are expensive*.
 * To do anything cool, we need multiple of them.

# Example - Voxels

Voxel-based games often make sections of multiple blocks effectively called *chunks*. With our new knowledge of *draw calls* we can explain one of the things that *chunks* solved for these games:

 * Draw calls are expensive.
 * Putting all of the blocks into one single giant mesh makes updates (read: *creating/destroying blocks*) slow.
 * The idea: break things up into medium sized *chunks*.
  * Small enough that dynamic updates are fast, the smaller the chunk the more fast dynamic updates are.
  * Large enough that we have *lesser draw calls*, the larger the chunk the more fast rendering of it is.

# Example - Sprites

For another example let's take static (i.e. non-animated) sprites. A sprite is a square card made up of two triangles (a quad) and a `gfx.Texture` (i.e. the sprites actual image). We can now look at this in a real-world situation:

 * 500 sprites -- each one has their own `gfx.Object` and `gfx.Mesh` -- totalling to *500 draw calls*.
  * Con: *Slow.*
  * Pro: *Sprites can pick-and-choose any shader, texture, etc.*
  * Pro: *Each sprite can move, scale, rotate, etc independently.*
 * 500 sprites -- one single `gfx.Object` and `gfx.Mesh` -- totalling to *1 draw call.*
  * Pro: *Fast.*
  * Con: *Each sprite must use the same exact shader, texture, etc.*
  * Con: *Moving, scaling, rotating, etc requires re-uploading the sprites to the GPU or tricky shaders.*

The best solution here is to *know your target, and test*. Does the sprite need to move constantly, and use it's own dedicated shader, textures, etc? Or will it be mostly static, could it share shaders and textures with many other sprites? The performance you acquire depends highly on how well you can answer these questions.
