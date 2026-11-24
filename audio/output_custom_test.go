//go:build mygo_custom_backend

package audio

import (
	"testing"
	"time"
)

func TestCustomOutputDefaultIsUnsupported(t *testing.T) {
	setup(t)
	UseOutput(nil)
	d, n, err := newOutput(time.Second)
	if d != nil || n != 0 || err != errUnsupported {
		t.Fatalf("custom build opened OS output: %v, %d, %v", d, n, err)
	}
}
