//go:build mygo_custom_backend && darwin

package gl

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo/internal/scene"
)

// newANGLE makes a real GLES 3 context for scene and presentation tests. Tests
// which replace an external sampler with sampler2D validate shader math and GL
// resource/state handling, not an OHOS allocator or EGLImage import.
func newANGLE(t *testing.T) (*Renderer, func(string) uintptr) {
	t.Helper()
	libraries := os.Getenv("MYGO_GLES_TEST_LIBRARIES")
	if libraries == "" {
		t.Skip("set MYGO_GLES_TEST_LIBRARIES to host ANGLE dylibs")
	}
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	egl, err := purego.Dlopen(filepath.Join(libraries, "libEGL.dylib"), purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Fatal(err)
	}
	gles, err := purego.Dlopen(filepath.Join(libraries, "libGLESv2.dylib"), purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Fatal(err)
	}
	var getProc func(string) uintptr
	var platform func(uint32, uintptr, *int32) uintptr
	var initialize func(uintptr, *int32, *int32) uint32
	var bindAPI func(uint32) uint32
	var choose func(uintptr, *int32, *uintptr, int32, *int32) uint32
	var createSurface func(uintptr, uintptr, *int32) uintptr
	var createContext func(uintptr, uintptr, uintptr, *int32) uintptr
	var makeCurrent func(uintptr, uintptr, uintptr, uintptr) uint32
	var destroySurface func(uintptr, uintptr) uint32
	var destroyContext func(uintptr, uintptr) uint32
	var terminate func(uintptr) uint32
	var current func() uintptr
	for name, dest := range map[string]any{"eglGetProcAddress": &getProc, "eglInitialize": &initialize, "eglBindAPI": &bindAPI, "eglChooseConfig": &choose, "eglCreatePbufferSurface": &createSurface, "eglCreateContext": &createContext, "eglMakeCurrent": &makeCurrent, "eglDestroySurface": &destroySurface, "eglDestroyContext": &destroyContext, "eglTerminate": &terminate, "eglGetCurrentContext": &current} {
		p, e := purego.Dlsym(egl, name)
		if e != nil {
			t.Fatal(e)
		}
		purego.RegisterFunc(dest, p)
	}
	purego.RegisterFunc(&platform, getProc("eglGetPlatformDisplayEXT"))
	display := platform(0x3202, 0, &[]int32{0x3203, 0x3489, 0x3038}[0])
	if display == 0 || initialize(display, nil, nil) == 0 {
		t.Fatal("initialize ANGLE")
	}
	t.Cleanup(func() { terminate(display) })
	if bindAPI(0x30A0) == 0 {
		t.Fatal("GLES API")
	}
	var config uintptr
	var count int32
	if choose(display, &[]int32{0x3033, 1, 0x3040, 0x40, 0x3024, 8, 0x3023, 8, 0x3022, 8, 0x3021, 8, 0x3038}[0], &config, 1, &count) == 0 || count != 1 {
		t.Fatal("config")
	}
	surface := createSurface(display, config, &[]int32{0x3057, 16, 0x3056, 16, 0x3038}[0])
	context := createContext(display, config, 0, &[]int32{0x3098, 3, 0x3038}[0])
	if surface == 0 || context == 0 {
		t.Fatal("context")
	}
	t.Cleanup(func() {
		makeCurrent(display, 0, 0, 0)
		destroyContext(display, context)
		destroySurface(display, surface)
	})
	if makeCurrent(display, surface, surface, context) == 0 {
		t.Fatal("make current")
	}
	resolve := func(name string) uintptr {
		p := getProc(name)
		if p == 0 {
			p, _ = purego.Dlsym(gles, name)
		}
		return p
	}
	if err := UseContext(resolve, current); err != nil {
		t.Fatal(err)
	}
	r, err := New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Release)
	return r, resolve
}

