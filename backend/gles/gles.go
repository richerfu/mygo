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

type ColorEncoding = gl.ColorEncoding
type ColorOutput = gl.ColorOutput
type ColorRenderer = gl.ColorRenderer

const (
	ColorSRGB         = gl.ColorSRGB
	ColorExtendedSRGB = gl.ColorExtendedSRGB
	ColorDisplayP3    = gl.ColorDisplayP3
	ColorBT2020PQ     = gl.ColorBT2020PQ
)

// NewColor creates an opt-in renderer for a host which negotiates its output
// color space. SetOutput must match the bound framebuffer's actual encoding.
// Extended sRGB needs a floating-point target; PQ needs at least 10 RGB bits.
// Outputs must use a linear framebuffer attachment: the shader writes encoded
// values, so an sRGB attachment would apply the transfer function twice.
// The renderer does not configure an OS window or claim display HDR support.
func NewColor(c Context, retained bool) (*ColorRenderer, error) {
	if err := gl.UseContext(c.Resolve, c.Identity); err != nil {
		return nil, err
	}
	return gl.NewColorRenderer(retained)
}

// Presenter copies software-rendered, premultiplied BGRA pixels into the
// current framebuffer. The host owns the context and presents the framebuffer.
type Presenter = gl.Presenter

// UseVideoUpload installs a host's decoder-frame uploader. native is the opaque
// token in scene.Surface.Native; texture is the renderer's RGBA8 target in the
// current context. The callback must preserve GL state and keep source reads
// valid until the GPU finishes them. False falls back to the picture's RGBA
// restoration. The host owns frame leases until the engine replaces the picture
// or closes. Callbacks may run until an in-flight render completes after release.
// The returned function removes only this registration and is idempotent.
func UseVideoUpload(upload func(native uintptr, texture uint32) bool) func() {
	return gl.UseVideoUpload(upload)
}

// VideoTexture is an owned external OES decoder texture and its color/geometry
// metadata. It is sampled directly for ordinary image draws. Operations that
// require a 2D image (blur and shader image inputs) convert on demand. The normal
// renderer uses Copy2D, UseVideoUpload or RGBA restoration for SDR. NewColor's
// wide modes retain extended sRGB in a float texture without using Copy2D.
// Direct draws need no intermediate video texture in either mode.
type VideoTexture = gl.VideoTexture

// UseVideoSampler installs an optional decoder importer. native is the opaque
// scene.Surface.Native token. Returning nil uses the upload/CPU fallback. Import
// must preserve GL state. A non-nil texture transfers one owned lease; Release
// is called with the same context current before Render returns, even on error,
// and must wait for all GPU reads before returning the producer buffer. A later
// render imports afresh, so no decoder lease survives context disposal.
// The returned function removes only this registration and is idempotent.
func UseVideoSampler(importFrame func(native uintptr) *VideoTexture) func() {
	return gl.UseVideoSampler(importFrame)
}

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
