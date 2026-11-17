package backend

import "github.com/egoist/mygo/internal/platform"

type TextSelectionClient = platform.TextSelectionClient

// KeyCodeForDOM maps a Web UI Events KeyboardEvent.code string to KeyCode.
func KeyCodeForDOM(code string) KeyCode { return platform.KeyCodeForDOM(code) }

// KeyCodeForEvdev maps a Linux evdev key code to KeyCode.
func KeyCodeForEvdev(code uint16) KeyCode { return platform.KeyCodeForEvdev(code) }

// ClientTextContext copies a bounded UTF-16 surrounding-text context.
func ClientTextContext(c TextInputClient) TextContext { return platform.ClientTextContext(c) }
