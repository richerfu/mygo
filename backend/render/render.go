// Package render defines the optional custom GPU renderer capability.
// Importing the platform contracts alone does not link UI rendering code.
package render

import (
	"github.com/egoist/mygo/internal/gpu"
	"github.com/egoist/mygo/internal/scene"
)

type Renderer = gpu.Renderer
type Factory = gpu.RendererFactory
type Scene = scene.Scene
