//go:build !mygo_noinspector && (mygo_custom_backend || (!darwin && !(linux && (amd64 || arm64)) && !(windows && (amd64 || arm64))))

package automation

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/internal/platform"
)

// chooseMenuItem uses the host's menu automation when available, otherwise
// activates the core's menu model. Main thread only.
func chooseMenuItem(w *mygo.Window, path []string) error {
	if len(path) == 0 {
		return errorf(codeInvalid, "menu needs the path of the item, as [\"File\", \"New\"]")
	}
	for _, label := range path {
		if label == "" {
			return errorf(codeInvalid, "menu path has an empty segment")
		}
	}
	if b, ok := platform.Current().(platform.MenuAutomation); ok {
		return b.ActivateMenuItem(w.NativeHandle(), path)
	}
	m := w.Menu()
	if m == nil {
		m = mygo.App.Menu()
	}
	return mygo.ActivateMenuItem(m, w, path...)
}
