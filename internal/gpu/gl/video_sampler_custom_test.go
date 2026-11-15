//go:build mygo_custom_backend && !js

package gl

import (
	"errors"
	"testing"

	"github.com/egoist/mygo/internal/scene"
)

func TestVideoSamplerOwnsEveryRenderedVersionUntilFrameEnd(t *testing.T) {
	r := &Renderer{}
	s := r.videoState(true)
	s.checked, s.program = true, 9
	t.Cleanup(func() { r.clearVideo(false) })
	imports, releases, copies := 0, 0, 0
	cleanupError := errors.New("GPU completion failed")
	registration := UseVideoSampler(func(native uintptr) *VideoTexture {
		if native != 42 {
			t.Fatal(native)
		}
		imports++
		return &VideoTexture{Texture: uint32(100 + imports), Width: 2, Height: 1, CropX: 1, CropY: 1, RangeScale: 1,
			Copy2D:  func(texture uint32) bool { copies++; return texture == 7 },
			Release: func() error { releases++; return cleanupError }}
	})
	t.Cleanup(registration)
	img := scene.NewImageRGBA(2, 1, nil)
	img.Surface = &scene.Surface{Native: 42}
	first := r.videoTexture(img, img.Surface)
	if first != 101 || r.videoTexture(img, img.Surface) != first || imports != 1 || releases != 0 {
		t.Fatal(imports, releases)
	}
	img.Changed()
	second := r.videoTexture(img, img.Surface)
	if second != 102 || imports != 2 || releases != 0 || len(s.textures) != 2 {
		t.Fatal("early retirement", imports, releases)
	}
	if r.videoProgram(first, nil) != 9 || r.videoProgram(second, &scene.Effect{}) != 0 {
		t.Fatal("sampler/program mismatch")
	}
	get := glGetIntegerv
	glGetIntegerv = func(_ uint32, p *int32) { *p = 7 }
	t.Cleanup(func() { glGetIntegerv = get })
	registration() // In-flight views survive unregistering the importer.
	if !r.uploadVideo(42) || copies != 1 {
		t.Fatal("owned 2D copy unavailable")
	}
	drawError := errors.New("draw failed")
	err := r.endVideoFrame(drawError)
	if !errors.Is(err, drawError) || !errors.Is(err, cleanupError) || releases != 2 || len(s.images) != 0 || len(s.textures) != 0 {
		t.Fatal(err, releases)
	}
	if err = r.endVideoFrame(nil); err != nil || releases != 2 {
		t.Fatal("double release", err, releases)
	}
	if r.videoTexture(img, img.Surface) != 0 {
		t.Fatal("registration survived release")
	}
}

func TestVideoSamplerRejectsInvalidMetadataAndOwnsRegistration(t *testing.T) {
	r := &Renderer{}
	s := r.videoState(true)
	s.checked, s.program = true, 9
	t.Cleanup(func() { r.clearVideo(false) })
	old := UseVideoSampler(func(uintptr) *VideoTexture { t.Fatal("retired importer"); return nil })
	released := 0
	current := UseVideoSampler(func(uintptr) *VideoTexture {
		return &VideoTexture{Texture: 7, Width: 2, Height: 1, CropX: 1, CropY: 1, RangeScale: -1, Release: func() error { released++; return nil }}
	})
	t.Cleanup(current)
	old()
	old()
	img := scene.NewImageRGBA(2, 1, nil)
	if r.videoTexture(img, &scene.Surface{Native: 1}) != 0 || released != 1 {
		t.Fatal("invalid range accepted", released)
	}
	_ = r.endVideoFrame(nil)
	if released != 1 {
		t.Fatal("invalid view released twice")
	}
}

func TestVideoSamplerRestoresExternalTextureAndSamplerBindings(t *testing.T) {
	r := &Renderer{}
	s := r.videoState(true)
	s.program = 9
	s.textures[101] = &sampledVideo{view: &VideoTexture{Texture: 101, Width: 2, Height: 1, CropX: 1, CropY: 1, RangeScale: 1}}
	get, active, bind, sampler, u1, u2 := glGetIntegerv, glActiveTexture, glBindTexture, glBindSampler, glUniform1i, glUniform2f
	t.Cleanup(func() {
		glGetIntegerv, glActiveTexture, glBindTexture, glBindSampler, glUniform1i, glUniform2f = get, active, bind, sampler, u1, u2
		r.clearVideo(false)
	})
	unit, texture, samplerID := uint32(2), uint32(73), uint32(19)
	texture2D := uint32(53)
	glActiveTexture = func(value uint32) { unit = value - glTexture0 }
	glGetIntegerv = func(name uint32, out *int32) {
		switch name {
		case 0x8D67:
			if unit != videoUnit {
				t.Fatal(unit)
			}
			*out = int32(texture)
		case 0x84E0:
			*out = int32(glTexture0 + unit)
		case 0x8919:
			if unit != videoUnit {
				t.Fatal(unit)
			}
			*out = int32(samplerID)
		case 0x8069:
			if unit != videoUnit {
				t.Fatal(unit)
			}
			*out = int32(texture2D)
		default:
			t.Fatal(name)
		}
	}
	glBindTexture = func(target, name uint32) {
		if unit != videoUnit {
			t.Fatal(target, unit)
		}
		switch target {
		case glTextureExternal:
			texture = name
		case glTexture2D:
			texture2D = name
		default:
			t.Fatal(target)
		}
	}
	glBindSampler = func(index, name uint32) {
		if index != videoUnit {
			t.Fatal(index)
		}
		samplerID = name
	}
	uniformFloat := glUniform1f
	t.Cleanup(func() { glUniform1f = uniformFloat })
	glUniform1f = func(int32, float32) {}
	glUniform1i = func(int32, int32) {}
	glUniform2f = func(int32, float32, float32) {}
	if !r.bindVideo(101, 9, 800, 600) || unit != 2 || texture != 101 || samplerID != 0 || texture2D != 0 {
		t.Fatal(unit, texture, samplerID, texture2D)
	}
	if err := r.endVideoFrame(nil); err != nil || unit != 2 || texture != 73 || samplerID != 19 || texture2D != 53 {
		t.Fatal(err, unit, texture, samplerID, texture2D)
	}
}
