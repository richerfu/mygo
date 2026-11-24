package ui

import (
	"bytes"
	"errors"
	"testing"

	"github.com/egoist/mygo/backend/render"
	"github.com/egoist/mygo/internal/gpu"
	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/internal/scene"
	"github.com/egoist/mygo/internal/surface"
)

type externalSceneRenderer struct{ frames, released, ops int }

func (r *externalSceneRenderer) Render(s *scene.Scene) error {
	r.frames++
	r.ops = len(s.Ops)
	return nil
}
func (r *externalSceneRenderer) Release() { r.released++ }

type externalRendererSurface struct {
	testSurface
	renderer *externalSceneRenderer
	made     int
}

func (s *externalRendererSurface) Native() platform.SurfaceNative { return platform.SurfaceNative{} }
func (s *externalRendererSurface) NewRenderer() (gpu.Renderer, error) {
	s.made++
	return s.renderer, nil
}
func TestExternalSurfaceOwnsRendererLifecycle(t *testing.T) {
	t.Setenv("MYGO_GPU", "")
	prepareExternalRendererFonts(t)
	s := &externalRendererSurface{renderer: new(externalSceneRenderer)}
	h := &windowHost{conn: &surface.Conn{Surface: s}}
	h.rt = newRuntime(func(c *context) {}, h)
	h.event(platform.SurfaceEvent{Kind: platform.SurfaceFrame})
	if s.made != 1 || s.renderer.frames != 1 || s.pixels != 0 {
		t.Fatalf("external renderer bypassed: made=%d frames=%d cpu=%d", s.made, s.renderer.frames, s.pixels)
	}
	h.event(platform.SurfaceEvent{Kind: platform.SurfaceRenew})
	if s.renderer.released != 1 {
		t.Fatal("surface renewal did not release renderer")
	}
	h.event(platform.SurfaceEvent{Kind: platform.SurfaceFrame})
	if s.made != 2 || s.renderer.frames != 2 || s.pixels != 0 {
		t.Fatal("renewed target did not use factory")
	}
	h.detach()
	if s.renderer.released != 2 {
		t.Fatal("detach did not release renewed renderer")
	}
}

type externalCapture struct {
	w, h  int
	rgba  []byte
	err   error
	seen  *scene.Scene
	calls int
}

func (c *externalCapture) Capture(s *scene.Scene) (int, int, []byte, error) {
	c.seen = s
	c.calls++
	return c.w, c.h, c.rgba, c.err
}

type externalCaptureRenderer struct {
	externalSceneRenderer
	*externalCapture
}

var _ render.Capturer = (*externalCaptureRenderer)(nil)

type externalCaptureSurface struct {
	testSurface
	*externalCapture
}

func TestExternalCapture(t *testing.T) {
	scene := &scene.Scene{Width: 2, Height: 1, Clear: scene.Color{R: 10, G: 20, B: 30, A: 255}}
	for _, fromSurface := range []bool{false, true} {
		c := &externalCapture{w: 1, h: 2, rgba: []byte{30, 20, 10, 255, 60, 50, 40, 255}}
		h := &windowHost{rt: &engine{}, last: scene, gpu: &externalSceneRenderer{}, conn: &surface.Conn{Surface: &testSurface{}}}
		if fromSurface {
			h.conn.Surface = &externalCaptureSurface{externalCapture: c}
		} else {
			h.gpu = &externalCaptureRenderer{externalCapture: c}
		}
		w, height, pix := h.capture()
		if w != c.w || height != c.h || !bytes.Equal(pix, c.rgba) || c.seen != scene || c.calls != 1 {
			t.Fatalf("surface=%v: got %dx%d %v, calls=%d scene=%p", fromSurface, w, height, pix, c.calls, c.seen)
		}
	}
}

func TestExternalCaptureFallsBack(t *testing.T) {
	s := &scene.Scene{Width: 2, Height: 1, Clear: scene.Color{R: 10, G: 20, B: 30, A: 255}}
	for _, tc := range []struct {
		name string
		c    externalCapture
	}{
		{"error", externalCapture{w: 1, h: 1, rgba: make([]byte, 4), err: errors.New("device lost")}},
		{"empty", externalCapture{w: 1, h: 1}},
		{"short", externalCapture{w: 2, h: 1, rgba: make([]byte, 4)}},
		{"long", externalCapture{w: 1, h: 1, rgba: make([]byte, 8)}},
		{"zero", externalCapture{w: 0, h: 1, rgba: make([]byte, 4)}},
		{"negative", externalCapture{w: -1, h: -1, rgba: make([]byte, 4)}},
		{"overflow", externalCapture{w: int(^uint(0) >> 1), h: 2, rgba: make([]byte, 4)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, fromSurface := range []bool{false, true} {
				h := &windowHost{rt: &engine{}, last: s, gpu: &externalSceneRenderer{}, conn: &surface.Conn{Surface: &testSurface{}}}
				if fromSurface {
					h.conn.Surface = &externalCaptureSurface{externalCapture: &tc.c}
				} else {
					h.gpu = &externalCaptureRenderer{externalCapture: &tc.c}
				}
				w, height, pix := h.capture()
				if w != 2 || height != 1 || !bytes.Equal(pix, []byte{10, 20, 30, 255, 10, 20, 30, 255}) {
					t.Fatalf("fallback: %dx%d %v", w, height, pix)
				}
			}
		})
	}
}

func TestExternalCapturePriority(t *testing.T) {
	s := &scene.Scene{Width: 1, Height: 1}
	r := &externalCapture{w: 1, h: 1, rgba: []byte{10, 20, 30, 255}}
	c := &externalCapture{w: 1, h: 1, rgba: []byte{40, 50, 60, 255}}
	h := &windowHost{rt: &engine{}, last: s, gpu: &externalCaptureRenderer{externalCapture: r},
		conn: &surface.Conn{Surface: &externalCaptureSurface{externalCapture: c}}}
	_, _, pix := h.capture()
	if !bytes.Equal(pix, r.rgba) || r.calls != 1 || c.calls != 0 {
		t.Fatal("renderer capture did not take precedence")
	}
	h.gpu = nil
	_, _, pix = h.capture()
	if c.calls != 0 || !bytes.Equal(pix, []byte{0, 0, 0, 0}) {
		t.Fatal("software rendering used a surface's GPU capture")
	}
	h.gpu = &externalSceneRenderer{}
	h.conn.Surface = &testSurface{}
	_, _, pix = h.capture()
	if !bytes.Equal(pix, []byte{0, 0, 0, 0}) {
		t.Fatal("renderer without capture did not fall back to software")
	}
}
