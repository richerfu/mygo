//go:build (!mygo_custom_backend && linux && (amd64 || arm64)) || js

package gl

import "github.com/egoist/mygo/internal/scene"

const directVideo = false
const maskUnit = 0

func (*Renderer) videoTexture(*scene.Image, *scene.Surface) uintptr { return 0 }
func (*Renderer) videoProgram(uintptr, *scene.Effect) uint32        { return 0 }
func (*Renderer) bindVideo(uintptr, uint32, int, int) bool          { return false }
func (*Renderer) videoTexture2D(texture uintptr) uintptr            { return texture }
func (*Renderer) endVideoFrame(err error) error                     { return err }
func (*Renderer) clearVideo(bool)                                   {}
func (*Renderer) uploadVideo(native uintptr) bool                   { return uploadElement(native) }
