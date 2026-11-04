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
  and recovery, and releases the renderer when the surface closes.
- `backend/gles`: with the custom backend tag, reuse the existing GLES scene
  renderer through a context's `Resolve` and `Identity`. The adapter must make
  its context current for construction, drawing and release, and owns presenting
  and context lifetime.
- `backend/scene`: use the same scene representation as the normal renderers.

These packages stay separate so a WebView-only application does not retain text
or rendering dependencies merely by using platform contracts.

`backend.HostedBackend` lets a native host own the event loop. `App.Run`
registers the application and returns; the host calls its completion callback
exactly once. Keep resources alive until that callback instead of cleaning them
up with a `defer` immediately after `Run`. App and font state remain process-wide;
this does not add support for restarting an application inside the same process.

Targeted contract checks:

```sh
CGO_ENABLED=0 go test -tags mygo_custom_backend .
CGO_ENABLED=0 go test -tags mygo_custom_backend ./ui -run TestExternalSurfaceOwnsRendererLifecycle
CGO_ENABLED=0 go test -tags mygo_custom_backend ./internal/text -run TestNativeProviderPreservesClustersAndGlyphOrigin
```

Tests that lay out text under the custom backend tag must install a provider;
the ordinary full test suite uses the built-in backend. Host-specific GPU,
window, input and service integration requires runtime validation in the adapter.
