//go:build mygo_custom_backend && !js

package gl

import (
	_ "embed"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/egoist/mygo/internal/scene"
)

const directVideo = true
const videoUnit = 11
const glTextureExternal = 0x8D65

// VideoTexture owns an external OES texture until Release. Rotation is clockwise
// quarter turns; Flip is 0 (none), 1 (horizontal), or 2 (vertical), before rotation.
// Transfer is 0 (SDR), 1 (PQ), 2 (HLG), 3 (sRGB), or 4 (linear). Gamut is 0
// (BT.709), 1 (BT.2020), or 2 (Display P3). Crop is visible/allocation size;
// encoded RGB is normalized as (sample - RangeOffset) * RangeScale.
type VideoTexture struct {
	Texture                               uint32
	Width, Height                         int
	Rotation, Flip, Transfer, Gamut       int
	CropX, CropY, RangeOffset, RangeScale float32
	// Copy2D optionally converts this same owned frame into an existing RGBA8
	// texture for operations requiring sampler2D. It must preserve GL state.
	Copy2D func(texture uint32) bool
	// Release runs with the creating context current and must finish GPU reads.
	Release func() error
}

type videoSamplerBinding struct{ importFrame func(uintptr) *VideoTexture }
type videoImageKey struct {
	id, version uint64
	native      uintptr
}
type sampledVideo struct {
	view    *VideoTexture
	image   *scene.Image
	surface scene.Surface
}
type videoRenderer struct {
	checked                                                           bool
	program                                                           uint32
	size, rotation, flip, transfer, gamut, crop, codeRange, imageSize int32
	images                                                            map[videoImageKey]*sampledVideo
	textures                                                          map[uintptr]*sampledVideo
	bound                                                             bool
	previousTexture, previousSampler                                  int32
	releaseErr                                                        error
}

var videoSamplers = struct {
	sync.Mutex
	current   *videoSamplerBinding
	renderers map[*Renderer]*videoRenderer
}{renderers: make(map[*Renderer]*videoRenderer)}

func UseVideoSampler(importFrame func(uintptr) *VideoTexture) func() {
	b := &videoSamplerBinding{importFrame}
	videoSamplers.Lock()
	videoSamplers.current = b
	videoSamplers.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			videoSamplers.Lock()
			defer videoSamplers.Unlock()
			if videoSamplers.current == b {
				videoSamplers.current = nil
			}
		})
	}
}

func (r *Renderer) videoState(create bool) *videoRenderer {
	videoSamplers.Lock()
	defer videoSamplers.Unlock()
	s := videoSamplers.renderers[r]
	if s == nil && create {
		s = &videoRenderer{images: make(map[videoImageKey]*sampledVideo), textures: make(map[uintptr]*sampledVideo)}
		videoSamplers.renderers[r] = s
	}
	return s
}

//go:embed video_external.glsl
var videoSource string

func videoFragment(header string) string {
	header = strings.Replace(header, "\n", "\n#extension GL_OES_EGL_image_external_essl3 : require\n", 1)
	source := strings.Replace(shaderSource, "uniform sampler2D uImage;", "uniform sampler2D uImage;\n"+videoSource, 1)
	// uImage stays a 2D sampler for the shader's unused NV12 branch. The
	// external sampler has a separate unit; two sampler types cannot share one.
	source = strings.Replace(source, "res = sampleImage(uImage, tex, vWidths, vParams.z, dtex);", "res = sampleVideo(tex, vWidths, vParams.z, dtex);", 1)
	return header + "#define FRAGMENT\n" + source
}

func (r *Renderer) prepareVideo(s *videoRenderer) bool {
	if s.checked {
		return s.program != 0
	}
	s.checked = true
	if !r.es || !hasExtension("GL_OES_EGL_image_external_essl3") {
		return false
	}
	program, err := link(r.header+"#define VERTEX\n"+shaderSource, videoFragment(r.header))
	if err != nil {
		return false
	} // The upload/CPU path remains available.
	s.configureProgram(program)
	return true
}

func (s *videoRenderer) configureProgram(program uint32) {
	s.program = program
	var previous int32
	glGetIntegerv(0x8B8D /* CURRENT_PROGRAM */, &previous)
	glUseProgram(program)
	for name, unit := range map[string]int32{"uMask": 0, "uColor": 1, "uImage": 2, "uClipMask": 4, "uStops": stopsUnit, "uChroma": chromaUnit, "uVideo": videoUnit} {
		glUniform1i(glGetUniformLocation(program, name), unit)
	}
	s.size = glGetUniformLocation(program, "uSize")
	s.rotation = glGetUniformLocation(program, "uVideoRotation")
	s.flip = glGetUniformLocation(program, "uVideoFlip")
	s.transfer = glGetUniformLocation(program, "uVideoTransfer")
	s.gamut = glGetUniformLocation(program, "uVideoGamut")
	s.crop = glGetUniformLocation(program, "uVideoCrop")
	s.codeRange = glGetUniformLocation(program, "uVideoRange")
	s.imageSize = glGetUniformLocation(program, "uVideoSize")
	glUseProgram(uint32(previous))
}

