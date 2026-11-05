package audio

import (
	"errors"
	"runtime"
	"sync"
	"time"
)

// Device plays interleaved stereo float32 samples at 48 kHz. Write must wait
// while the output holds enough audio already and must not retain samples.
// An error closes the device and causes the output to open it again.
// All calls, including the opener, run on the audio goroutine's locked thread,
// independently of the application's UI thread.
type Device interface {
	Write(samples []float32) error
	Close()
}

// OutputOpener opens an output holding about ahead of audio and returns its
// preferred number of stereo frames per Write (positive on success).
type OutputOpener func(ahead time.Duration) (Device, int, error)

var customOutput struct {
	sync.RWMutex
	open OutputOpener
}

// UseOutput registers an output opener for subsequent device opens. Call it
// before playing audio; it does not interrupt a device already open. nil
// restores the built-in output. Registration is safe from any goroutine.
// An opener's error uses the normal silent-output retry path, not OS audio.
func UseOutput(open OutputOpener) {
	customOutput.Lock()
	customOutput.open = open
	customOutput.Unlock()
}

type customDevice struct{ Device }

func (d customDevice) write(samples []float32) error { return d.Write(samples) }
func (d customDevice) close()                        { d.Close() }

func newOutput(ahead time.Duration) (device, int, error) {
	customOutput.RLock()
	open := customOutput.open
	customOutput.RUnlock()
	if open != nil {
		d, n, err := open(ahead)
		if err != nil {
			return nil, 0, err
		}
		if d == nil || n <= 0 {
			if d != nil {
				d.Close()
			}
			return nil, 0, errors.New("audio: output opener returned an invalid device or frame count")
		}
		return customDevice{d}, n, nil
	}
	return openOutput(ahead)
}

// A device is the system's audio output, opened by openDevice to hold
// about ahead of what is heard.
type device interface {
	// write plays interleaved stereo frames after those written before,
	// waiting while the device holds enough already. An error closes the
	// device, and the output opens it again.
	write(samples []float32) error
	close()
}

// idleClose is how long the output stays open with nothing playing.
const idleClose = 5 * time.Second

var (
	openOutput = openDevice // tests replace it

	wake     = make(chan struct{}, 1)
	runOnce  sync.Once
	testMode bool // tests mix by hand
)

// wakeOutput starts the output, or tells it a voice started. The caller
// holds mu.
func wakeOutput() {
	if testMode {
		return
	}
	runOnce.Do(func() { go run() })
	select {
	case wake <- struct{}{}:
	default:
	}
}

func active() bool {
	mu.Lock()
	defer mu.Unlock()
	return len(voices) > 0 && !testMode
}

// mixFor mixes into buf for an output open at latency lat, and reports
// whether anything plays; ok is false, and it mixes nothing, once tests
// mix by hand or the latency changed.
func mixFor(buf []float32, lat Latency) (busy, ok bool) {
	mu.Lock()
	defer mu.Unlock()
	if testMode || latency != lat {
		return false, false
	}
	return mixLocked(buf), true
}

// run opens the device while anything plays, and mixes into it.
func run() {
	// Devices that need it, as COM's, stay on one thread.
	runtime.LockOSThread()
	for {
		for !active() {
			<-wake
		}
		lat := outputLatency()
		dev, n, err := newOutput(lat.ahead())
		if err != nil {
			setErr(err)
			dev, n = newNullDevice(err != errUnsupported), nullFrames
			// It holds nothing: what it takes is heard as it takes it.
			setHeld(0)
		} else {
			setErr(nil)
			setHeld(int64(lat.ahead() * sampleRate / time.Second))
		}
		buf := make([]float32, n*channels)
		idle := time.Duration(0)
		per := frames(int64(n), sampleRate)
		for {
			busy, ok := mixFor(buf, lat)
			if !ok {
				break
			}
			if err := dev.write(buf); err != nil {
				// Lost, or a null device trying the real one again.
				break
			}
			if busy {
				idle = 0
			} else if idle += per; idle >= idleClose {
				break
			}
		}
		dev.close()
	}
}

var errUnsupported = errors.New("audio: no audio output on this platform")

// nullFrames is how many frames a null device takes at a time: 10 ms.
const nullFrames = sampleRate / 100

// A nullDevice plays nothing, on time: players still end when they would
// have. One standing in for a device that failed to open fails after a
// while, so that the output tries the device again.
type nullDevice struct {
	next  time.Time
	retry time.Time
	// now and sleep are its clock, which tests replace.
	now   func() time.Time
	sleep func(time.Duration)
}

func newNullDevice(retry bool) *nullDevice {
	d := &nullDevice{next: time.Now(), now: time.Now, sleep: time.Sleep}
	if retry {
		d.retry = d.next.Add(2 * time.Second)
	}
	return d
}

var errRetry = errors.New("audio: try the device again")

func (d *nullDevice) write(samples []float32) error {
	now := d.now()
	if !d.retry.IsZero() && now.After(d.retry) {
		return errRetry
	}
	if now.Sub(d.next) > 100*time.Millisecond {
		// Behind, as after a sleep: don't hurry to catch up.
		d.next = now
	}
	d.next = d.next.Add(frames(int64(len(samples)/channels), sampleRate))
	d.sleep(d.next.Sub(d.now()))
	return nil
}

func (d *nullDevice) close() {}
