//go:build js

package gpu

import "gpu/internal/glfw"

// yield hands control back to the browser until the next animation frame.
func yield(w *glfw.Window) { w.SwapBuffers() }

// attach makes PollEvents deliver the canvas's events.
func attach(w *glfw.Window) { w.MakeContextCurrent() }
