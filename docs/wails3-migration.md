# Wails 3 migration

Migration target: Wails **v3.0.0-beta.33**, verified against the Go module proxy and npm on 2026-10-20.
This is a beta release. The Go module, CLI used by CI, and `@wailsio/runtime` are pinned to the same release.

Official references:
- https://github.com/wailsapp/wails/releases/tag/v3.0.0-beta.33
- https://v3.wails.io/migration/v2-to-v3/

## Feature mapping

| Existing capability | Wails 3 implementation | Decision |
| --- | --- | --- |
| Wails startup and Go bindings | `application.New`, `application.NewService`, generated `frontend/bindings` | Replace v2 startup and generated runtime |
| Transparent, frameless, resizable, always-on-top window | `WebviewWindowOptions`, `BackgroundTypeTransparent`, native Window API | Replace v2 window options; retain React appearance settings |
| Window size, centering, minimising, maximising and reloading | `@wailsio/runtime` Window API | Replace runtime calls; adapt Size's width/height fields |
| Global boss key | `app.GlobalShortcut.Register("CmdOrCtrl+Space", ...)` | Replace `golang.design/x/hotkey`, platform wrappers, goroutine and manual cleanup |
| Single instance | `SingleInstanceOptions`, same unique ID, unminimise/show/focus | Replace v2 instance lock; retain launchArgs event |
| File drop | `EnableFileDrop`, `WindowFilesDropped`, frontend event and drop-target attribute | Replace v2 OnFileDrop; retain existing import pipeline |
| File picker | Native Wails 3 open dialog attached to the reader window | Replace v2 dialog and propagate native errors |
| Open external URLs | Runtime Browser API | Replace v2 runtime |
| Frontend error logs | Go service LogError, existing rotating logrus log | Keep rotating logging; forward framework logs through slog |
| TXT/EPUB/MOBI/AZW/AZW3 parsing, remote conversion | Existing book modules and conversion service | Retain: Wails does not provide an ebook parsing engine |
| Chapter splitting, search, pagination and reading preferences | Existing Go and React code | Retain |
| Hide content when the pointer leaves the window | Existing React mouse handlers | Retain: Wails HideOnFocusLost hides a window on focus loss, which differs from this feature |
| Windows portable ZIP, macOS DMG, Linux DEB/AppImage | Wails 3 Taskfiles and platform release workflows | Replace v2 build/packaging commands; keep output formats |

The inactive root `server.go` file service is retained and still not started. Reading uses Go bindings.

## Development

Requirements:
- Go 1.26 or newer (beta.33 requires Go 1.26.0; upgrade validation used Go 1.26.0).
- Node.js 24 LTS and npm recommended (upgrade validation used Node 22.22.3 and npm 10.9.8).
- Wails CLI v3.0.0-beta.33 on PATH.
- Windows: WebView2 Runtime.
- macOS: Xcode command-line tools; the Wails 3 build template targets macOS 12+.
- Linux: GTK4, WebKitGTK 6, pkg-config and a C compiler. Ubuntu 24.04 CI installs `libgtk-4-dev libwebkitgtk-6.0-dev libx11-dev libfuse2t64`.

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.33
wails3 dev
wails3 build
```

Go and CLI binaries must be on PATH. Go can download the required toolchain
automatically; the Wails CLI is installed in `$(go env GOPATH)/bin` by default.

A fresh build installs frontend dependencies and regenerates bindings automatically.
Bindings use method names, so they are readable and do not depend on opaque numeric IDs.
The existing React, Ant Design and Vite versions were retained to avoid unrelated UI migrations.
The existing Vite build emits Ant Design `use client` and bundle-size warnings; these do not prevent the build.

Wails beta.33 uses a TCP probe for frontend readiness. The previous Vite
headerless-GET readiness middleware is no longer needed and has been removed.
The Vite listener is explicitly bound to `127.0.0.1`, matching the Wails
development proxy's IPv4 connection. Its port follows `WAILS_VITE_PORT`, so
`wails3 dev -port ...` continues to work.

## Packaging

```sh
node scripts/set-version.mjs 1.4.0
wails3 build VERSION=1.4.0
wails3 package VERSION=1.4.0
wails3 task linux:create:deb VERSION=1.4.0
wails3 task linux:create:appimage VERSION=1.4.0
```

All artifacts now go in the root `bin/` directory. Windows release workflows include `build/books` in the portable ZIP.
macOS DMGs use Wails' packaging tool; Linux DEB/AppImage use its native tasks.
Run packaging on the matching OS. Windows arm64 can be built on Windows amd64 without the old hotkey CGO dependency.
The Linux default stack changes from GTK3/WebKitGTK 4.0 to GTK4/WebKitGTK 6; old Linux distributions need a separate legacy GTK3 build and matching package dependencies.
Wails' global shortcut backend requires platform support; Linux Wayland support depends on its desktop portal, and reserved shortcuts can fail registration.

## Existing data

- No book files are migrated or removed. Windows still uses `books` beneath the working directory; macOS/Linux still use `~/books`.
- localStorage keys retain the `Wd-` prefix. Windows retains the executable name and `http://wails.localhost/` origin.
- macOS/Linux explicitly retain the v2 `wails://wails/` origin. macOS retains the `com.wails.wdreader` bundle identifier.
- Reusing old reading preferences also depends on the OS reusing the same WebView profile. Existing-profile upgrades on macOS/Linux have not been tested on this Windows workstation.
- The conversion server and its current HTTP URL are retained. Remote format conversion availability is outside Wails' control.

## Verification

### beta.33 upgrade (2026-10-20)

- `go mod tidy`, `go test ./...` and `go vet ./...`: passed on Windows amd64.
- `npm test`: all 13 tests passed; `npm run build`: passed with the existing warnings.
- `wails3 version`: v3.0.0-beta.33; binding generation, `wails3 build VERSION=dev` and `wails3 build DEV=true`: passed.
- Windows arm64: `CGO_ENABLED=0 GOARCH=arm64 go build -tags production -trimpath`: passed (compile only).
- beta.29 and beta.33 common Taskfile templates were compared; no build-task changes were required.
- Native UI interaction, full `wails3 dev` startup, macOS/Linux builds and installers were not revalidated during this upgrade.

### Original beta.29 migration

- `go test ./...`: passed, including the available AZW3 extraction fixture; the existing conversion test skips if its expected source file is absent.
- `go vet ./...`: passed.
- `npm run build`: passed.
- `wails3 build VERSION=dev`: passed on Windows amd64.
- `wails3 build DEV=true`: passed.
- `wails3 dev`: reproduced and fixed the readiness 404 and IPv4/IPv6 mismatch; the full dev workflow subsequently launched a native window displaying the existing 38-book shelf.
- Windows arm64 production cross-compilation with `CGO_ENABLED=0`: passed (compile check, not an arm64 runtime test).
- Native Windows UI: book shelf loaded 38 existing books; opening a book, switching from chapter 1 to chapter 2, toggling window size, and second-instance activation worked.
- Further UI automation was stopped by the user with Escape. File picker, OS file dragging, global shortcut dispatch, transparent mode and cross-platform runtime checks remain manual acceptance items.
- Test output for the AZW3 extraction test now uses `t.TempDir()` rather than writing files beside the repository fixture.
- Fixed an existing string/error formatting mismatch in the remote conversion client because it prevented Go test/vet verification.

No remote release was published, and no Git commit was created.
