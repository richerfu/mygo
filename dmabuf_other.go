//go:build mygo_custom_backend || !(linux && !android && (amd64 || arm64))

package mygo

func linkDMABUFProbe() {}
