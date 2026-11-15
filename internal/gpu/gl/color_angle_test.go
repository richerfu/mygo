//go:build mygo_custom_backend && darwin

package gl

import (
	"errors"
	"math"
	"os"
	"strings"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo/internal/gamut"
	"github.com/egoist/mygo/internal/scene"
)

func colorScene(c [4]float32) *scene.Scene {
	return &scene.Scene{Width: 16, Height: 16, Ops: []scene.Op{{Kind: scene.OpFill, Rect: scene.Rect{W: 16, H: 16}, Color: scene.Color{R: 128, G: 64, B: 32, A: 255}, Wide: 1}}, Wide: []scene.WideColors{{Set: scene.WideColor, Color: c}}}
}
func readColor(t *testing.T, x, y int) [4]float32 {
	t.Helper()
	var p [4]float32
	glReadPixels(int32(x), int32(y), 1, 1, glRGBA, glFloat, unsafe.Pointer(&p[0]))
	if e := glGetError(); e != glNoError {
		t.Fatalf("float read: %#x", e)
	}
	return p
}
func assertColor(t *testing.T, got, want [4]float32, tolerance float32) {
	t.Helper()
	for i := range got {
		if math.IsNaN(float64(got[i])) || float32(math.Abs(float64(got[i]-want[i]))) > tolerance {
			t.Fatalf("color %v, want %v (tolerance %g)", got, want, tolerance)
		}
	}
}
func colorTarget(t *testing.T) *target {
	t.Helper()
	out := &target{}
	if err := fitFloatTarget(out, 16, 16); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(out.release)
	return out
}
func TestWideSceneOnANGLE(t *testing.T) {
	r, _ := newANGLE(t)
	c := wrapColorRenderer(r)
	t.Cleanup(c.Release)
	out := colorTarget(t)
	if err := c.SetOutput(ColorOutput{Encoding: ColorExtendedSRGB}); err != nil {
		t.Fatal(err)
	}
	r.Partial = true
	sc := colorScene([4]float32{2, -.25, .5, .5})
	draw := func(s *scene.Scene) {
		t.Helper()
		glBindFramebuffer(glFramebuffer, out.fb)
		if err := c.Render(s); err != nil {
			t.Fatal(err)
		}
	}
	draw(sc)
	assertColor(t, readColor(t, 8, 8), [4]float32{1, -.125, .25, .5}, .002)
	draw(sc)
	if r.redrew != redrewNothing {
		t.Fatal("retained scene redrawn", r.FrameInfo())
	}
	sc.Wide[0].Color = [4]float32{3, -.5, .25, .5}
	draw(sc)
	assertColor(t, readColor(t, 8, 8), [4]float32{1.5, -.25, .125, .5}, .002)
	// Every retained canvas must change format when the output changes, even
	// when its scene and version are unchanged.
	canvas := scene.NewCanvas(16, 16)
	canvas.Scene = sc
	cs := &scene.Scene{Width: 16, Height: 16, Ops: []scene.Op{{Kind: scene.OpImage, Rect: scene.Rect{W: 16, H: 16}, Image: canvas, Src: scene.Rect{W: 16, H: 16}}}}
	draw(cs)
	assertColor(t, readColor(t, 8, 8), [4]float32{1.5, -.25, .125, .5}, .003)
	cs.Ops[0].Blur = 1
	draw(cs)
	assertColor(t, readColor(t, 8, 8), [4]float32{1.5, -.25, .125, .5}, .006)
	cs.Ops[0].Blur = 0
	if err := c.SetOutput(ColorOutput{}); err != nil {
		t.Fatal(err)
	}
	draw(cs)
	assertColor(t, readColor(t, 8, 8), [4]float32{128.0 / 255, 64.0 / 255, 32.0 / 255, 1}, .005)
	if err := c.SetOutput(ColorOutput{Encoding: ColorExtendedSRGB}); err != nil {
		t.Fatal(err)
	}
	draw(cs)
	assertColor(t, readColor(t, 8, 8), [4]float32{1.5, -.25, .125, .5}, .003)
	c.Release()
	c.Release()
	if err := c.Render(sc); err == nil {
		t.Fatal("render after release")
	}
	if err := c.SetOutput(ColorOutput{}); err == nil {
		t.Fatal("configure after release")
	}
}

