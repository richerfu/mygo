//go:build mygo_custom_backend && !js

package gl

import (
	_ "embed"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/egoist/mygo/internal/gpu"
	"github.com/egoist/mygo/internal/scene"
)

type ColorEncoding uint8

const (
	ColorSRGB ColorEncoding = iota
	ColorExtendedSRGB
	ColorDisplayP3
	ColorBT2020PQ
)

// ColorOutput describes the encoding of the host's bound framebuffer, not a
// request to configure a display. The host must negotiate and tag that buffer.
// ReferenceWhiteNits defines extended sRGB's RGB 1; HLGPeakNits defines its HLG
// reference display. Zero selects 203 / 1000. Other values must be in 1..10000
// and 400..10000 respectively. HDR decoding uses zero reference black.
type ColorOutput struct {
	Encoding                        ColorEncoding
	ReferenceWhiteNits, HLGPeakNits float32
}

// ColorRenderer retains extended sRGB in half-float scene/intermediate buffers
// and converts only the completed scene to the negotiated output encoding.
// It is opt-in: the ordinary renderer and ordinary platform builds stay SDR.
// Like Renderer, use it serially with its context current; SetOutput only
// validates/stores a request and can be called before acquiring that context.
type ColorRenderer struct {
	renderer               *Renderer
	output, active         ColorOutput
	frame                  target
	present                uint32
	encoding, white        int32
	uniforms               map[uint32]int32
	upload                 func(native uintptr, texture uint32, white, peak float32) error
	uploadGeneration       uint64
	activeUploadGeneration uint64
	native                 map[uint64]*texture
	released               bool
}

var colorRenderers = struct {
	sync.RWMutex
	values map[*Renderer]*ColorRenderer
}{values: make(map[*Renderer]*ColorRenderer)}

func NewColorRenderer(retained bool) (*ColorRenderer, error) {
	r, err := New()
	if err != nil {
		return nil, err
	}
	r.Partial = retained
	return wrapColorRenderer(r), nil
}

func wrapColorRenderer(r *Renderer) *ColorRenderer {
	c := &ColorRenderer{renderer: r, uniforms: make(map[uint32]int32), native: make(map[uint64]*texture)}
	c.output = ColorOutput{ReferenceWhiteNits: 203, HLGPeakNits: 1000}
	c.active = c.output
	colorRenderers.Lock()
	colorRenderers.values[r] = c
	colorRenderers.Unlock()
	return c
}

func (r *Renderer) colorState() *ColorRenderer {
	colorRenderers.RLock()
	c := colorRenderers.values[r]
	colorRenderers.RUnlock()
	return c
}

func (c *ColorRenderer) SetOutput(output ColorOutput) error {
	if c.released {
		return fmt.Errorf("gl: color renderer is released")
	}
	if output.ReferenceWhiteNits == 0 {
		output.ReferenceWhiteNits = 203
	}
	if output.HLGPeakNits == 0 {
		output.HLGPeakNits = 1000
	}
	if output.Encoding > ColorBT2020PQ || !finiteRange(output.ReferenceWhiteNits, 1, 10000) || !finiteRange(output.HLGPeakNits, 400, 10000) {
		return fmt.Errorf("gl: invalid color output")
	}
	c.output = output
	return nil
}

func finiteRange(v, low, high float32) bool { return !math.IsNaN(float64(v)) && v >= low && v <= high }

// SetExtendedVideoUpload supplies the fallback when a native image cannot be
// imported for direct sampling. The callback fills an RGBA16F texture with
// premultiplied extended sRGB, preserving GL state and finishing source reads
// before returning. It runs synchronously under Render's current context.
// A wide render reports missing/failed conversion instead of silently using
// an SDR frame. SetOutput(ColorSRGB) lets a host explicitly fall back to SDR.
// Like SetOutput, call this serially with this renderer's other methods.
func (c *ColorRenderer) SetExtendedVideoUpload(upload func(native uintptr, texture uint32, white, peak float32) error) error {
	if c.released {
		return fmt.Errorf("gl: color renderer is released")
	}
	c.upload = upload
	c.uploadGeneration++
	c.renderer.Invalidate()
	return nil
}

func (c *ColorRenderer) clearNative() {
	for _, t := range c.native {
		glDeleteTextures(1, &t.tex)
	}
	clear(c.native)
}

func (c *ColorRenderer) clearIntermediates() {
	r := c.renderer
	c.clearNative()
	for _, t := range r.canvases {
		r.deleteCanvas(t)
	}
	clear(r.canvases)
	for i := range r.blurred {
		r.blurred[i].release()
	}
	for i := range r.blurTmp {
		r.blurTmp[i].release()
	}
	r.blurred = nil
	for _, t := range []*texture{&r.grab, &r.backdrop[0], &r.backdrop[1]} {
		if t.tex != 0 {
			glDeleteTextures(1, &t.tex)
		}
		*t = texture{}
	}
	r.Invalidate()
}

