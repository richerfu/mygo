package audio

import (
	"errors"
	"slices"
	"testing"
	"time"
)

type externalOutput struct {
	samples []float32
	closed  int
	err     error
}

func (d *externalOutput) Write(samples []float32) error {
	d.samples = append(d.samples, samples...)
	return d.err
}
func (d *externalOutput) Close() { d.closed++ }

func TestCustomOutput(t *testing.T) {
	setup(t)
	t.Cleanup(func() { UseOutput(nil) })
	want := &externalOutput{err: errors.New("write failed")}
	var gotAhead time.Duration
	UseOutput(func(ahead time.Duration) (Device, int, error) {
		gotAhead = ahead
		// Registration must not hold a lock while calling user code.
		UseOutput(nil)
		return want, 256, nil
	})
	d, n, err := newOutput(40 * time.Millisecond)
	if err != nil || n != 256 || gotAhead != 40*time.Millisecond {
		t.Fatalf("open: frames=%d ahead=%v err=%v", n, gotAhead, err)
	}
	samples := []float32{0.1, -0.2, 0.3, -0.4}
	if err := d.write(samples); err != want.err || !slices.Equal(want.samples, samples) {
		t.Fatalf("write: samples=%v err=%v", want.samples, err)
	}
	d.close()
	if want.closed != 1 {
		t.Fatalf("closed %d times", want.closed)
	}
}

func TestCustomOutputFailure(t *testing.T) {
	setup(t)
	t.Cleanup(func() { UseOutput(nil) })
	failure := errors.New("custom device unavailable")
	UseOutput(func(time.Duration) (Device, int, error) { return nil, 0, failure })
	if _, _, err := newOutput(time.Second); err != failure {
		t.Fatalf("custom opener error was lost: %v", err)
	}
	for _, tc := range []struct {
		d *externalOutput
		n int
	}{{nil, 64}, {&externalOutput{}, 0}, {&externalOutput{}, -1}} {
		UseOutput(func(time.Duration) (Device, int, error) {
			if tc.d == nil {
				return nil, tc.n, nil
			}
			return tc.d, tc.n, nil
		})
		if _, _, err := newOutput(time.Second); err == nil {
			t.Fatal("invalid opener result was accepted")
		}
		if tc.d != nil && tc.d.closed != 1 {
			t.Fatal("invalid output device was not closed")
		}
	}
}