func TestColorPresentationOnANGLE(t *testing.T) {
	r, _ := newANGLE(t)
	c := wrapColorRenderer(r)
	t.Cleanup(c.Release)
	out := colorTarget(t)
	// Matrix values are CSS Color 4's linear sRGB -> P3 / BT.2020 matrices.
	matrices := map[ColorEncoding][3][3]float64{
		ColorDisplayP3: {{.82246197, .17753803, 0}, {.03319420, .96680580, 0}, {.01708263, .07239744, .91051993}},
		ColorBT2020PQ:  {{.62740390, .32928304, .04331306}, {.06909729, .91954040, .01136231}, {.01639144, .08801331, .89559525}},
	}
	for encoding, matrix := range matrices {
		for _, rgb := range [][4]float32{{1, 1, 1, 1}, {2, .1, -.1, .5}, {-.25, 1.2, .3, .75}, {0, 0, 0, 0}} {
			for _, white := range []float32{80, 203, 1000} {
				if err := c.SetOutput(ColorOutput{Encoding: encoding, ReferenceWhiteNits: white}); err != nil {
					t.Fatal(err)
				}
				glBindFramebuffer(glFramebuffer, out.fb)
				if err := c.Render(colorScene(rgb)); err != nil {
					t.Fatal(err)
				}
				linear := [3]float64{}
				for i := range linear {
					a := math.Abs(float64(rgb[i]))
					if a <= .04045 {
						a /= 12.92
					} else {
						a = math.Pow((a+.055)/1.055, 2.4)
					}
					linear[i] = math.Copysign(a, float64(rgb[i]))
				}
				want := [4]float32{0, 0, 0, rgb[3]}
				for i, row := range matrix {
					value := row[0]*linear[0] + row[1]*linear[1] + row[2]*linear[2]
					if encoding == ColorDisplayP3 {
						value = min(max(value, 0), 1)
						if value <= .0031308 {
							value *= 12.92
						} else {
							value = 1.055*math.Pow(value, 1/2.4) - .055
						}
					} else {
						p := math.Pow(min(max(value*float64(white)/10000, 0), 1), 2610.0/16384)
						value = math.Pow((3424.0/4096+2413.0/128*p)/(1+2392.0/128*p), 2523.0/32)
					}
					want[i] = float32(value) * rgb[3]
				}
				assertColor(t, readColor(t, 8, 8), want, .0025)
			}
		}
	}
	valid := c.output
	for _, bad := range []ColorOutput{{Encoding: 99}, {ReferenceWhiteNits: float32(math.NaN())}, {ReferenceWhiteNits: -1}, {ReferenceWhiteNits: 10001}, {HLGPeakNits: 399}, {HLGPeakNits: 10001}} {
		if c.SetOutput(bad) == nil || c.output != valid {
			t.Fatal("invalid output accepted", bad)
		}
	}
	// The normal eight-bit window cannot masquerade as an HDR output.
	glBindFramebuffer(glFramebuffer, 0)
	for _, encoding := range []ColorEncoding{ColorExtendedSRGB, ColorBT2020PQ} {
		if err := c.SetOutput(ColorOutput{Encoding: encoding}); err != nil {
			t.Fatal(err)
		}
		if err := c.Render(colorScene([4]float32{2, 1, 0, 1})); err == nil {
			t.Fatal("accepted eight-bit HDR target", encoding)
		}
	}
	// Reject double transfer encoding in an sRGB FBO.
	srgb := newTexture(0x8C43, glRGBA, 16, 16, nil, 0)
	defer glDeleteTextures(1, &srgb)
	glBindFramebuffer(glFramebuffer, out.fb)
	glFramebufferTexture2D(glFramebuffer, glColorAttachment0, glTexture2D, srgb, 0)
	if err := c.SetOutput(ColorOutput{Encoding: ColorDisplayP3}); err != nil {
		t.Fatal(err)
	}
	if err := c.Render(colorScene([4]float32{1, 1, 1, 1})); err == nil {
		t.Fatal("accepted double encoding target")
	}
}