// Capture draws an SDR snapshot without changing the host window's output
// format or color metadata. The result is premultiplied RGBA, top row first.
// The renderer's context must be current, but it needs no window surface.
func (c *ColorRenderer) Capture(s *scene.Scene) (width, height int, rgba []byte, err error) {
	if c.released {
		return 0, 0, nil, fmt.Errorf("gl: color renderer is released")
	}
	if currentContext() == 0 {
		return 0, 0, nil, errNoContext
	}
	if s == nil || s.Width <= 0 || s.Height <= 0 || s.Width > c.renderer.maxSize || s.Height > c.renderer.maxSize {
		return 0, 0, nil, fmt.Errorf("gl: invalid capture dimensions")
	}
	var draw, read int32
	glGetIntegerv(glFramebufferBinding, &draw)
	glGetIntegerv(0x8CAA, &read)
	output := c.output
	var frame target
	defer func() {
		frame.release()
		glBindFramebuffer(glDrawFramebuffer, uint32(draw))
		glBindFramebuffer(glReadFramebuffer, uint32(read))
		c.output = output
		// A retained renderer must redraw the live target after this snapshot.
		c.Invalidate()
	}()
	if err := frame.fit(s.Width, s.Height); err != nil {
		return 0, 0, nil, err
	}
	c.output.Encoding = ColorSRGB
	c.Invalidate()
	if err := c.Render(s); err != nil {
		return 0, 0, nil, err
	}
	glBindFramebuffer(glReadFramebuffer, frame.fb)
	rgba, err = ReadFramebuffer(s.Width, s.Height)
	if err != nil {
		return 0, 0, nil, err
	}
	for i := 0; i < len(rgba); i += 4 {
		rgba[i], rgba[i+2] = rgba[i+2], rgba[i]
	}
	return s.Width, s.Height, rgba, nil
}

func (c *ColorRenderer) Render(s *scene.Scene) error {
	if c.released {
		return fmt.Errorf("gl: color renderer is released")
	}
	if s.Width <= 0 || s.Height <= 0 {
		return nil
	}
	r := c.renderer
	if currentContext() == 0 {
		return errNoContext
	}
	if id := currentContext(); id != r.context {
		// Objects of the old context belong to that context's owner.
		c.frame, c.present = target{}, 0
		clear(c.uniforms)
		clear(c.native)
		r.forget()
		r.context = id
		if err := r.init(); err != nil {
			return err
		}
	}
	if c.active != c.output || c.activeUploadGeneration != c.uploadGeneration {
		c.clearIntermediates()
		c.active = c.output
		c.activeUploadGeneration = c.uploadGeneration
	}
	r.diff.Wide = c.active.Encoding != ColorSRGB
	if c.active.Encoding == ColorSRGB {
		return r.Render(s)
	}
	if s.Width > r.maxSize || s.Height > r.maxSize {
		return fmt.Errorf("gl: color frame exceeds texture limits")
	}
	var draw, read int32
	glGetIntegerv(glFramebufferBinding, &draw)
	glGetIntegerv(0x8CAA, &read)
	defer func() {
		glBindFramebuffer(glDrawFramebuffer, uint32(draw))
		glBindFramebuffer(glReadFramebuffer, uint32(read))
	}()
	if err := validateColorTarget(c.active.Encoding, draw); err != nil {
		return err
	}
	changed := c.frame.w != s.Width || c.frame.h != s.Height
	if err := fitFloatTarget(&c.frame, s.Width, s.Height); err != nil {
		return err
	}
	if changed {
		r.Invalidate()
	}
	glBindFramebuffer(glFramebuffer, c.frame.fb)
	if err := r.Render(s); err != nil {
		// Upload/lease errors can arrive after a canvas was drawn. Its version
		// must not hide a failed conversion on the next render's retry.
		c.clearIntermediates()
		return err
	}
	if r.frame%120 == 0 {
		for id, t := range c.native {
			if r.frame-t.lastFrame >= 120 {
				glDeleteTextures(1, &t.tex)
				delete(c.native, id)
			}
		}
	}
	if c.present == 0 {
		var err error
		c.present, err = link(r.header+colorVertex, r.header+colorFragment)
		if err != nil {
			return err
		}
		c.encoding = glGetUniformLocation(c.present, "uEncoding")
		c.white = glGetUniformLocation(c.present, "uWhite")
		glUseProgram(c.present)
		glUniform1i(glGetUniformLocation(c.present, "uImage"), 0)
	}
	glBindFramebuffer(glDrawFramebuffer, uint32(draw))
	glViewport(0, 0, int32(s.Width), int32(s.Height))
	glDisable(glBlend)
	glDisable(glScissorTest)
	glUseProgram(c.present)
	glUniform1i(c.encoding, int32(c.active.Encoding))
	glUniform1f(c.white, c.active.ReferenceWhiteNits)
	glBindVertexArray(r.passVAO)
	glActiveTexture(glTexture0)
	var sampler int32
	glGetIntegerv(0x8919, &sampler)
	glBindSampler(0, 0)
	glBindTexture(glTexture2D, c.frame.tex)
	glDrawArraysInstanced(glTriangleStrip, 0, 4, 1)
	glBindTexture(glTexture2D, 0)
	glBindSampler(0, uint32(sampler))
	glBindVertexArray(0)
	glUseProgram(0)
	if err := glGetError(); err != glNoError {
		return fmt.Errorf("gl: color presentation failed (%#x)", err)
	}
	return nil
}

