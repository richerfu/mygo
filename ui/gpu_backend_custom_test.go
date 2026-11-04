//go:build mygo_custom_backend

package ui

import (
	"github.com/egoist/mygo/internal/text"
	"testing"
)

type rendererTestFonts struct{}

func (rendererTestFonts) Font(text.Style) text.NativeFont { return nil }
func (rendererTestFonts) Shape([]rune, text.Style, []text.Span, float32, bool, bool) []text.NativeLine {
	return nil
}
func (rendererTestFonts) Register([]byte, string) error { return nil }
func (rendererTestFonts) Families() []text.FontFamily   { return nil }
func prepareExternalRendererFonts(t *testing.T) {
	t.Helper()
	if err := text.UseNativeProvider(rendererTestFonts{}); err != nil {
		t.Fatal(err)
	}
}