func TestWideBackdropEffectsOnANGLE(t *testing.T) {
	r, _ := newANGLE(t)
	c := wrapColorRenderer(r)
	t.Cleanup(c.Release)
	out := colorTarget(t)
	if err := c.SetOutput(ColorOutput{Encoding: ColorExtendedSRGB}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"identity", "blur", "glass"} {
		source := "vec4 effect(vec2 p, Effect e) { return vec4(sampleBackdrop(e,p),1.0); }"
		var params [5][4]float32
		want := [4]float32{2, -.25, .5, 1}
		if name != "identity" {
			data, err := os.ReadFile("../../../plugins/glass/" + name + ".glsl")
			if err != nil {
				t.Fatal(err)
			}
			source = string(data)
			if name == "blur" {
				params[1] = [4]float32{1, 1, 0, 1}
			} else {
				params[1] = [4]float32{.1, .2, .3, 1}
				params[4] = want
			}
		}
		sc := colorScene(want)
		sc.Effects = []scene.EffectOp{{Effect: &scene.Effect{Name: name, GLSL: source, Backdrop: true}, Params: params}}
		sc.Ops = append(sc.Ops, scene.Op{Kind: scene.OpEffect, Rect: scene.Rect{W: 16, H: 16}})
		glBindFramebuffer(glFramebuffer, out.fb)
		if err := c.Render(sc); err != nil {
			t.Fatal(name, err)
		}
		assertColor(t, readColor(t, 8, 8), want, .004)
	}
	// Shader-wide mode must also reach normal custom shader inputs.
	source := `vec4 effect(vec2 p, Effect e) { return e.wide ? vec4(2.0,-.5,.25,1.0) : vec4(0.0); }`
	sc := &scene.Scene{Width: 16, Height: 16, Ops: []scene.Op{{Kind: scene.OpEffect, Rect: scene.Rect{W: 16, H: 16}}}, Effects: []scene.EffectOp{{Effect: &scene.Effect{Name: "wide-mode", Shader: &scene.ShaderLayout{}, GLSL: source}}}}
	glBindFramebuffer(glFramebuffer, out.fb)
	if err := c.Render(sc); err != nil {
		t.Fatal(err)
	}
	assertColor(t, readColor(t, 8, 8), [4]float32{2, -.5, .25, 1}, .002)
	// The shader transform must not silently stop matching after a source edit.
	if strings.Contains(sceneShaderSource(), "clamp(fromOklab(") {
		t.Fatal("Oklab still clamps wide colors")
	}
}

