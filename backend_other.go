//go:build !mygo_custom_backend && !(darwin && !ios) && !(linux && !android && (amd64 || arm64)) && !(windows && (amd64 || arm64)) && !js && !ios && !android

package mygo

import (
	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/internal/unsupported"
)

func newBackend() platform.Backend { return unsupported.New() }
