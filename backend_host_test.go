//go:build !js

package mygo

import (
	"errors"
	"fmt"
	"github.com/egoist/mygo/internal/fake"
	"github.com/egoist/mygo/internal/platform"
	"os"
	"os/exec"
	"strings"
	"testing"
)

type hostedTestBackend struct {
	*fake.Backend
	handler platform.AppHandler
	stopped func()
	fail    bool
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