func TestWideVideoOnANGLE(t *testing.T) {
	r, resolve := newANGLE(t)
	c := wrapColorRenderer(r)
	t.Cleanup(c.Release)
	out := colorTarget(t)
	vs := r.videoState(true)
	vs.checked = true
	standIn := func(source string) string {
		source = strings.ReplaceAll(source, "#extension GL_OES_EGL_image_external_essl3 : require\n", "")
		return strings.ReplaceAll(source, "samplerExternalOES", "sampler2D")
	}
	program, err := link(r.header+"#define VERTEX\n"+sceneShaderSource(), standIn(videoFragment(r.header)))
	if err != nil {
		t.Fatal(err)
	}
	vs.configureProgram(program)
	program, err = link(r.header+colorVertex, standIn(videoCopyFragment(r.header)))
	if err != nil {
		t.Fatal(err)
	}
	vs.conversion = &videoRenderer{}
	vs.conversion.configureProgram(program)
	bind, get := glBindTexture, glGetIntegerv
	glBindTexture = func(target, texture uint32) {
		if target == glTextureExternal {
			target = glTexture2D
		}
		bind(target, texture)
	}
	glGetIntegerv = func(name uint32, value *int32) {
		if name == 0x8D67 {
			name = 0x8069
		}
		get(name, value)
	}
	t.Cleanup(func() { glBindTexture, glGetIntegerv = bind, get })
	var finish func()
	purego.RegisterFunc(&finish, resolve("glFinish"))
	texture := newFloatTexture(4, 2)
	t.Cleanup(func() { glDeleteTextures(1, &texture) })
	imports, releases := 0, 0
	transfer, gamut, rotation, flip := 0, 0, 0, 0
	width, height := 4, 2
	registration := UseVideoSampler(func(uintptr) *VideoTexture {
		imports++
		return &VideoTexture{Texture: texture, Width: width, Height: height, Transfer: transfer, Gamut: gamut, Rotation: rotation, Flip: flip, CropX: 1, CropY: 1, RangeScale: 1,
			Copy2D:  func(uint32) bool { t.Fatal("wide frame took SDR conversion"); return false },
			Release: func() error { finish(); releases++; return nil }}
	})
	t.Cleanup(registration)
	readAll := func() []float32 {
		pixels := make([]float32, 16*16*4)
		glReadPixels(0, 0, 16, 16, glRGBA, glFloat, unsafe.Pointer(&pixels[0]))
		if e := glGetError(); e != 0 {
			t.Fatalf("read %#x", e)
		}
		return pixels
	}
	convert := func(rgb [3]float64, alpha, white, peak float64) [4]float32 {
		if transfer != 0 {
			for i, x := range rgb {
				a := math.Abs(x)
				switch transfer {
				case 1:
					p := math.Pow(min(max(x, 0), 1), 32.0/2523)
					rgb[i] = 10000 * math.Pow(max(p-3424.0/4096, 0)/(2413.0/128-2392.0/128*p), 16384.0/2610)
				case 2:
					if a <= .5 {
						a = a * a / 3
					} else {
						a = (math.Exp((a-.55991073)/.17883277) + .28466892) / 12
					}
					rgb[i] = math.Copysign(a, x)
				case 3:
					if a <= .04045 {
						a /= 12.92
					} else {
						a = math.Pow((a+.055)/1.055, 2.4)
					}
					rgb[i] = math.Copysign(a, x)
				}
			}
			if transfer == 2 {
				luma := [3]float64{.2126, .7152, .0722}
				if gamut == 1 {
					luma = [3]float64{.2627, .678, .0593}
				} else if gamut == 2 {
					luma = [3]float64{.22897456, .69173852, .07928691}
				}
				y := rgb[0]*luma[0] + rgb[1]*luma[1] + rgb[2]*luma[2]
				gamma := 1.2 + .42*math.Log10(peak/1000)
				if peak > 2000 {
					gamma = 1.2 * math.Pow(1.111, math.Log2(peak/1000))
				}
				scale := peak * math.Pow(max(y, 0), gamma-1)
				for i := range rgb {
					rgb[i] *= scale
				}
			}
			if gamut == 1 {
				rgb = [3]float64{1.660491*rgb[0] - .587641*rgb[1] - .072850*rgb[2], -.124550*rgb[0] + 1.132900*rgb[1] - .008349*rgb[2], -.018151*rgb[0] - .100579*rgb[1] + 1.118730*rgb[2]}
			} else if gamut == 2 {
				rgb = [3]float64{1.22494018*rgb[0] - .22494018*rgb[1], -.04205695*rgb[0] + 1.04205695*rgb[1], -.01963755*rgb[0] - .07863605*rgb[1] + 1.09827360*rgb[2]}
			}
			for i, x := range rgb {
				if transfer == 1 || transfer == 2 {
					x /= white
				}
				a := math.Abs(x)
				if a <= .0031308 {
					a *= 12.92
				} else {
					a = 1.055*math.Pow(a, 1/2.4) - .055
				}
				rgb[i] = math.Copysign(a, x)
			}
		}
		return [4]float32{float32(rgb[0] * alpha), float32(rgb[1] * alpha), float32(rgb[2] * alpha), float32(alpha)}
	}
	shader := &scene.Effect{Name: "wide-video", Shader: &scene.ShaderLayout{}, GLSL: "uniform sampler2D T0; vec4 effect(vec2 p, Effect e) { return texture(T0,(p-e.rect.xy)/e.rect.zw); }"}
	cases := 0
	for _, kind := range []int{0, 1, 2, 3, 4} {
		transfer = kind
		for _, space := range []int{0, 1, 2} {
			gamut = space
			for _, peak := range []float32{400, 1000, 4000} {
				output := ColorOutput{Encoding: ColorExtendedSRGB, ReferenceWhiteNits: 203, HLGPeakNits: peak}
				if err := c.SetOutput(output); err != nil {
					t.Fatal(err)
				}
				// Exact binary fractions avoid losing input accuracy in the test texture.
				sample := [4]float32{1, .5, .25, .5}
				if transfer == 0 || transfer == 3 || transfer == 4 {
					sample = [4]float32{2, -.25, .5, .5}
				}
				pixels := make([]float32, 4*2*4)
				for i := 0; i < len(pixels); i += 4 {
					copy(pixels[i:], sample[:])
				}
				glActiveTexture(glTexture0)
				glBindTexture(glTexture2D, texture)
				glTexSubImage2D(glTexture2D, 0, 0, 0, 4, 2, glRGBA, glFloat, unsafe.Pointer(&pixels[0]))
				img := scene.NewImageRGBA(4, 2, nil)
				img.Surface = &scene.Surface{Format: scene.SurfaceBGRA, Native: 42}
				want := convert([3]float64{float64(sample[0]), float64(sample[1]), float64(sample[2])}, float64(sample[3]), 203, float64(peak))
				for mode := range 3 {
					sc := &scene.Scene{Width: 16, Height: 16, Ops: []scene.Op{{Kind: scene.OpImage, Rect: scene.Rect{W: 16, H: 16}, Image: img, Src: scene.Rect{W: 4, H: 2}}}}
					if mode == 1 {
						sc.Effects = []scene.EffectOp{{Effect: shader, Images: [4]*scene.Image{img}}}
						sc.Ops = []scene.Op{{Kind: scene.OpEffect, Rect: scene.Rect{W: 16, H: 16}}}
					}
					if mode == 2 {
						sc.Ops[0].Blur = .1
					}
					glBindFramebuffer(glFramebuffer, out.fb)
					if err := c.Render(sc); err != nil {
						t.Fatal(kind, space, peak, mode, err)
					}
					assertColor(t, readColor(t, 8, 8), want, .014)
					if imports != releases {
						t.Fatal("owned frame leaked", imports, releases)
					}
					cases++
				}
			}
		}
	}
	// Conversion must retain the same orientation and premultiplied filtering
	// as direct sampling, including transparent high-color edge pixels.
	transfer, gamut = 1, 1
	pixels := []float32{1, 0, 0, 1, 0, 1, 0, .5, 0, 0, 1, .25, 1, 1, 0, 0, 0, 1, 1, .75, 1, 0, 1, .5, .5, .5, .5, 1, 1, 1, 1, .125}
	glActiveTexture(glTexture0)
	glBindTexture(glTexture2D, texture)
	glTexSubImage2D(glTexture2D, 0, 0, 0, 4, 2, glRGBA, glFloat, unsafe.Pointer(&pixels[0]))
	for direction := range 12 {
		rotation = []int{0, 1, 2, 3, 0, 0, 1, 1, 2, 2, 3, 3}[direction]
		flip = []int{0, 0, 0, 0, 1, 2, 1, 2, 1, 2, 1, 2}[direction]
		width, height = 4, 2
		if rotation%2 != 0 {
			width, height = 2, 4
		}
		img := scene.NewImageRGBA(width, height, nil)
		img.Surface = &scene.Surface{Format: scene.SurfaceBGRA, Native: 42}
		sc := &scene.Scene{Width: 16, Height: 16, Ops: []scene.Op{{Kind: scene.OpImage, Rect: scene.Rect{W: 16, H: 16}, Image: img, Src: scene.Rect{W: float32(width), H: float32(height)}}}}
		glBindFramebuffer(glFramebuffer, out.fb)
		if err := c.Render(sc); err != nil {
			t.Fatal(err)
		}
		direct := readAll()
		sc.Effects = []scene.EffectOp{{Effect: shader, Images: [4]*scene.Image{img}}}
		sc.Ops = []scene.Op{{Kind: scene.OpEffect, Rect: scene.Rect{W: 16, H: 16}}}
		glBindFramebuffer(glFramebuffer, out.fb)
		if err := c.Render(sc); err != nil {
			t.Fatal(err)
		}
		converted := readAll()
		for i := range direct {
			if math.Abs(float64(direct[i]-converted[i])) > .02 {
				t.Fatalf("orientation %d, component %d: %g vs %g", direction, i, direct[i], converted[i])
			}
		}
	}
	if imports != releases {
		t.Fatal(imports, releases)
	}
	t.Logf("%d float video/mode cases; 12 orientations; %d owned imports/releases; no RGBA8 conversion", cases, imports)
}

