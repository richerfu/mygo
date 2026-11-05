//go:build mygo_custom_backend || (!js && ((!darwin && !linux && !windows) || (!arm64 && !amd64)))

package audio

import "time"

func openDevice(ahead time.Duration) (device, int, error) { return nil, 0, errUnsupported }
