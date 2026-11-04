//go:build mygo_custom_backend && !js

// Package gles adapts the built-in complete scene renderer to an external
// OpenGL ES 3 host. Native context ownership remains entirely in the host.
package gles

import (
	"github.com/egoist/mygo/backend/render"
	"github.com/egoist/mygo/internal/gpu/gl"
)

type Context interface {
	Resolve(string) uintptr
	Identity() uintptr
}

func New(c Context, retained bool) (render.Renderer, error) {
	if err := gl.UseContext(c.Resolve, c.Identity); err != nil {
		return nil, err
	}
	r, err := gl.New()
	if err != nil {
		return nil, err
	}
	r.Partial = retained
	return r, nil
}