func TestWideVideoFallbackOnANGLE(t *testing.T) {
	r, _ := newANGLE(t)
	c := wrapColorRenderer(r)
	t.Cleanup(c.Release)
	out := colorTarget(t)
	r.Partial = true
	if err := c.SetOutput(ColorOutput{Encoding: ColorExtendedSRGB}); err != nil {
		t.Fatal(err)
	}
	img := scene.NewImageRGBA(1, 1, []byte{255, 0, 0, 255})
	img.Surface = &scene.Surface{Format: scene.SurfaceBGRA, Native: 42}
	sc := &scene.Scene{Width: 16, Height: 16, Ops: []scene.Op{{Kind: scene.OpImage, Rect: scene.Rect{W: 16, H: 16}, Image: img, Src: scene.Rect{W: 1, H: 1}}}}
	render := func() error { glBindFramebuffer(glFramebuffer, out.fb); return c.Render(sc) }
	// Even though SDR pixels exist, they must not silently replace a wide frame.
	if err := render(); err == nil {
		t.Fatal("missing extended uploader used SDR restoration")
	}
	count := 0
	failure := errors.New("frame unavailable")
	failing := true
	c.SetExtendedVideoUpload(func(native uintptr, texture uint32, white, peak float32) error {
		count++
		if native != 42 || white != 203 || peak != 1000 {
			t.Fatal(native, white, peak)
		}
		if failing {
			return failure
		}
		var previous int32
		glGetIntegerv(0x8069, &previous)
		defer glBindTexture(glTexture2D, uint32(previous))
		glBindTexture(glTexture2D, texture)
		pixel := []float32{2, -.5, .25, .5}
		glTexSubImage2D(glTexture2D, 0, 0, 0, 1, 1, glRGBA, glFloat, unsafe.Pointer(&pixel[0]))
		return nil
	})
	for range 2 {
		if err := render(); !errors.Is(err, failure) {
			t.Fatalf("lost conversion failure: %v", err)
		}
	}
	if count != 2 {
		t.Fatal("failed upload reused as a retained scene", count)
	}
	failing = false
	if err := render(); err != nil {
		t.Fatal(err)
	}
	assertColor(t, readColor(t, 8, 8), [4]float32{2, -.5, .25, .5}, .002)
	if err := render(); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatal("unchanged upload not cached", count)
	}
	img.Changed()
	if err := render(); err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatal("new video version ignored", count)
	}
	canvas := scene.NewCanvas(16, 16)
	canvas.Scene = sc
	sc = &scene.Scene{Width: 16, Height: 16, Ops: []scene.Op{{Kind: scene.OpImage, Rect: scene.Rect{W: 16, H: 16}, Image: canvas, Src: scene.Rect{W: 16, H: 16}}}}
	if err := render(); err != nil {
		t.Fatal(err)
	}
	c.SetExtendedVideoUpload(nil)
	for range 2 {
		if err := render(); err == nil {
			t.Fatal("removed uploader reused an old or failed canvas")
		}
	}
	if err := c.SetOutput(ColorOutput{}); err != nil {
		t.Fatal(err)
	}
	if err := render(); err != nil {
		t.Fatal(err)
	}
	assertColor(t, readColor(t, 8, 8), [4]float32{1, 0, 0, 1}, .003)
}

