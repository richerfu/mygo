//go:build mygo_custom_backend || !(linux && (amd64 || arm64))

package mygo

func linkDMABUFProbe() {}