func validVideo(v *VideoTexture, image *scene.Image) bool {
	if v.Texture == 0 || v.Release == nil || v.Width != image.W || v.Height != image.H ||
		v.Rotation < 0 || v.Rotation > 3 || v.Flip < 0 || v.Flip > 2 || v.Transfer < 0 || v.Transfer > 4 || v.Gamut < 0 || v.Gamut > 2 {
		return false
	}
	for _, x := range []float32{v.CropX, v.CropY, v.RangeOffset, v.RangeScale} {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return false
		}
	}
	return v.CropX > 0 && v.CropX <= 1 && v.CropY > 0 && v.CropY <= 1 && v.RangeScale > 0
}

func (r *Renderer) videoTexture(image *scene.Image, surface *scene.Surface) uintptr {
	videoSamplers.Lock()
	binding := videoSamplers.current
	videoSamplers.Unlock()
	if binding == nil || binding.importFrame == nil {
		return 0
	}
	s := r.videoState(true)
	if !r.prepareVideo(s) {
		return 0
	}
	key := videoImageKey{image.ID(), image.Version(), surface.Native}
	if existing := s.images[key]; existing != nil {
		return uintptr(existing.view.Texture)
	}
	v := binding.importFrame(surface.Native)
	if v == nil {
		return 0
	}
	if !validVideo(v, image) {
		if v.Release != nil {
			s.releaseErr = errors.Join(s.releaseErr, v.Release())
		}
		return 0
	}
	t := &sampledVideo{v, image, *surface}
	s.images[key], s.textures[uintptr(v.Texture)] = t, t
	return uintptr(v.Texture)
}

func (r *Renderer) videoProgram(texture uintptr, effect *scene.Effect) uint32 {
	if effect == nil {
		if s := r.videoState(false); s != nil && s.textures[texture] != nil {
			return s.program
		}
	}
	return 0
}

func (r *Renderer) bindVideo(texture uintptr, program uint32, width, height int) bool {
	s := r.videoState(false)
	if s == nil || s.textures[texture] == nil {
		return false
	}
	if program != s.program {
		glBindTexture(glTexture2D, uint32(r.videoTexture2D(texture)))
		return true
	}
	v := s.textures[texture].view
	glActiveTexture(glTexture0 + videoUnit)
	if !s.bound {
		glGetIntegerv(0x8D67 /* TEXTURE_BINDING_EXTERNAL_OES */, &s.previousTexture)
		glGetIntegerv(0x8919 /* SAMPLER_BINDING of the active unit */, &s.previousSampler)
		s.bound = true
	}
	glBindSampler(videoUnit, 0)
	glBindTexture(glTextureExternal, v.Texture)
	glActiveTexture(glTexture0 + 2)
	glUniform2f(s.size, float32(width), float32(height))
	glUniform1i(s.rotation, int32(v.Rotation))
	glUniform1i(s.flip, int32(v.Flip))
	glUniform1i(s.transfer, int32(v.Transfer))
	glUniform1i(s.gamut, int32(v.Gamut))
	glUniform2f(s.crop, v.CropX, v.CropY)
	glUniform2f(s.codeRange, v.RangeOffset, v.RangeScale)
	glUniform2f(s.imageSize, float32(v.Width), float32(v.Height))
	return true
}

func (r *Renderer) videoTexture2D(texture uintptr) uintptr {
	if s := r.videoState(false); s != nil {
		if t := s.textures[texture]; t != nil {
			return r.uploadedElementTexture(t.image, &t.surface)
		}
	}
	return texture
}

func (r *Renderer) uploadVideo(native uintptr) bool {
	if s := r.videoState(false); s != nil {
		for key, t := range s.images {
			if key.native == native && t.view.Copy2D != nil {
				var texture int32
				glGetIntegerv(0x8069, &texture)
				return texture != 0 && t.view.Copy2D(uint32(texture))
			}
		}
	}
	return uploadElement(native)
}

func (r *Renderer) endVideoFrame(err error) error {
	s := r.videoState(false)
	if s == nil {
		return err
	}
	if s.bound {
		var active int32
		glGetIntegerv(0x84E0 /* ACTIVE_TEXTURE */, &active)
		glActiveTexture(glTexture0 + videoUnit)
		glBindTexture(glTextureExternal, uint32(s.previousTexture))
		glBindSampler(videoUnit, uint32(s.previousSampler))
		glActiveTexture(uint32(active))
		s.bound = false
	}
	err = errors.Join(err, s.releaseErr)
	s.releaseErr = nil
	for _, t := range s.images {
		if e := t.view.Release(); e != nil {
			err = errors.Join(err, fmt.Errorf("gl: release video frame: %w", e))
		}
	}
	clear(s.images)
	clear(s.textures)
	return err
}

func (r *Renderer) clearVideo(current bool) {
	videoSamplers.Lock()
	s := videoSamplers.renderers[r]
	delete(videoSamplers.renderers, r)
	videoSamplers.Unlock()
	if s != nil && current && s.program != 0 {
		glDeleteProgram(s.program)
	}
}