func TestWideOklabOnANGLE(t *testing.T) {
	r, _ := newANGLE(t)
	c := wrapColorRenderer(r)
	t.Cleanup(c.Release)
	out := colorTarget(t)
	if err := c.SetOutput(ColorOutput{Encoding: ColorExtendedSRGB}); err != nil {
		t.Fatal(err)
	}
	a, b := [4]float32{1.5, -.2, .1, .5}, [4]float32{1.2, -.1, .5, .75}
	lab := func(v [4]float32) [3]float64 {
		l, a, b := gamut.Oklab(gamut.Decode(float64(v[0])), gamut.Decode(float64(v[1])), gamut.Decode(float64(v[2])))
		return [3]float64{l, a, b}
	}
	la, lb := lab(a), lab(b)
	sc := colorScene(a)
	sc.Wide[0].Set |= scene.WideColor2
	sc.Wide[0].Color2 = b
	sc.Ops[0].Paint = scene.PaintOklab
	sc.Ops[0].Gradient = [4]float32{0, 0, 16, 0}
	glBindFramebuffer(glFramebuffer, out.fb)
	if err := c.Render(sc); err != nil {
		t.Fatal(err)
	}
	for x := range 16 {
		f := (float64(x) + .5) / 16
		alpha := float64(a[3])*(1-f) + float64(b[3])*f
		var blend [3]float64
		for i := range blend {
			blend[i] = (la[i]*float64(a[3])*(1-f) + lb[i]*float64(b[3])*f) / alpha
		}
		cr, cg, cb := gamut.LinearSRGB(blend[0], blend[1], blend[2])
		want := [4]float32{float32(gamut.Encode(cr) * alpha), float32(gamut.Encode(cg) * alpha), float32(gamut.Encode(cb) * alpha), float32(alpha)}
		assertColor(t, readColor(t, x, 8), want, .004)
	}
}

