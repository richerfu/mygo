# Custom native backends

An external platform adapter installs `backend.Backend` using `mygo.UseBackend`
before any API initializes the platform. Installation is process-wide and cannot
be replaced after first use. Build with `-tags mygo_custom_backend` to exclude
the built-in platform and font implementations. MyGo itself still builds with
`CGO_ENABLED=0`; the adapter owns its native interop.

The public `backend` package aliases the contracts used by built-in platforms.
`backend.Base` and `backend.WindowBase` provide unsupported defaults for optional
operations. Implement the actual capabilities in the adapter, keep UI callbacks
on its serial application thread, and honor the existing main-thread dispatch
contracts. Controls and application code continue to use `mygo` and `ui`.

Native UI uses separate, optional extensions:

- `backend/text`: install a `NativeTextProvider` before laying out text. Preserve
  clusters, glyph origins, fallback fonts, composition and inline objects; the
  framework retains its layout, editing and glyph-atlas implementation.
- `backend/render`: a surface may implement `Factory` to supply its renderer.
  The UI uses this before inspecting native handles, retains normal invalidation
  and recovery, and releases the renderer when the surface closes. A renderer
  or surface can also implement `render.Capturer` for screenshots (see below).
- `backend/gles`: with the custom backend tag, reuse the existing GLES scene
  renderer through a context's `Resolve` and `Identity`. The adapter must make
  its context current for construction, drawing and release, and owns presenting
  and context lifetime.
- `backend/scene`: use the same scene representation as the normal renderers.

These packages stay separate so a WebView-only application does not retain text
or rendering dependencies merely by using platform contracts.

`gles.New(context, retained)` draws complete scenes on the GPU.
`gles.NewPresenter(context)` instead returns a `*gles.Presenter` whose
`Present(pixels, stride, width, height)` copies software-rendered frames into
the bound framebuffer. `gles.ReadFramebuffer(context, width, height)` reads
the bound framebuffer back to memory. Both use the existing GL convention:
premultiplied **BGRA**, with rows starting at the top; readback is tightly
packed. Convert RGBA software buffers to BGRA before presenting them. All GLES
contexts in a process share the first installed function resolver; `Identity`
must identify the context currently bound. Make it current for every call.
The host presents the framebuffer and destroys the context, which releases
the presenter's textures.

`render.Capturer` has this optional method:

```go
Capture(s *render.Scene) (width, height int, rgba []byte, err error)
```

For a window with an active GPU renderer, screenshots try its `Capturer`, or
the surface's if the renderer does not implement it. MyGo supplies the latest
complete scene, building a pending frame first when needed. Capture runs on
the UI thread; the implementation owns binding its target and rendering or
reading it. Return an owned buffer of exactly `width * height * 4` bytes of
premultiplied **RGBA**, top row first, with positive pixel dimensions. Convert
BGRA readback from `gles.ReadFramebuffer` to RGBA here. Errors, empty or invalid
buffers, and missing hooks use the existing CPU scene renderer instead.

An optional `backend.MenuAutomation` lets the automation plugin activate native
menus with `ActivateMenuItem(window uintptr, path []string) error`. The handle
is the window's native handle, and the path contains its visible labels, such
as `[]string{"File", "Open"}`. Calls run on the UI thread after path validation;
the backend checks availability and returns failures to the caller. Without
this hook, custom backends use `Window.Menu()`, falling back to `App.Menu()`
when nil. The core's `mygo.ActivateMenuItem(menu, window, path...)` resolves
labels without `&` mnemonics, rejects hidden, disabled and submenu targets,
and dispatches normal roles, checkbox/radio changes and click callbacks.
An explicit empty window menu does not fall back to the application menu.
Automation remains excluded by `mygo_noinspector`.

Audio is independent of the UI thread. Register `audio.UseOutput(opener)` before
playback, where `audio.OutputOpener` is
`func(ahead time.Duration) (audio.Device, int, error)`. It returns an output
device and a positive number of stereo frames per write. A device implements
`Write([]float32) error` and `Close()`: samples are interleaved stereo at 48 kHz,
borrowed for the write; writes block while enough audio is queued. Opening,
writing and closing happen on the audio goroutine's locked thread. An opener
error follows the normal silent-output/retry path. `UseOutput(nil)` restores
the default for the next open; registration does not interrupt an open device.
Custom backend builds exclude built-in OS and browser audio implementations,
so without an opener they use silent unsupported output and load no OS audio
libraries. Ordinary browser builds retain their AudioWorklet output.

`backend.HostedBackend` lets a native host own the event loop. `App.Run`
registers the application and returns; the host calls its completion callback
exactly once. Keep resources alive until that callback instead of cleaning them
up with a `defer` immediately after `Run`. App and font state remain process-wide;
this does not add support for restarting an application inside the same process.

Targeted contract checks:

```sh
CGO_ENABLED=0 go test -tags mygo_custom_backend . -run 'TestHostedBackendLifecycle|TestBackend'
CGO_ENABLED=0 go test -tags mygo_custom_backend ./backend/... ./audio ./plugins/automation
CGO_ENABLED=0 go test -tags mygo_custom_backend ./ui -run 'TestExternal'
CGO_ENABLED=0 go test -tags mygo_custom_backend ./internal/text -run TestNativeProviderPreservesClustersAndGlyphOrigin
```

Tests that lay out text under the custom backend tag must install a provider;
the ordinary full test suite uses the built-in backend. Host-specific GPU,
window, input and service integration requires runtime validation in the adapter.
