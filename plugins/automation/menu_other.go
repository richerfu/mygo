//go:build !mygo_noinspector && (mygo_custom_backend || (!darwin && !(linux && (amd64 || arm64)) && !(windows && (amd64 || arm64))))

package automation

import "github.com/egoist/mygo"

// chooseMenuItem has no menus to choose from on other platforms.
func chooseMenuItem(*mygo.Window, []string) error {
	return errorf(codeUnsupported, "this platform has no menus to choose from")
}