func TestColorBufferFormatsAndResizeOnANGLE(t *testing.T) {
	r, _ := newANGLE(t)
	c := wrapColorRenderer(r)
	t.Cleanup(c.Release)
	out := colorTarget(t)
	read := colorTarget(t)
	r.Partial = true
	// A complete 10-bit normalized output, as an HDR window would supply.
	var ten uint32
	glGenTextures(1, &ten)
	defer glDeleteTextures(1, &ten)
	glBindTexture(glTexture2D, ten)
	glTexImage2D(glTexture2D, 0, 0x8059, 16, 16, 0, glRGBA, 0x8368, nil)
	glBindFramebuffer(glFramebuffer, out.fb)
	glFramebufferTexture2D(glFramebuffer, glColorAttachment0, glTexture2D, ten, 0)
	if glCheckFramebufferState(glFramebuffer) != glFramebufferDone {
		t.Fatal("ten-bit framebuffer incomplete")
	}
	if err := c.SetOutput(ColorOutput{Encoding: ColorBT2020PQ}); err != nil {
		t.Fatal(err)
	}
	if err := c.Render(colorScene([4]float32{1, 1, 1, 1})); err != nil {
		t.Fatal(err)
	}
	var packed uint32
	glReadPixels(8, 8, 1, 1, glRGBA, 0x8368, unsafe.Pointer(&packed))
	if e := glGetError(); e != 0 {
		t.Fatalf("ten-bit read %#x", e)
	}
	p := math.Pow(203.0/10000, 2610.0/16384)
	expected := math.Pow((3424.0/4096+2413.0/128*p)/(1+2392.0/128*p), 2523.0/32)
	for i := range 3 {
		got := float64((packed>>uint(i*10))&1023) / 1023
		if math.Abs(got-expected) > .0015 {
			t.Fatal("ten-bit PQ", got, expected)
		}
	}
	if packed>>30 != 3 {
		t.Fatal("ten-bit alpha", packed)
	}
	// The target must be restored even when its read and draw bindings differ.
	glFramebufferTexture2D(glFramebuffer, glColorAttachment0, glTexture2D, out.tex, 0)
	if err := c.SetOutput(ColorOutput{Encoding: ColorExtendedSRGB}); err != nil {
		t.Fatal(err)
	}
	glBindFramebuffer(glReadFramebuffer, read.fb)
	if err := c.Render(colorScene([4]float32{2, -.5, .25, 1})); err != nil {
		t.Fatal(err)
	}
	var drawBinding, readBinding int32
	glGetIntegerv(glFramebufferBinding, &drawBinding)
	glGetIntegerv(0x8CAA, &readBinding)
	if uint32(drawBinding) != out.fb || uint32(readBinding) != read.fb {
		t.Fatal("framebuffer bindings lost", drawBinding, readBinding)
	}
	// Resize a retained frame: no old pixels may survive at the new edges.
	if err := fitFloatTarget(out, 24, 24); err != nil {
		t.Fatal(err)
	}
	sc := colorScene([4]float32{3, -.25, .75, 1})
	sc.Width, sc.Height = 24, 24
	sc.Ops[0].Rect = scene.Rect{W: 24, H: 24}
	if err := c.Render(sc); err != nil {
		t.Fatal(err)
	}
	assertColor(t, readColor(t, 23, 23), [4]float32{3, -.25, .75, 1}, .003)
	if r.redrew != redrewWhole {
		t.Fatal("resize retained stale contents")
	}
	// A host without a current context gets a clean error and can retry later.
	contextMu.Lock()
	saved := contextID
	contextID = func() uintptr { return 0 }
	contextMu.Unlock()
	err := c.Render(sc)
	contextMu.Lock()
	contextID = saved
	contextMu.Unlock()
	if !errors.Is(err, errNoContext) {
		t.Fatal("missing context", err)
	}
	if err := c.Render(sc); err != nil {
		t.Fatal("context restored", err)
	}
}

