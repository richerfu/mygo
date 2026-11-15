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

// Some external-image drivers sample correctly only on unit zero. Keep the
// mask atlas on a separate unit so scene shaders never alias sampler types.
const videoUnit = 0
const maskUnit = 11
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
	wide    target
}
type videoRenderer struct {
	checked                                                           bool
	program                                                           uint32
	size, rotation, flip, transfer, gamut, crop, codeRange, imageSize int32
	images                                                            map[videoImageKey]*sampledVideo
	textures                                                          map[uintptr]*sampledVideo
	bound                                                             bool
	previousTexture, previousSampler, previous2D                      int32
	releaseErr                                                        error
	wide, referenceWhite, hlgPeak                                     int32
	conversion                                                        *videoRenderer
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
	source := sceneShaderSource()
	// External-image drivers can inspect entry points before preprocessing.
	// Omit the unused vertex and backdrop passes rather than exposing their
	// inactive main functions to the fragment shader's external-image lowering.
	start := strings.Index(source, "#if defined(FRAGMENT) || defined(EFFECT)")
	end := strings.Index(source, "// The passes computing the backdrop")
	if start >= 0 && end > start {
		source = source[start:end]
	}
	// uImage stays a 2D sampler for the shader's unused NV12 branch. The
	// external sampler has a separate unit; two sampler types cannot share one.
	source = strings.Replace(source, "res = sampleImage(uImage, tex, vWidths, vParams.z, dtex);", "res = sampleVideo(tex, vWidths, vParams.z, dtex);", 1)
	return header + videoSource + "\n#define FRAGMENT\n" + source
}

