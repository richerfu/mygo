package text

import "testing"

type providerFontTest struct{}

func (*providerFontTest) Metrics() NativeFontMetrics {
	return NativeFontMetrics{Size: 20, Ascent: 16, Descent: 4}
}
func (*providerFontTest) Glyph(uint32, float32, float32, Shade, bool) NativeBitmap {
	return NativeBitmap{Pixels: []byte{255, 128, 128, 255}, Width: 2, Height: 2, Left: -1, Top: -2}
}

type providerTest struct {
	face   *providerFontTest
	shapes int
}

func (p *providerTest) Font(Style) NativeFont { return p.face }
func (p *providerTest) Shape(_ []rune, _ Style, _ []Span, _ float32, _, _ bool) []NativeLine {
	p.shapes++
	return []NativeLine{{End: 3, Runs: []NativeRun{{Font: p.face, End: 3, Glyphs: []NativeGlyph{{ID: 7, Cluster: 0, Advance: 11}, {ID: 8, Cluster: 1, X: 11, Advance: 17}}}}}}
}
func (*providerTest) Register([]byte, string) error { return nil }
func (*providerTest) Families() []FontFamily        { return nil }
func TestNativeProviderPreservesClustersAndGlyphOrigin(t *testing.T) {
	p := &providerTest{face: new(providerFontTest)}
	s := newSystem()
	s.eng = &providerEngine{provider: p, fonts: map[NativeFont]*Font{}}
	l := s.Layout(Params{Text: "Afi", Style: Style{Size: 20}, NoWrap: true})
	if len(l.Lines) != 1 || len(l.Lines[0].Glyphs) != 2 {
		t.Fatalf("lost native glyphs: %+v", l.Lines)
	}
	gs := l.Lines[0].Glyphs
	if gs[0].Cluster != 0 || gs[1].Cluster != 1 || gs[1].Runes != 2 || gs[1].X != 11 {
		t.Fatalf("lost ligature mapping/position: %+v", gs)
	}
	_ = s.Layout(Params{Text: "Afi", Style: Style{Size: 20}, NoWrap: true})
	if p.shapes != 1 {
		t.Fatal("native layout bypassed framework cache", p.shapes)
	}
	image := s.Glyph(gs[0].Font, 7, 1, 0, 0, false)
	if !image.OK {
		t.Fatal("native glyph never entered the atlas")
	}
}
