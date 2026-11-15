//go:build (!mygo_custom_backend && linux && (amd64 || arm64)) || js

package gl

import (
	"github.com/egoist/mygo/internal/gpu"
	"github.com/egoist/mygo/internal/scene"
)

func (r *Renderer) fallbackElementTexture(img *scene.Image, s *scene.Surface) uintptr {
	return r.uploadedElementTexture(img, s)
}

func sceneShaderSource() string                             { return shaderSource }
func colorEffectSource(src string) string                   { return EffectSource(src) }
func (*Renderer) prepareWideBuilder(*gpu.Builder)           {}
func (*Renderer) bindWide(uint32)                           {}
func (*Renderer) newRenderTexture(w, h int) uint32          { return newTexture(glRGBA8, glRGBA, w, h, nil, 0) }
func (*Renderer) fitRenderTarget(t *target, w, h int) error { return t.fit(w, h) }
