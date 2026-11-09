//go:build mygo_custom_backend && !js

package gles_test

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/backend/gles"
)

type missingContext struct{ resolved []string }

func (c *missingContext) Resolve(name string) uintptr {
	c.resolved = append(c.resolved, name)
	return 0
}
func (*missingContext) Identity() uintptr { return 1 }

var unavailableContext missingContext

func TestPresenterAndReadbackReportMissingDispatch(t *testing.T) {
	c := &unavailableContext
	p, err := gles.NewPresenter(c)
	if p != nil || err == nil || !strings.Contains(err.Error(), "glGetString") || len(c.resolved) == 0 {
		t.Fatalf("presenter did not validate host dispatch: %v, %v", p, err)
	}
	pix, err := gles.ReadFramebuffer(c, 1, 1)
	if pix != nil || err == nil || !strings.Contains(err.Error(), "glReadPixels") {
		t.Fatalf("readback did not report missing host dispatch: %v, %v", pix, err)
	}
}
