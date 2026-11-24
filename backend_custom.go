//go:build mygo_custom_backend

package mygo

import (
	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/internal/unsupported"
)

// An external host installs its backend through UseBackend before App.Run.
func newBackend() platform.Backend { return unsupported.New() }
func linkContentRules()            {}
func linkNowPlaying()              {}
func linkNotificationImages()      {}
func linkMenuIcons()               {}
func linkContextMenus()            {}

func enableNotificationSenders() {}