func TestColorCaptureKeepsLiveOutputOnANGLE(t *testing.T) {
	r, _ := newANGLE(t)
	c := wrapColorRenderer(r)
	t.Cleanup(c.Release)
	out, read := colorTarget(t), colorTarget(t)
	r.Partial = true
	s := colorScene([4]float32{2, -.25, .5, .5})
	output := ColorOutput{Encoding: ColorExtendedSRGB, ReferenceWhiteNits: 203, HLGPeakNits: 1000}
	if err := c.SetOutput(output); err != nil {
		t.Fatal(err)
	}
	glBindFramebuffer(glFramebuffer, out.fb)
	if err := c.Render(s); err != nil {
		t.Fatal(err)
	}
	glBindFramebuffer(glReadFramebuffer, read.fb)
	for range 2 {
		w, h, rgba, err := c.Capture(s)
		if err != nil || w != 16 || h != 16 || len(rgba) != 16*16*4 {
			t.Fatal(w, h, len(rgba), err)
		}
		// SDR uses the scene's mapped fallback color, and the capture contract is
		// RGBA rather than ReadFramebuffer's BGRA. It must draw on repeated calls.
		for i := 0; i < len(rgba); i += 4 {
			if rgba[i] != 128 || rgba[i+1] != 64 || rgba[i+2] != 32 || rgba[i+3] != 255 {
				t.Fatal("SDR capture color", rgba[i:i+4])
			}
		}
		var drawBinding, readBinding int32
		glGetIntegerv(glFramebufferBinding, &drawBinding)
		glGetIntegerv(0x8CAA, &readBinding)
		if uint32(drawBinding) != out.fb || uint32(readBinding) != read.fb || c.output != output {
			t.Fatal("capture changed host output", drawBinding, readBinding, c.output)
		}
	}
	glBindFramebuffer(glReadFramebuffer, out.fb)
	assertColor(t, readColor(t, 8, 8), [4]float32{1, -.125, .25, .5}, .002)
	if err := c.Render(s); err != nil {
		t.Fatal("resume HDR", err)
	}
	assertColor(t, readColor(t, 8, 8), [4]float32{1, -.125, .25, .5}, .002)
	// A same-mode capture must not leave partial rendering believing the live
	// target was drawn, even if no output transition invalidates its contents.
	if err := c.SetOutput(ColorOutput{}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := c.Capture(s); err != nil {
		t.Fatal(err)
	}
	if err := c.Render(s); err != nil {
		t.Fatal(err)
	}
	assertColor(t, readColor(t, 8, 8), [4]float32{128.0 / 255, 64.0 / 255, 32.0 / 255, 1}, .002)
	if _, _, _, err := c.Capture(&scene.Scene{Width: r.maxSize + 1, Height: 1}); err == nil {
		t.Fatal("oversize capture")
	}
	c.Release()
	if _, _, _, err := c.Capture(s); err == nil {
		t.Fatal("capture after release")
	}
}
