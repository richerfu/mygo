package text

import (
	"errors"
	"github.com/egoist/mygo/internal/scene"
	"math"
)

// NativeFont is a native font held for the lifetime of every layout referring
// to it. Implementations are comparable and retain their native resources until
// unreachable; the provider may be called from any goroutine, under System's
// lock. Glyph images are owned Go buffers, with coverage or premultiplied RGBA.
type NativeFont interface {
	Metrics() NativeFontMetrics
	Glyph(id uint32, scale, dx float32, shade Shade, subpixel bool) NativeBitmap
}
type NativeFontMetrics struct{ Size, Ascent, Descent, LineGap, UnderlineTop, UnderlineThickness, StrikeTop, StrikeThickness float32 }
type NativeBitmap struct {
	Pixels                   []byte
	Width, Height, Left, Top int
	Color, Subpixel          bool
}
type NativeGlyph struct {
	ID            uint32
	Cluster       int
	X, Y, Advance float32
	RTL, Object   bool
}
type NativeRun struct {
	Font       NativeFont
	Start, End int
	Glyphs     []NativeGlyph
}
type NativeLine struct {
	Start, End int
	Runs       []NativeRun
}

// NativeTextProvider supplies the system font stack. Shape ranges are rune
// indices, glyphs lie relative to the line origin, Y down from its baseline.
// All shape arrays are borrowed only until the next provider call.
type NativeTextProvider interface {
	Font(Style) NativeFont
	Shape(text []rune, style Style, spans []Span, width float32, rtl, wholeWords bool) []NativeLine
	Register(data []byte, family string) error
	Families() []FontFamily
}

// NativeTextRendering optionally preserves the host's rasterization rules.
// Providers without it use grayscale masks at quarter-pixel horizontal origins.
type NativeTextRendering interface {
	GlyphPositions(NativeFont, float32, bool) (int, bool)
	Baseline(float32) float32
	JoinGlyphs() bool
	TextParams() (scene.TextParams, bool)
}
type NativeRegisteredFonts interface{ RegisteredFamilies() []FontFamily }

// UseNativeProvider installs the host text stack before the process-wide text
// system is first used. It never replaces live fonts or existing layouts.
func UseNativeProvider(p NativeTextProvider) error {
	if p == nil {
		return errors.New("mygo: nil native font provider")
	}
	s := Shared()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.eng != nil {
		return errors.New("mygo: text engine already initialized")
	}
	s.eng = &providerEngine{provider: p, fonts: map[NativeFont]*Font{}}
	return nil
}

type providerEngine struct {
	shapeScratch
	provider NativeTextProvider
	fonts    map[NativeFont]*Font
}

func (e *providerEngine) intern(face NativeFont) *Font {
	if face == nil {
		return nil
	}
	if f := e.fonts[face]; f != nil {
		return f
	}
	m := face.Metrics()
	f := &Font{Size: m.Size, Ascent: m.Ascent, Descent: m.Descent, LineGap: m.LineGap, data: face, underlineTop: m.UnderlineTop, underlineThick: m.UnderlineThickness, strikeTop: m.StrikeTop, strikeThick: m.StrikeThickness}
	e.fonts[face] = f
	return f
}
func (e *providerEngine) font(s Style) *Font { return e.intern(e.provider.Font(s)) }
func (e *providerEngine) shape(t []rune, s Style, spans []Span, w float32, rtl, words bool) []shapedLine {
	lines := e.provider.Shape(t, s, spans, w, rtl, words)
	out := make([]shapedLine, len(lines))
	for i, l := range lines {
		out[i] = shapedLine{start: l.Start, end: l.End, runs: make([]shapedRun, len(l.Runs))}
		for j, r := range l.Runs {
			f := e.intern(r.Font)
			run := &out[i].runs[j]
			*run = shapedRun{font: f, start: r.Start, end: r.End, glyphs: make([]Glyph, len(r.Glyphs))}
			for k, g := range r.Glyphs {
				run.glyphs[k] = Glyph{Font: f, ID: g.ID, Size: f.Size, X: g.X, Y: g.Y, Advance: g.Advance, Cluster: g.Cluster, RTL: g.RTL, Object: g.Object}
			}
		}
	}
	return out
}
func (e *providerEngine) glyph(f *Font, id uint32, scale, dx float32, shade Shade, subpixel bool) bitmap {
	b := f.data.(NativeFont).Glyph(id, scale, dx, shade, subpixel)
	channels := 1
	if b.Color || b.Subpixel {
		channels = 4
	}
	if b.Width < 0 || b.Height < 0 || b.Width > 16384 || b.Height > 16384 || len(b.Pixels) < b.Width*b.Height*channels {
		return bitmap{}
	}
	return bitmap{left: b.Left, top: b.Top, w: b.Width, h: b.Height, pix: b.Pixels, color: b.Color, subpixel: b.Subpixel}
}
func (e *providerEngine) positions(f *Font, scale float32, subpixel bool) (int, bool) {
	if p, ok := e.provider.(NativeTextRendering); ok {
		return p.GlyphPositions(f.data.(NativeFont), scale, subpixel)
	}
	return 4, false
}
func (e *providerEngine) baseline(y float32) float32 {
	if p, ok := e.provider.(NativeTextRendering); ok {
		return p.Baseline(y)
	}
	return float32(math.Round(float64(y)))
}
func (e *providerEngine) join() bool {
	p, ok := e.provider.(NativeTextRendering)
	return ok && p.JoinGlyphs()
}
func (e *providerEngine) textParams() (scene.TextParams, bool) {
	if p, ok := e.provider.(NativeTextRendering); ok {
		return p.TextParams()
	}
	return scene.TextParams{}, false
}
func (e *providerEngine) register(b []byte, f string) error { return e.provider.Register(b, f) }
func (e *providerEngine) appFamilies() []FontFamily {
	if p, ok := e.provider.(NativeRegisteredFonts); ok {
		return p.RegisteredFamilies()
	}
	return nil
}
func (e *providerEngine) systemFamilies() func() []FontFamily { return e.provider.Families }
func (e *providerEngine) fontCount() int                      { return len(e.fonts) }
func (e *providerEngine) forgetFonts()                        { e.fonts = map[NativeFont]*Font{} }
func (e *providerEngine) releaseFonts()                       {}
func (e *providerEngine) decorate(r decoRange) []Stroke {
	var out []Stroke
	base := e.baseline(r.baseline() * r.scale)
	r.runs(func(f *Font, i, j int) {
		x0, x1 := r.span(i, j)
		top, thick := f.strikeTop, f.strikeThick
		if r.d == Underline {
			top, thick = f.underlineTop, f.underlineThick
		}
		if thick <= 0 {
			thick = max(f.Size/16, 1/r.scale)
		}
		out = append(out, Stroke{X0: (r.x + x0) * r.scale, X1: (r.x + x1) * r.scale, Top: base - top*r.scale, Bottom: base - top*r.scale + thick*r.scale})
	})
	return out
}
