//go:build !js

package mygo

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	nativebackend "github.com/egoist/mygo/backend"
	"github.com/egoist/mygo/internal/fake"
	"github.com/egoist/mygo/internal/platform"
)

type hostedTestBackend struct {
	*fake.Backend
	handler platform.AppHandler
	stopped func()
	fail    bool
}

func (b *hostedTestBackend) ActivateMenuItem(window uintptr, path []string) error {
	if !b.IsMainThread() || window != 42 || len(path) != 2 || path[0] != "File" || path[1] != "Open" {
		return errors.New("incorrect menu dispatch")
	}
	return nil
}

func (b *hostedTestBackend) Init(h platform.AppHandler, o platform.AppOptions) error {
	b.handler = h
	return b.Backend.Init(h, o)
}
func (b *hostedTestBackend) RunHosted(stopped func()) error {
	b.stopped = stopped
	if b.fail {
		return errors.New("host registration failed")
	}
	b.handler.Ready()
	b.handler.Dispatch()
	return nil
}
func hostedTestHelper() {
	b := &hostedTestBackend{Backend: fake.New(), fail: os.Getenv("MYGO_TEST_HOSTED_FAIL") == "1"}
	if err := UseBackend(b); err != nil {
		panic(err)
	}
	menus, ok := platform.Current().(nativebackend.MenuAutomation)
	if !ok {
		panic("installed backend lost optional menu automation")
	}
	quits := 0
	App.OnQuit(func() { quits++ })
	err := App.Run()
	if b.fail {
		if err == nil || !App.ended || quits != 1 {
			panic("failed startup leaked application")
		}
	} else {
		if err != nil || App.ended || quits != 0 || !App.IsReady() {
			panic("hosted Run finished the live application")
		}
		if err := menus.ActivateMenuItem(42, []string{"File", "Open"}); err != nil {
			panic(err)
		}
		b.stopped()
		b.stopped()
		if !App.ended || quits != 1 {
			panic("hosted shutdown did not finish exactly once")
		}
	}
	fmt.Println("hosted lifecycle passed")
	os.Exit(0)
}
func TestHostedBackendLifecycle(t *testing.T) {
	for _, failure := range []string{"0", "1"} {
		t.Run(failure, func(t *testing.T) {
			cmd := exec.Command(os.Args[0])
			cmd.Env = append(os.Environ(), "MYGO_TEST_HOSTED=1", "MYGO_TEST_HOSTED_FAIL="+failure)
			out, err := cmd.CombinedOutput()
			if err != nil || !strings.Contains(string(out), "hosted lifecycle passed") {
				t.Fatalf("%v: %s", err, out)
			}
		})
	}
}
func TestBackendCannotBeReplacedAfterFirstUse(t *testing.T) {
	if err := UseBackend(fake.New()); err == nil {
		t.Fatal("replaced a running backend")
	}
}

func TestBackendMenuModelActivation(t *testing.T) {
	w, _ := testWindow(t, WindowOptions{})
	if w.Menu() != nil {
		t.Fatal("new window has an explicit menu")
	}
	clicks := 0
	check := &MenuItem{Label: "&Check", Type: MenuItemCheckbox, Click: func(it *MenuItem, win *Window) {
		if !fb.IsMainThread() || win != w || !it.IsChecked() {
			t.Error("click lost its UI thread, owner, or checkbox state")
		}
		clicks++
	}}
	m := NewMenu([]*MenuItem{{Label: "&File", Submenu: []*MenuItem{
		check,
		{Label: "Minimize", Role: RoleMinimize},
		{Label: "Disabled", Disabled: true, Click: func(*MenuItem, *Window) { t.Error("disabled item clicked") }},
		{Label: "Hidden", Hidden: true},
	}}})
	w.SetMenu(m)
	if w.Menu() != m {
		t.Fatal("window menu accessor lost its menu")
	}
	if err := ActivateMenuItem(m, w, "File", "Check"); err != nil || clicks != 1 {
		t.Fatalf("checkbox: clicks=%d err=%v", clicks, err)
	}
	if err := ActivateMenuItem(m, w, "File", "Minimize"); err != nil || !w.IsMinimized() {
		t.Fatalf("role: minimized=%v err=%v", w.IsMinimized(), err)
	}
	for _, tc := range []struct {
		path []string
		want string
	}{
		{nil, "no menu item"},
		{[]string{"File", ""}, "empty menu path segment"},
		{[]string{"Missing"}, `no menu item "Missing"`},
		{[]string{"File"}, `"File" is a submenu`},
		{[]string{"File", "Disabled"}, `"Disabled" is disabled`},
		{[]string{"File", "Hidden"}, `no menu item "Hidden"`},
		{[]string{"File", "Check", "Child"}, `"Check" has no submenu`},
	} {
		if err := ActivateMenuItem(m, w, tc.path...); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("path %q: %v, want %s", tc.path, err, tc.want)
		}
	}
	m.Items()[0].SetEnabled(false)
	if err := ActivateMenuItem(m, w, "File", "Check"); err == nil || clicks != 1 {
		t.Fatalf("disabled parent: clicks=%d err=%v", clicks, err)
	}
	if err := ActivateMenuItem(nil, w, "File"); err == nil || !strings.Contains(err.Error(), "no menu bar") {
		t.Fatalf("missing menu: %v", err)
	}
}
