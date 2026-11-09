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

// Presenter copies software-rendered, premultiplied BGRA pixels into the
// current framebuffer. The host owns the context and presents the framebuffer.
type Presenter = gl.Presenter

// NewPresenter prepares a software-frame presenter with c current.
func NewPresenter(c Context) (*Presenter, error) {
	if err := gl.UseContext(c.Resolve, c.Identity); err != nil {
		return nil, err
	}
	return gl.NewPresenter()
}

// ReadFramebuffer reads the current framebuffer as tightly packed,
// premultiplied BGRA rows, starting at the top. c must be current.
func ReadFramebuffer(c Context, width, height int) ([]byte, error) {
	if err := gl.UseContext(c.Resolve, c.Identity); err != nil {
		return nil, err
	}
	return gl.ReadFramebuffer(width, height)
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
