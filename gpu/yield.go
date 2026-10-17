//go:build !js

package gpu

import (
	_ "github.com/gogpu/wgpu/hal/allbackends" // the desktop GPU backends

	"gpu/internal/glfw"
)

// yield: on the desktop presenting already waited for the display.
func yield(*glfw.Window) {}

// attach: GLFW delivers the window's events by itself.
func attach(*glfw.Window) {}
