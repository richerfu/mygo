package backend

// WindowBase supplies inert native properties and unsupported web-page APIs.
// An external native host embeds it and implements window/surface lifetime and
// the properties it supports. Every asynchronous method completes exactly once.
type WindowBase struct{}

var _ Window = (*WindowBase)(nil)

func (*WindowBase) Handle() uintptr                      { var r0 uintptr; return r0 }
func (*WindowBase) Surface() Surface                     { var r0 Surface; return r0 }
func (*WindowBase) SetTitle(p0 string)                   {}
func (*WindowBase) Title() string                        { var r0 string; return r0 }
func (*WindowBase) SetRepresentedFile(p0 string)         {}
func (*WindowBase) SetDocumentEdited(p0 bool)            {}
func (*WindowBase) SetBounds(p0 Rect)                    {}
func (*WindowBase) Bounds() Rect                         { var r0 Rect; return r0 }
func (*WindowBase) SetContentBounds(p0 Rect)             {}
func (*WindowBase) ContentBounds() Rect                  { var r0 Rect; return r0 }
func (*WindowBase) SetMinimumSize(p0 Size)               {}
func (*WindowBase) SetMaximumSize(p0 Size)               {}
func (*WindowBase) SetResizable(p0 bool)                 {}
func (*WindowBase) IsResizable() bool                    { var r0 bool; return r0 }
func (*WindowBase) SetMovable(p0 bool)                   {}
func (*WindowBase) IsMovable() bool                      { var r0 bool; return r0 }
func (*WindowBase) SetMinimizable(p0 bool)               {}
func (*WindowBase) IsMinimizable() bool                  { var r0 bool; return r0 }
func (*WindowBase) SetMaximizable(p0 bool)               {}
func (*WindowBase) IsMaximizable() bool                  { var r0 bool; return r0 }
func (*WindowBase) SetClosable(p0 bool)                  {}
func (*WindowBase) IsClosable() bool                     { var r0 bool; return r0 }
func (*WindowBase) SetAlwaysOnTop(p0 bool)               {}
func (*WindowBase) IsAlwaysOnTop() bool                  { var r0 bool; return r0 }
func (*WindowBase) Show()                                {}
func (*WindowBase) ShowInactive()                        {}
func (*WindowBase) Hide()                                {}
func (*WindowBase) IsVisible() bool                      { var r0 bool; return r0 }
func (*WindowBase) Focus()                               {}
func (*WindowBase) Blur()                                {}
func (*WindowBase) IsFocused() bool                      { var r0 bool; return r0 }
func (*WindowBase) Minimize()                            {}
func (*WindowBase) IsMinimized() bool                    { var r0 bool; return r0 }
func (*WindowBase) Maximize()                            {}
func (*WindowBase) Unmaximize()                          {}
func (*WindowBase) IsMaximized() bool                    { var r0 bool; return r0 }
func (*WindowBase) Restore()                             {}
func (*WindowBase) SetFullScreen(p0 bool)                {}
func (*WindowBase) IsFullScreen() bool                   { var r0 bool; return r0 }
func (*WindowBase) Center()                              {}
func (*WindowBase) SetBackgroundColor(p0 Color)          {}
func (*WindowBase) SetOpacity(p0 float64)                {}
func (*WindowBase) Opacity() float64                     { var r0 float64; return r0 }
func (*WindowBase) SetHasShadow(p0 bool)                 {}
func (*WindowBase) HasShadow() bool                      { var r0 bool; return r0 }
func (*WindowBase) SetIgnoreMouseEvents(p0 bool)         {}
func (*WindowBase) SetContentProtection(p0 bool)         {}
func (*WindowBase) SetVibrancy(p0 string)                {}
func (*WindowBase) SetActiveAppearance(p0 bool)          {}
func (*WindowBase) SetWindowButtonsTheme(p0 string)      {}
func (*WindowBase) SetProgressBar(p0 string, p1 float64) {}
func (*WindowBase) FlashFrame(p0 bool)                   {}
func (*WindowBase) SetSkipTaskbar(p0 bool)               {}
func (*WindowBase) SetVisibleOnAllWorkspaces(p0 bool)    {}
func (*WindowBase) SetIcon(p0 []byte) error              { return ErrUnsupported }
func (*WindowBase) SetMenu(p0 *Menu)                     {}
func (*WindowBase) SetAutoHideMenu(p0 bool)              {}
func (*WindowBase) TitleBar() TitleBar                   { var r0 TitleBar; return r0 }
func (*WindowBase) StartDrag()                           {}
func (*WindowBase) TitleBarDoubleClicked()               {}
func (*WindowBase) PlaceFlyout(p0 Flyout, p1 Size)       {}
func (*WindowBase) Close()                               {}
func (*WindowBase) WebViewHandle() uintptr               { var r0 uintptr; return r0 }
func (*WindowBase) LoadURL(p0 string)                    {}
func (*WindowBase) LoadHTML(p0 string, p1 string)        {}
func (*WindowBase) LoadFile(p0 string, p1 string)        {}
func (*WindowBase) Reload(p0 bool)                       {}
func (*WindowBase) StopLoading()                         {}
func (*WindowBase) GoBack()                              {}
func (*WindowBase) GoForward()                           {}
func (*WindowBase) CanGoBack() bool                      { var r0 bool; return r0 }
func (*WindowBase) CanGoForward() bool                   { var r0 bool; return r0 }
func (*WindowBase) URL() string                          { var r0 string; return r0 }
func (*WindowBase) IsLoading() bool                      { var r0 bool; return r0 }
func (*WindowBase) Eval(p0 string)                       {}
func (*WindowBase) SetUserScripts(p0 []UserScript)       {}
func (*WindowBase) SetContentRules(p0 []any)             {}
func (*WindowBase) CallAsyncFunction(p0 string, p1 func(result string, err error)) {
	var z0 string
	if p1 != nil {
		p1(z0, ErrUnsupported)
	}
}
func (*WindowBase) SetZoom(p0 float64)                              {}
func (*WindowBase) Zoom() float64                                   { var r0 float64; return r0 }
func (*WindowBase) SetUserAgent(p0 string)                          {}
func (*WindowBase) UserAgent() string                               { var r0 string; return r0 }
func (*WindowBase) ProcessID() int                                  { var r0 int; return r0 }
func (*WindowBase) IsAudible() bool                                 { var r0 bool; return r0 }
func (*WindowBase) SetAudioMuted(p0 bool)                           {}
func (*WindowBase) IsAudioMuted() bool                              { var r0 bool; return r0 }
func (*WindowBase) CaptureState(p0 CaptureKind) CaptureState        { var r0 CaptureState; return r0 }
func (*WindowBase) SetCaptureState(p0 CaptureKind, p1 CaptureState) {}
func (*WindowBase) TogglePictureInPicture()                         {}
func (*WindowBase) PictureInPictureActive(p0 func(active bool)) {
	var z0 bool
	if p0 != nil {
		p0(z0)
	}
}
func (*WindowBase) SessionState() []byte               { var r0 []byte; return r0 }
func (*WindowBase) RestoreSessionState(p0 []byte) bool { var r0 bool; return r0 }
func (*WindowBase) OpenDevTools()                      {}
func (*WindowBase) CloseDevTools()                     {}
func (*WindowBase) IsDevToolsOpened() bool             { var r0 bool; return r0 }
func (*WindowBase) CapturePage(p0 func(png []byte, err error)) {
	var z0 []byte
	if p0 != nil {
		p0(z0, ErrUnsupported)
	}
}
func (*WindowBase) PrintToPDF(p0 PDFOptions, p1 func(pdf []byte, err error)) {
	var z0 []byte
	if p1 != nil {
		p1(z0, ErrUnsupported)
	}
}
func (*WindowBase) DroppedFiles() []string { var r0 []string; return r0 }
func (*WindowBase) Print()                 {}

func (*WindowBase) GPUProcessID() int { return 0 }

func (*WindowBase) IgnoresMouseEvents() bool { return false }
func (*WindowBase) SetVibrancyTint(*Color)   {}
