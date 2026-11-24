//go:build mygo_custom_backend

package text

func newEngine() engine {
	panic("mygo: custom backend must install a font provider before text is used")
}
