//go:build !mygo_custom_backend && !darwin && !(linux && (amd64 || arm64)) && !(windows && (amd64 || arm64))

package mygo

func linkMenuIcons() {}
