//go:build mygo_custom_backend || (!(darwin && !ios) && !(linux && !android) && !windows)

package netproxy

import "net/url"

// In browsers the browser makes the requests, through its own proxy; other
// systems go direct.
func system(*url.URL) *url.URL { return nil }
