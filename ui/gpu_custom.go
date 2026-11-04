//go:build mygo_custom_backend

package ui

import (
	"errors"
	"github.com/egoist/mygo/internal/platform"
)

func newGPURenderer(platform.SurfaceNative) (gpuRenderer, error) {
	return nil, errors.New("mygo: custom surface must supply a renderer factory")
}
