//go:build linux && !android

package glfw

import glfw "github.com/go-gl/glfw/v3.3/glfw"

var GetX11Display = glfw.GetX11Display