func (r *Renderer) prepareVideo(s *videoRenderer) bool {
	if s.checked {
		return s.program != 0
	}
	s.checked = true
	if !r.es || !hasExtension("GL_OES_EGL_image_external_essl3") {
		return false
	}
	program, err := link(r.header+"#define VERTEX\n"+sceneShaderSource(), videoFragment(r.header))
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
	for name, unit := range map[string]int32{"uMask": maskUnit, "uColor": 1, "uImage": 2, "uClipMask": 4, "uStops": stopsUnit, "uChroma": chromaUnit, "uVideo": videoUnit} {
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
	s.wide = glGetUniformLocation(program, "uVideoWide")
	s.referenceWhite = glGetUniformLocation(program, "uVideoWhite")
	s.hlgPeak = glGetUniformLocation(program, "uVideoPeak")
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
	t := &sampledVideo{view: v, image: image, surface: *surface}
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
	r.bindExternal(s, v)
	r.setVideoUniforms(s, v, width, height)
	return true
}

func (r *Renderer) bindExternal(s *videoRenderer, v *VideoTexture) {
	glActiveTexture(glTexture0 + videoUnit)
	if !s.bound {
		glGetIntegerv(0x8D67 /* TEXTURE_BINDING_EXTERNAL_OES */, &s.previousTexture)
		glGetIntegerv(0x8919 /* SAMPLER_BINDING of the active unit */, &s.previousSampler)
		glGetIntegerv(0x8069 /* TEXTURE_BINDING_2D */, &s.previous2D)
		s.bound = true
	}
	glBindSampler(videoUnit, 0)
	// External-image drivers may resolve the unit's 2D binding even with an
	// external sampler. No scene sampler2D uses this reserved unit.
	glBindTexture(glTexture2D, 0)
	glBindTexture(glTextureExternal, v.Texture)
	glActiveTexture(glTexture0 + 2)
}

func (r *Renderer) setVideoUniforms(s *videoRenderer, v *VideoTexture, width, height int) {
	glUniform2f(s.size, float32(width), float32(height))
	glUniform1i(s.rotation, int32(v.Rotation))
	glUniform1i(s.flip, int32(v.Flip))
	glUniform1i(s.transfer, int32(v.Transfer))
	glUniform1i(s.gamut, int32(v.Gamut))
	glUniform2f(s.crop, v.CropX, v.CropY)
	glUniform2f(s.codeRange, v.RangeOffset, v.RangeScale)
	glUniform2f(s.imageSize, float32(v.Width), float32(v.Height))
	var wide int32
	white, peak := float32(203), float32(1000)
	if output, on := r.wideOutput(); on {
		wide = 1
		white, peak = output.ReferenceWhiteNits, output.HLGPeakNits
	}
	glUniform1i(s.wide, wide)
	glUniform1f(s.referenceWhite, white)
	glUniform1f(s.hlgPeak, peak)
}

func (r *Renderer) videoTexture2D(texture uintptr) uintptr {
	if s := r.videoState(false); s != nil {
		if t := s.textures[texture]; t != nil {
			if _, wide := r.wideOutput(); wide {
				return r.floatVideoTexture(s, t)
			}
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
		glBindTexture(glTexture2D, uint32(s.previous2D))
		glBindTexture(glTextureExternal, uint32(s.previousTexture))
		glBindSampler(videoUnit, uint32(s.previousSampler))
		glActiveTexture(uint32(active))
		s.bound = false
	}
	err = errors.Join(err, s.releaseErr)
	s.releaseErr = nil
	for _, t := range s.images {
		t.wide.release()
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
		if s.conversion != nil {
			glDeleteProgram(s.conversion.program)
		}
	}
}

func videoCopyFragment(header string) string {
	header = strings.Replace(header, "\n", "\n#extension GL_OES_EGL_image_external_essl3 : require\n", 1)
	return header + videoSource + "\nin vec2 uv; out vec4 color; void main(){ color=videoTexel(floor(uv*uVideoSize)); }"
}

func (r *Renderer) floatVideoTexture(s *videoRenderer, t *sampledVideo) uintptr {
	if t.wide.tex != 0 {
		return uintptr(t.wide.tex)
	}
	var program, vao, draw, read, active, binding int32
	var viewport [4]int32
	glGetIntegerv(0x8B8D, &program)
	glGetIntegerv(0x85B5, &vao)
	glGetIntegerv(glFramebufferBinding, &draw)
	glGetIntegerv(0x8CAA, &read)
	glGetIntegerv(0x84E0, &active)
	glGetIntegerv(0x8069, &binding)
	glGetIntegerv(0x0BA2, &viewport[0])
	blend, scissor := glIsEnabled(glBlend), glIsEnabled(glScissorTest)
	defer func() {
		glUseProgram(uint32(program))
		glBindVertexArray(uint32(vao))
		glBindFramebuffer(glDrawFramebuffer, uint32(draw))
		glBindFramebuffer(glReadFramebuffer, uint32(read))
		glViewport(viewport[0], viewport[1], viewport[2], viewport[3])
		if blend {
			glEnable(glBlend)
		} else {
			glDisable(glBlend)
		}
		if scissor {
			glEnable(glScissorTest)
		} else {
			glDisable(glScissorTest)
		}
		glActiveTexture(uint32(active))
		glBindTexture(glTexture2D, uint32(binding))
	}()
	fail := func(err error) uintptr { s.releaseErr = errors.Join(s.releaseErr, err); t.wide.release(); return 0 }
	if s.conversion == nil {
		p, err := link(r.header+colorVertex, videoCopyFragment(r.header))
		if err != nil {
			return fail(err)
		}
		s.conversion = &videoRenderer{}
		s.conversion.configureProgram(p)
	}
	if err := fitFloatTarget(&t.wide, t.view.Width, t.view.Height); err != nil {
		return fail(err)
	}
	glBindFramebuffer(glFramebuffer, t.wide.fb)
	glUseProgram(s.conversion.program)
	r.bindExternal(s, t.view)
	r.setVideoUniforms(s.conversion, t.view, t.view.Width, t.view.Height)
	glBindVertexArray(r.passVAO)
	glViewport(0, 0, int32(t.view.Width), int32(t.view.Height))
	glDisable(glBlend)
	glDisable(glScissorTest)
	glDrawArraysInstanced(glTriangleStrip, 0, 4, 1)
	if e := glGetError(); e != glNoError {
		return fail(fmt.Errorf("gl: wide video conversion failed (%#x)", e))
	}
	return uintptr(t.wide.tex)
}
