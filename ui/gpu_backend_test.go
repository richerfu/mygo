package ui

import (
	"github.com/egoist/mygo/internal/gpu"
	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/internal/scene"
	"github.com/egoist/mygo/internal/surface"
	"testing"
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
