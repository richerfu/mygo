package backend

import "github.com/egoist/mygo/internal/platform"

type TextSelectionClient = platform.TextSelectionClient

// ClientTextContext copies a bounded UTF-16 surrounding-text context.
func ClientTextContext(c TextInputClient) TextContext { return platform.ClientTextContext(c) }
