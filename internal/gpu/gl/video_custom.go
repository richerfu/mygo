//go:build mygo_custom_backend && !js

package gl

import "sync"

type videoUploadBinding struct {
	upload func(uintptr, uint32) bool
}

var videoUpload struct {
	sync.RWMutex
	current *videoUploadBinding
}

func UseVideoUpload(upload func(uintptr, uint32) bool) func() {
	b := &videoUploadBinding{upload}
	videoUpload.Lock()
	videoUpload.current = b
	videoUpload.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			videoUpload.Lock()
			defer videoUpload.Unlock()
			if videoUpload.current == b {
				videoUpload.current = nil
			}
		})
	}
}

func uploadElement(native uintptr) bool {
	videoUpload.RLock()
	b := videoUpload.current
	videoUpload.RUnlock()
	if b == nil || b.upload == nil {
		return false
	}
	var texture int32
	glGetIntegerv(0x8069 /* GL_TEXTURE_BINDING_2D */, &texture)
	return texture != 0 && b.upload(native, uint32(texture))
}
