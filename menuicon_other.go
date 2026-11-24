//go:build !mygo_custom_backend && !(darwin && !ios) && !(linux && !android && (amd64 || arm64)) && !(windows && (amd64 || arm64))

package mygo

func linkMenuIcons() {}