func validateColorTarget(encoding ColorEncoding, framebuffer int32) error {
	attachment := uint32(glColorAttachment0)
	if framebuffer == 0 {
		attachment = 0x0405 /* BACK */
	}
	var component, colorEncoding int32
	glGetFramebufferAttachmentParameteriv(glDrawFramebuffer, attachment, 0x8211, &component)
	glGetFramebufferAttachmentParameteriv(glDrawFramebuffer, attachment, 0x8210, &colorEncoding)
	var sizes [4]int32
	for i := range sizes {
		glGetFramebufferAttachmentParameteriv(glDrawFramebuffer, attachment, 0x8212+uint32(i), &sizes[i])
	}
	if e := glGetError(); e != glNoError {
		return fmt.Errorf("gl: color target query failed (%#x)", e)
	}
	// Presentation writes encoded values. An sRGB attachment would encode them
	// again; integer attachments cannot receive float fragment outputs.
	if colorEncoding != glLinear || component != glFloat && component != 0x8C17 /* UNSIGNED_NORMALIZED */ {
		return fmt.Errorf("gl: color output needs a linear float or normalized framebuffer attachment")
	}
	for _, bits := range sizes[:3] {
		if encoding == ColorExtendedSRGB && (component != glFloat || bits < 16) || encoding == ColorBT2020PQ && bits < 10 || bits < 8 {
			return fmt.Errorf("gl: output framebuffer cannot represent color encoding %d", encoding)
		}
	}
	if encoding == ColorExtendedSRGB && sizes[3] < 16 {
		return fmt.Errorf("gl: extended output needs a floating-point alpha channel")
	}
	return nil
}

func (c *ColorRenderer) Release() {
	if c.released {
		return
	}
	c.released = true
	colorRenderers.Lock()
	delete(colorRenderers.values, c.renderer)
	colorRenderers.Unlock()
	if currentContext() == c.renderer.context {
		c.clearNative()
		c.frame.release()
		if c.present != 0 {
			glDeleteProgram(c.present)
		}
	}
	c.frame, c.present = target{}, 0
	clear(c.uniforms)
	clear(c.native)
	c.upload = nil
	c.renderer.Release()
}

func (r *Renderer) wideOutput() (ColorOutput, bool) {
	c := r.colorState()
	if c == nil {
		return ColorOutput{}, false
	}
	return c.active, c.active.Encoding != ColorSRGB
}

