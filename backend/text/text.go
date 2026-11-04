// Package text supplies the optional native font-provider extension.
package text

import (
	"github.com/egoist/mygo/internal/scene"
	"github.com/egoist/mygo/internal/text"
)

type TextStyle = text.Style
type TextSpan = text.Span
type FontFamily = text.FontFamily
type FontShade = text.Shade
type NativeFont = text.NativeFont
type NativeFontMetrics = text.NativeFontMetrics
type NativeBitmap = text.NativeBitmap
type NativeGlyph = text.NativeGlyph
type NativeRun = text.NativeRun
type NativeLine = text.NativeLine
type NativeTextProvider = text.NativeTextProvider

// UseFontProvider selects the host's native system fonts before any UI text is
// laid out. Desktop backends keep their default engine unless explicitly used.
func UseFontProvider(p NativeTextProvider) error { return text.UseNativeProvider(p) }

type TextParams = scene.TextParams
type NativeTextRendering = text.NativeTextRendering
type NativeRegisteredFonts = text.NativeRegisteredFonts
