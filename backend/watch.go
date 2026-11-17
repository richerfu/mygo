package backend

import (
	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/internal/watchdriver"
)

// FileWatchDriver drives one OS-backed file watch via the shared watchdriver loop.
type FileWatchDriver = watchdriver.Driver

// NewFileWatch starts a FileWatch running d and delivering notices to notify.
func NewFileWatch(d FileWatchDriver, notify func(FileWatchNotice)) FileWatch {
	if notify == nil {
		notify = func(platform.FileWatchNotice) {}
	}
	return watchdriver.New(d, notify)
}