func (r *Renderer) fallbackElementTexture(img *scene.Image, surface *scene.Surface) uintptr {
	c := r.colorState()
	if c == nil || c.active.Encoding == ColorSRGB {
		return r.uploadedElementTexture(img, surface)
	}
	fail := func(err error) uintptr {
		s := r.videoState(true)
		s.releaseErr = errors.Join(s.releaseErr, fmt.Errorf("gl: extended video upload: %w", err))
		return 0
	}
	if c.upload == nil {
		return fail(fmt.Errorf("native image has no HDR-preserving import or uploader"))
	}
	t := c.native[img.ID()]
	if t != nil && (t.w != img.W || t.h != img.H) {
		glDeleteTextures(1, &t.tex)
		delete(c.native, img.ID())
		t = nil
	}
	if t == nil {
		t = &texture{tex: newFloatTexture(img.W, img.H), w: img.W, h: img.H}
		c.native[img.ID()] = t
	}
	if t.ver != img.Version() || t.gen != c.uploadGeneration {
		if err := c.upload(surface.Native, t.tex, c.active.ReferenceWhiteNits, c.active.HLGPeakNits); err != nil {
			return fail(err)
		}
		t.ver, t.gen = img.Version(), c.uploadGeneration
	}
	t.lastFrame = r.frame
	return uintptr(t.tex)
}
func (r *Renderer) prepareWideBuilder(b *gpu.Builder) { _, b.Wide = r.wideOutput() }
func (r *Renderer) bindWide(program uint32) {
	var on int32
	c := r.colorState()
	if c != nil && c.active.Encoding != ColorSRGB {
		on = 1
	}
	location := int32(-1)
	if c != nil {
		var ok bool
		location, ok = c.uniforms[program]
		if !ok {
			location = glGetUniformLocation(program, "uWide")
			c.uniforms[program] = location
		}
	}
	if location >= 0 {
		glUniform1i(location, on)
	}
}
func (r *Renderer) newRenderTexture(w, h int) uint32 {
	if _, wide := r.wideOutput(); wide {
		return newFloatTexture(w, h)
	}
	return newTexture(glRGBA8, glRGBA, w, h, nil, 0)
}
func (r *Renderer) fitRenderTarget(t *target, w, h int) error {
	if _, wide := r.wideOutput(); wide {
		return fitFloatTarget(t, w, h)
	}
	return t.fit(w, h)
}
func newFloatTexture(w, h int) uint32 {
	var texture uint32
	glGenTextures(1, &texture)
	glBindTexture(glTexture2D, texture)
	glTexParameteri(glTexture2D, glTextureMinFilter, glLinear)
	glTexParameteri(glTexture2D, glTextureMagFilter, glLinear)
	glTexParameteri(glTexture2D, glTextureWrapS, glClampToEdge)
	glTexParameteri(glTexture2D, glTextureWrapT, glClampToEdge)
	glTexImage2D(glTexture2D, 0, 0x881A /* RGBA16F */, int32(w), int32(h), 0, glRGBA, 0x140B /* HALF_FLOAT */, nil)
	return texture
}
func fitFloatTarget(t *target, w, h int) error {
	if t.tex != 0 && t.w == w && t.h == h {
		return nil
	}
	if t.tex != 0 {
		glDeleteTextures(1, &t.tex)
	}
	t.tex = newFloatTexture(w, h)
	if t.fb == 0 {
		glGenFramebuffers(1, &t.fb)
	}
	glBindFramebuffer(glFramebuffer, t.fb)
	glFramebufferTexture2D(glFramebuffer, glColorAttachment0, glTexture2D, t.tex, 0)
	if glCheckFramebufferState(glFramebuffer) != glFramebufferDone {
		t.release()
		return fmt.Errorf("gl: half-float rendering is unavailable")
	}
	t.w, t.h = w, h
	return nil
}

func sceneShaderSource() string { return wideShaderSource }

var wideShaderSource = makeWideShaderSource()

func makeWideShaderSource() string {
	s := strings.Replace(shaderSource, "uniform vec2 uSize; // the frame's size in pixels", "uniform vec2 uSize;\nuniform bool uWide;\nvec3 wideClamp(vec3 c) { return uWide ? c : clamp(c,0.0,1.0); }", 1)
	s = strings.Replace(s, "return mix(pow((c + 0.055) / 1.055, vec3(2.4)), c / 12.92, lessThanEqual(c, vec3(0.04045)));", "vec3 a=abs(c); return sign(c)*mix(pow((a+.055)/1.055,vec3(2.4)),a/12.92,lessThanEqual(a,vec3(.04045)));", 1)
	start := strings.Index(s, "vec3 toSRGB(vec3 c) {")
	end := strings.Index(s[start:], "\n}") + start + 2
	s = s[:start] + "vec3 toSRGB(vec3 c) { vec3 a=abs(c); return sign(c)*mix(1.055*pow(a,vec3(1.0/2.4))-.055,12.92*a,lessThanEqual(a,vec3(.0031308))); }" + s[end:]
	s = strings.ReplaceAll(s, "clamp(fromOklab(mix(ap.yzw * a.a, bp.yzw * b.a, f) / alpha), 0.0, 1.0)", "wideClamp(fromOklab(mix(ap.yzw * a.a, bp.yzw * b.a, f) / alpha))")
	return strings.ReplaceAll(s, "clamp(fromOklab(lab / a), 0.0, 1.0)", "wideClamp(fromOklab(lab / a))")
}
func colorEffectSource(src string) string {
	s := strings.Replace(EffectSource(src), shaderSource, sceneShaderSource(), 1)
	s = strings.Replace(s, "struct Effect {", "struct Effect {\n\tbool wide;", 1)
	s = strings.Replace(s, "Effect e = Effect(", "Effect e = Effect(uWide, ", 1)
	return "#define MYGO_WIDE_COLOR\n" + s
}

const colorVertex = `out vec2 uv;
void main() { uv=vec2(float(gl_VertexID&1),float(gl_VertexID>>1)); gl_Position=vec4(uv*2.0-1.0,0.0,1.0); }`

//go:embed color_output.glsl
var colorFragment string

func (c *ColorRenderer) Invalidate()       { c.renderer.Invalidate() }
func (c *ColorRenderer) Software() bool    { return c.renderer.Software() }
func (c *ColorRenderer) FrameInfo() string { return c.renderer.FrameInfo() }