func TestVideoSceneOnANGLE(t *testing.T) {
	r, resolve := newANGLE(t)
	vs := r.videoState(true)
	vs.checked = true
	fragment := strings.ReplaceAll(videoFragment(r.header), "#extension GL_OES_EGL_image_external_essl3 : require\n", "")
	fragment = strings.ReplaceAll(fragment, "samplerExternalOES", "sampler2D")
	program, err := link(r.header+"#define VERTEX\n"+shaderSource, fragment)
	if err != nil {
		t.Fatal(err)
	}
	vs.configureProgram(program)
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
	// A 4x2 visible image inside a 6x4 allocation, with poison-colored padding.
	source := make([]byte, 6*4*4)
	for i := 0; i < len(source); i += 4 {
		copy(source[i:], []byte{255, 0, 255, 255})
	}
	colors := [][4]byte{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}, {255, 255, 0, 255}, {0, 255, 255, 255}, {255, 0, 255, 255}, {128, 128, 128, 255}, {255, 255, 255, 128}}
	for y := range 2 {
		for x := range 4 {
			copy(source[(y*6+x)*4:], colors[y*4+x][:])
		}
	}
	texture := newTexture(glRGBA8, glRGBA, 6, 4, source, 24)
	t.Cleanup(func() { glDeleteTextures(1, &texture) })
	imports, releases, copies := 0, 0, 0
	rotation, flip, width, height := 0, 0, 4, 2
	transfer, gamut := 0, 0
	rangeOffset, rangeScale := float32(0), float32(1)
	var expected []byte
	registration := UseVideoSampler(func(uintptr) *VideoTexture {
		imports++
		return &VideoTexture{Texture: texture, Width: width, Height: height, Rotation: rotation, Flip: flip, Transfer: transfer, Gamut: gamut, CropX: 4.0 / 6, CropY: .5, RangeOffset: rangeOffset, RangeScale: rangeScale,
			Copy2D: func(target uint32) bool {
				copies++
				glBindTexture(glTexture2D, target)
				unpack(width*4, glRGBA8)
				glTexSubImage2D(glTexture2D, 0, 0, 0, int32(width), int32(height), glRGBA, glUnsignedByte, unsafe.Pointer(&expected[0]))
				unpackReset()
				return true
			},
			Release: func() error { finish(); releases++; return nil }}
	})
	t.Cleanup(registration)
	for direction := range 12 {
		rotation = []int{0, 1, 2, 3, 0, 0, 1, 1, 2, 2, 3, 3}[direction]
		flip = []int{0, 0, 0, 0, 1, 2, 1, 2, 1, 2, 1, 2}[direction]
		width, height = 4, 2
		if rotation%2 != 0 {
			width, height = 2, 4
		}
		expected = make([]byte, width*height*4)
		for y := range height {
			for x := range width {
				sx, sy := x, y
				switch rotation {
				case 1:
					sx, sy = y, 1-x
				case 2:
					sx, sy = 3-x, 1-y
				case 3:
					sx, sy = 3-y, x
				}
				if flip == 1 {
					sx = 3 - sx
				}
				if flip == 2 {
					sy = 1 - sy
				}
				copy(expected[(y*width+x)*4:], colors[sy*4+sx][:])
				for c := range 3 {
					at := (y*width+x)*4 + c
					expected[at] = byte((int(expected[at])*int(expected[(y*width+x)*4+3]) + 127) / 255)
				}
			}
		}
		for mode := range 4 {
			r.Partial = mode == 0
			blur := float32(0)
			if mode == 1 {
				blur = 1
			}
			img := scene.NewImageRGBA(width, height, nil)
			img.Surface = &scene.Surface{Format: scene.SurfaceBGRA, Native: 42}
			sc := &scene.Scene{Width: 16, Height: 16, Ops: []scene.Op{{Kind: scene.OpPushClip, Rect: scene.Rect{X: 2, Y: 2, W: 10, H: 10}, Radii: scene.Uniform(2)},
				{Kind: scene.OpImage, Rect: scene.Rect{X: 1, Y: 1, W: 12, H: 12}, Image: img, Src: scene.Rect{X: .25, Y: .25, W: float32(width) - .5, H: float32(height) - .5}, Color: scene.Color{R: 255, G: 255, B: 255, A: 255}, Opacity: .7, Blur: blur, Pixelated: direction%2 != 0},
				{Kind: scene.OpPopClip}}}
			var canvas *scene.Image
			if mode == 2 {
				// Alternate direct video, an ordinary image and a shader which
				// needs the same video as sampler2D, then draw the video again.
				ordinary := scene.NewImageRGBA(1, 1, []byte{40, 80, 120, 255})
				sc.Effects = []scene.EffectOp{{Effect: &scene.Effect{Name: "video-input", Shader: &scene.ShaderLayout{}, GLSL: "uniform sampler2D T0; vec4 effect(vec2 p, Effect e) { return texture(T0, (p-e.rect.xy)/e.rect.zw); }"}, Images: [4]*scene.Image{img}}}
				sc.Ops = []scene.Op{
					{Kind: scene.OpImage, Rect: scene.Rect{W: 8, H: 8}, Image: img, Src: scene.Rect{W: float32(width), H: float32(height)}},
					{Kind: scene.OpImage, Rect: scene.Rect{X: 8, W: 8, H: 8}, Image: ordinary, Src: scene.Rect{W: 1, H: 1}},
					{Kind: scene.OpEffect, Rect: scene.Rect{Y: 8, W: 8, H: 8}},
					{Kind: scene.OpImage, Rect: scene.Rect{X: 8, Y: 8, W: 8, H: 8}, Image: img, Src: scene.Rect{W: float32(width), H: float32(height)}},
				}
			}
			if mode == 3 {
				canvas = scene.NewCanvas(16, 16)
				canvas.Scene = sc
				sc = &scene.Scene{Width: 16, Height: 16, Ops: []scene.Op{{Kind: scene.OpImage, Rect: scene.Rect{W: 16, H: 16}, Image: canvas, Src: scene.Rect{W: 16, H: 16}}}}
			}
			copyBefore := copies
			if err := r.Render(sc); err != nil {
				t.Fatal(direction, blur, err)
			}
			if releases != imports {
				t.Fatal("lease survived Render", imports, releases)
			}
			if (mode == 0 || mode == 3) && (copies != copyBefore || r.images[img.ID()] != nil) {
				t.Fatal("direct draw allocated/converted an image")
			}
			got, err := ReadFramebuffer(16, 16)
			if err != nil {
				t.Fatal(err)
			}
			if r.Partial {
				before := imports
				if err := r.Render(sc); err != nil || imports != before {
					t.Fatal("unchanged retained frame imported again", err, before, imports)
				}
			}
			img.Surface = nil
			img.Pix = append([]byte(nil), expected...)
			img.Changed()
			if canvas != nil {
				canvas.Changed()
			}
			if err := r.Render(sc); err != nil {
				t.Fatal(err)
			}
			want, err := ReadFramebuffer(16, 16)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(want, make([]byte, len(want))) {
				t.Fatal("reference renderer produced an empty frame")
			}
			if !bytes.Equal(got, want) {
				maxDiff := 0
				for i := range got {
					d := int(got[i]) - int(want[i])
					if d < 0 {
						d = -d
					}
					maxDiff = max(maxDiff, d)
				}
				if maxDiff > 1 {
					t.Fatalf("direction=%d mode=%d maximum channel difference %d imports=%d copies=%d center=%v/%v", direction, mode, maxDiff, imports, copies, got[(8*16+8)*4:(8*16+8)*4+4], want[(8*16+8)*4:(8*16+8)*4+4])
				}
			}
		}
	}
	if imports != 48 || releases != 48 || copies != 24 {
		t.Fatal("unexpected frame/conversion counts", imports, releases, copies)
	}
	// Render's error return must still retire the owned frame.
	r.Partial = false
	img := scene.NewImageRGBA(width, height, nil)
	img.Surface = &scene.Surface{Format: scene.SurfaceBGRA, Native: 42}
	sc := &scene.Scene{Width: 16, Height: 16, Ops: []scene.Op{{Kind: scene.OpImage, Rect: scene.Rect{W: 16, H: 16}, Image: img, Src: scene.Rect{W: float32(width), H: float32(height)}}}}
	getError := glGetError
	glGetError = func() uint32 { return 0x0502 }
	err = r.Render(sc)
	glGetError = getError
	if err == nil || releases != 49 || imports != releases {
		t.Fatal("draw error leaked video", err, imports, releases)
	}
	rotation, flip, width, height = 0, 0, 4, 2
	for _, c := range []struct {
		transfer, gamut int
		offset, scale   float32
		code            byte
		lo, hi          byte
	}{
		{0, 0, 16.0 / 255, 255.0 / 219, 125, 62, 65},
		{1, 1, 0, 1, 128, 77, 84},
		{2, 1, 0, 1, 191, 92, 99},
	} {
		transfer, gamut, rangeOffset, rangeScale = c.transfer, c.gamut, c.offset, c.scale
		for y := range 2 {
			for x := range 4 {
				copy(source[(y*6+x)*4:], []byte{c.code, c.code, c.code, 128})
			}
		}
		glBindTexture(glTexture2D, texture)
		unpack(24, glRGBA8)
		glTexSubImage2D(glTexture2D, 0, 0, 0, 6, 4, glRGBA, glUnsignedByte, unsafe.Pointer(&source[0]))
		unpackReset()
		img = scene.NewImageRGBA(width, height, nil)
		img.Surface = &scene.Surface{Format: scene.SurfaceBGRA, Native: 42}
		sc = &scene.Scene{Width: 4, Height: 2, Ops: []scene.Op{{Kind: scene.OpImage, Rect: scene.Rect{W: 4, H: 2}, Image: img, Src: scene.Rect{W: 4, H: 2}}}}
		if err := r.Render(sc); err != nil {
			t.Fatal(err)
		}
		pixels, err := ReadFramebuffer(4, 2)
		if err != nil {
			t.Fatal(err)
		}
		for i, p := range pixels {
			if i%4 == 3 {
				if p != 128 {
					t.Fatal("alpha", p)
				}
				continue
			}
			if p < c.lo || p > c.hi {
				t.Fatalf("transfer %d gray/alpha result %d, want [%d,%d]", c.transfer, p, c.lo, c.hi)
			}
		}
	}
	if imports != 52 || releases != imports || copies != 24 {
		t.Fatal(imports, releases, copies)
	}
	t.Logf("%d imports/releases; %d on-demand 2D conversions; 12 orientations with alpha, rounded clipping, blur, mixed shader inputs and canvas; 2D sampler stand-in, no OHOS allocator validation", imports, copies)
}
