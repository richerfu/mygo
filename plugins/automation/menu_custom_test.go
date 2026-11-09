//go:build mygo_custom_backend && !mygo_noinspector && !js

package automation

import (
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/internal/fake"
	"github.com/egoist/mygo/internal/platform"
)

var menuTestBackend *fake.Backend
var menuTestNative *nativeMenuBackend
var menuTestFailure = errors.New("native menu failed")

type nativeMenuBackend struct {
	*fake.Backend
	handle uintptr
	path   []string
	onUI   bool
	calls  int
}

func (b *nativeMenuBackend) ActivateMenuItem(handle uintptr, path []string) error {
	b.handle, b.path, b.onUI = handle, slices.Clone(path), b.IsMainThread()
	b.calls++
	return menuTestFailure
}

func TestMain(m *testing.M) {
	menuTestBackend = fake.New()
	var b platform.Backend = menuTestBackend
	if os.Getenv("MYGO_TEST_MENU_NATIVE") == "1" {
		menuTestNative = &nativeMenuBackend{Backend: menuTestBackend}
		b = menuTestNative
	}
	if err := mygo.UseBackend(b); err != nil {
		panic(err)
	}
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.WhenReady(func() { go func() { os.Exit(m.Run()) }() })
	if err := mygo.App.Run(); err != nil {
		panic(err)
	}
}

func chooseForTest(w *mygo.Window, path ...string) error {
	var err error
	mygo.RunOnMain(func() { err = chooseMenuItem(w, path) })
	return err
}

func TestCustomMenuNativeHook(t *testing.T) {
	if menuTestNative == nil {
		cmd := exec.Command(os.Args[0], "-test.run=^TestCustomMenuNativeHook$")
		cmd.Env = append(os.Environ(), "MYGO_TEST_MENU_NATIVE=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("native hook: %v\n%s", err, out)
		}
		return
	}
	w := mygo.NewWindow(mygo.WindowOptions{})
	t.Cleanup(w.Destroy)
	for _, path := range [][]string{nil, {""}, {"File", ""}} {
		var invalid *Error
		if err := chooseForTest(w, path...); !errors.As(err, &invalid) || invalid.Code != codeInvalid {
			t.Fatalf("invalid path %q: %v", path, err)
		}
	}
	if menuTestNative.calls != 0 {
		t.Fatal("invalid path reached host")
	}
	if err := chooseForTest(w, "File", "Open"); err != menuTestFailure {
		t.Fatalf("host error was lost: %v", err)
	}
	b := menuTestNative
	if b.calls != 1 || !b.onUI || b.handle != w.NativeHandle() || !slices.Equal(b.path, []string{"File", "Open"}) {
		t.Fatalf("host call: handle=%d path=%q UI=%v calls=%d", b.handle, b.path, b.onUI, b.calls)
	}
}

func TestCustomMenuFallback(t *testing.T) {
	w := mygo.NewWindow(mygo.WindowOptions{})
	t.Cleanup(w.Destroy)
	previous := mygo.App.Menu()
	t.Cleanup(func() { mygo.App.SetMenu(previous) })
	appClicks, windowClicks := 0, 0
	makeMenu := func(clicks *int) *mygo.Menu {
		return mygo.NewMenu([]*mygo.MenuItem{{Label: "&File", Submenu: []*mygo.MenuItem{
			{Label: "&Open", Click: func(_ *mygo.MenuItem, owner *mygo.Window) {
				if !menuTestBackend.IsMainThread() || owner != w {
					t.Error("fallback lost UI thread or window")
				}
				*clicks++
			}},
		}}})
	}
	mygo.App.SetMenu(makeMenu(&appClicks))
	if err := chooseForTest(w, "File", "Open"); err != nil || appClicks != 1 {
		t.Fatalf("app fallback: clicks=%d err=%v", appClicks, err)
	}
	w.SetMenu(makeMenu(&windowClicks))
	if err := chooseForTest(w, "File", "Open"); err != nil || windowClicks != 1 || appClicks != 1 {
		t.Fatalf("window precedence: window=%d app=%d err=%v", windowClicks, appClicks, err)
	}
	w.SetMenu(mygo.NewMenu(nil))
	if err := chooseForTest(w, "File", "Open"); err == nil || appClicks != 1 {
		t.Fatalf("explicit empty menu used application menu: clicks=%d err=%v", appClicks, err)
	}
	w.SetMenu(nil)
	mygo.App.SetMenu(nil)
	if err := chooseForTest(w, "File", "Open"); err == nil || !strings.Contains(err.Error(), "no menu bar") {
		t.Fatalf("missing menus: %v", err)
	}
}
