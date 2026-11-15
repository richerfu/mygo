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
- `backend/media`: install `UseFactory(func(Source) (Engine, error))` for
  `ui/video.Open`, `OpenURL` and `OpenData` under the custom backend tag. Sources
  retain file paths/URLs or input bytes. An engine supplies immutable pictures,
  playback controls and state; callbacks only wake the UI, and an event-driven
  engine returns zero from `Interval`. Releasing a factory prevents future opens
  without removing a newer binding or restoring an older disposed host. The
  host remains responsible for closing players it already opened. Registering a
  factory opens no player. Without one, custom builds return `ErrUnsupported`.

For decoder frames, `backend/gles.UseVideoUpload` registers a callback that takes
an opaque `backend/scene.Surface.Native` token and fills the renderer's RGBA8
texture directly in its current context. This registration exists only in custom
GLES builds and adds no fields to the shared scene types. The callback preserves
GL state and keeps source reads valid until the GPU has finished them. Return false to
use the picture's `RGBA` restoration path. Pictures stay valid until the engine
returns another picture or closes; the host owns native leases throughout that
interval. The renderer caches uploads by image version and replaces targets on
resize. This contract expresses SDR pictures; it does not imply HDR presentation.

`backend/gles.UseVideoSampler` additionally imports an owned external OES texture.
With `GL_OES_EGL_image_external_essl3`, ordinary image draws sample the decoder
allocation directly, with no intermediate RGBA texture. The host supplies crop,
orientation, transfer, gamut and range metadata; the renderer normalizes and
premultiplies texels before filtering, including PQ/HLG-to-SDR conversion. Use a
non-NV12 surface format when the importer returns RGB samples rather than separate
luma/chroma planes, so image blur remains available. A non-nil `VideoTexture`
transfers one lease: `Release` runs with its context current before `Render`
returns, also on failure, and must finish the GPU's reads. Repeated uses of the
same image version share that lease within a render. Blur and custom shader image
inputs need a 2D texture and convert lazily through the owned view's `Copy2D`, or
the upload/RGBA fallback. Unsupported external import also falls back in the
normal SDR renderer. None of these hooks enter default platform builds.

`gles.NewColor(context, retained)` opts into color-managed presentation. Its
`SetOutput(gles.ColorOutput{Encoding: ...})` describes the framebuffer the host
has already negotiated and bound; it does not configure an OS window.
`ColorExtendedSRGB` needs RGBA16F or RGBA32F, `ColorDisplayP3` needs at least
eight RGB bits, and `ColorBT2020PQ` needs at least ten. These are encoded outputs
in a **linear framebuffer attachment**, not an sRGB attachment that encodes
again. The host must tag the surface with the same color space and HDR metadata,
choose a display-compatible buffer/configuration and present it. The renderer
rejects incompatible targets rather than silently clipping HDR to eight bits.

Wide modes preserve extended sRGB through the scene, canvases, image blur,
backdrops and effects using RGBA16F intermediates. Oklab gradients retain signed
colors outside sRGB; GLSL effects receive `e.wide` under `MYGO_WIDE_COLOR`.
Ordinary images and glyph atlases retain their sRGB representation. External
video is converted and premultiplied before interpolation; direct draws sample
the decoder allocation, while blur and shader inputs convert lazily to RGBA16F.
`ReferenceWhiteNits` defaults to 203 and accepts 1–10000; `HLGPeakNits` defaults
to 1000 and accepts 400–10000. They control PQ/HLG conversion into the working
space; HLG uses a zero reference black and BT.2100-3 system gamma. Final
presentation converts to P3 or BT.2020 PQ only after composition.

If direct import is unavailable, `SetExtendedVideoUpload` supplies a synchronous
uploader into the renderer's RGBA16F texture. It receives the native token,
texture name, reference white and HLG peak; it must preserve GL state, keep
source data alive through GPU reads, and report conversion errors. Without
this path, a wide render reports an error instead of using an SDR restoration.
The host can explicitly select `ColorSRGB` and rerender for an SDR fallback.
`ColorRenderer.Capture` implements `render.Capturer`: it renders into a private
RGBA8 target and returns an SDR, premultiplied RGBA snapshot with top-first rows.
It preserves the configured output and both framebuffer bindings, and invalidates
retained drawing before the next live frame. A retained context may be leased
without a window surface for capture; the host need not change the live window
format or its color metadata. Output changes invalidate retained scenes, canvas formats
and converted video caches. Reconfigure on display changes, and make every
render/release with its context current. These APIs provide a renderer; OS
HDR output and real decoder import still require host/device validation.

`netproxy.UseTransport` optionally registers a host transport wrapper for ordered
proxy policies. Use `netproxy.WrapTransport(nil)` for the standard configuration,
or configure `netproxy.Transport().Clone()` and pass it to `WrapTransport`.
MyGo's updater, fetch and websocket clients use this path. Clients constructed
before host startup select the wrapper when making requests; replacing or releasing
a binding retires its idle pool without interrupting requests already in progress.
Without a wrapper,
behavior remains the existing transport. `UseResolver` and `Proxy` still expose
the single-URL compatibility hook. A host implementing failover must preserve
environment/loopback precedence and avoid replaying an already sent request.

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

Release size check for the optional transport and video-upload hooks (Go 1.27.3,
CGO disabled, `-trimpath -buildvcs=false -tags mygo_noinspector`, stripped
production builds): `counter-native` stays at 9,970,226 bytes and `hello` at
9,765,058 bytes. Their linker dependency graphs are unchanged. The registration
hooks are absent from both applications.
