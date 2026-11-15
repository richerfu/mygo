//go:build mygo_custom_backend || (!(linux && (amd64 || arm64)) && !darwin && !(windows && (amd64 || arm64)) && !js)

package video

import "github.com/egoist/mygo/internal/media"

func newEngine(s source) (media.Engine, error) {
	if open := media.Factory(); open != nil {
		return open(media.Source{URL: s.url, Path: s.path, Data: s.data})
	}
	return nil, ErrUnsupported
}
