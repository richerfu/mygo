//go:build !mygo_custom_backend && !(windows && (amd64 || arm64)) && !darwin && !(linux && (amd64 || arm64)) && !js

package text

func newEngine() engine { return &stubEngine{} }
