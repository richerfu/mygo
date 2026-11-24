package media

import "sync"

// Source describes a file/URL or immutable bytes. Path is set for local files;
// Data, when nonempty, is the source instead of URL/Path.
type Source struct {
	URL, Path string
	Data      []byte
}

type factoryBinding struct{ open func(Source) (Engine, error) }

var factories struct {
	sync.RWMutex
	current *factoryBinding
}

// UseFactory binds the media service of an external host. Releasing a binding
// cannot remove a newer binding, and never restores an older, disposed host.
func UseFactory(open func(Source) (Engine, error)) func() {
	b := &factoryBinding{open: open}
	factories.Lock()
	factories.current = b
	factories.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			factories.Lock()
			defer factories.Unlock()
			if factories.current == b {
				factories.current = nil
			}
		})
	}
}

func Factory() func(Source) (Engine, error) {
	factories.RLock()
	defer factories.RUnlock()
	if factories.current != nil {
		return factories.current.open
	}
	return nil
}
