// Package media exposes the optional video engine contract to external hosts.
// Installing a factory opens no player and starts no polling or worker.
package media

import "github.com/egoist/mygo/internal/media"

type Engine = media.Engine
type Picture = media.Picture
type Source = media.Source
type State = media.State

const (
	Loading   = media.Loading
	Ready     = media.Ready
	Buffering = media.Buffering
	Ended     = media.Ended
	Failed    = media.Failed
)

func UseFactory(open func(Source) (Engine, error)) func() { return media.UseFactory(open) }
