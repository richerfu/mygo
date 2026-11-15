//go:build mygo_custom_backend && !js

package gl

import (
	"github.com/egoist/mygo/internal/scene"
	"testing"
	"unsafe"
)

func TestExternalVideoUploadCacheAndCPUFallback(t *testing.T) {
	gen, bind, param, alloc, sub, store, del := glGenTextures, glBindTexture, glTexParameteri, glTexImage2D, glTexSubImage2D, glPixelStorei, glDeleteTextures
	get := glGetIntegerv
	t.Cleanup(func() {
		glGenTextures, glBindTexture, glTexParameteri, glTexImage2D, glTexSubImage2D, glPixelStorei, glDeleteTextures = gen, bind, param, alloc, sub, store, del
		glGetIntegerv = get
	})
	next, deleted := uint32(0), 0
	var bound uint32
	glGenTextures = func(_ int32, out *uint32) { next++; *out = next }
	glBindTexture = func(_ uint32, texture uint32) { bound = texture }
	glGetIntegerv = func(name uint32, out *int32) {
		if name != 0x8069 {
			t.Fatal("unexpected texture query", name)
		}
		*out = int32(bound)
	}
	glTexParameteri = func(uint32, uint32, int32) {}
	glPixelStorei = func(uint32, int32) {}
	glDeleteTextures = func(_ int32, _ *uint32) { deleted++ }
	glTexImage2D = func(_ uint32, _, _, _, _, _ int32, _, _ uint32, p unsafe.Pointer) {
		if p != nil {
			t.Fatal("CPU upload on GPU allocation")
		}
	}
	cpuUploads := 0
	glTexSubImage2D = func(_ uint32, _, _, _, _, _ int32, _, _ uint32, p unsafe.Pointer) {
		cpuUploads++
		if p == nil {
			t.Fatal("missing restored pixels")
		}
	}
	image := scene.NewImageRGBA(1, 1, nil)
	uploads, restores := 0, 0
	succeed := true
	image.Surface = &scene.Surface{Native: 42}
	t.Cleanup(UseVideoUpload(func(native uintptr, texture uint32) bool {
		uploads++
		if native != 42 || texture == 0 {
			t.Fatal("wrong frame or no target", native, texture)
		}
		return succeed
	}))
	image.Restore = func() []byte { restores++; return []byte{255, 0, 0, 255} }
	renderer := &Renderer{images: map[uint64]*texture{}, maxSize: 1024}
	first := renderer.imageTexture(image)
	if first == 0 || uploads != 1 || restores != 0 || cpuUploads != 0 {
		t.Fatal(first, uploads, restores, cpuUploads)
	}
	if again := renderer.imageTexture(image); again != first || uploads != 1 {
		t.Fatal("unchanged video uploaded twice")
	}
	image.Changed()
	renderer.imageTexture(image)
	if uploads != 2 || restores != 0 {
		t.Fatal("new frame not uploaded on GPU")
	}
	succeed = false
	image.Changed()
	renderer.imageTexture(image)
	if uploads != 3 || restores != 1 || cpuUploads != 1 {
		t.Fatal("CPU restore not used on unsupported import", uploads, restores, cpuUploads)
	}
	succeed = true
	image.Resize(2, 1)
	renderer.imageTexture(image)
	if uploads != 4 || deleted != 1 || next != 2 {
		t.Fatal("resize retained old target", uploads, deleted, next)
	}
}

func TestVideoUploadRegistrationOwnership(t *testing.T) {
	get := glGetIntegerv
	t.Cleanup(func() { glGetIntegerv = get })
	glGetIntegerv = func(_ uint32, out *int32) { *out = 7 }
	old := UseVideoUpload(func(uintptr, uint32) bool { t.Fatal("disposed uploader"); return false })
	current := UseVideoUpload(func(native uintptr, texture uint32) bool { return native == 42 && texture == 7 })
	t.Cleanup(current)
	old()
	old()
	if !uploadElement(42) {
		t.Fatal("old release removed current uploader")
	}
	current()
	current()
	if uploadElement(42) {
		t.Fatal("uploader survived release")
	}
}
