# Wails 3 build assets

`Taskfile.yml` in the repository root dispatches to the desktop tasks in this directory.
Application metadata lives in `build/config.yml`; Wails v2's `wails.json` is no longer used.

- `wails3 dev`: build the development binary, start Vite and watch Go sources.
- `wails3 build`: build a production binary in the root `bin/` directory.
- `wails3 package`: build the native installer/package on the matching OS.
- `wails3 task linux:create:deb`: build a DEB.
- `wails3 task linux:create:appimage`: build an AppImage.
- `wails3 package ARCH=arm64`: select the target architecture.

Use `VERSION=1.4.0` on build/package commands to set the runtime version.
For release metadata, first run `node scripts/set-version.mjs 1.4.0`.
This regenerates Windows resources, macOS plist files and Linux package metadata from `config.yml`.
Regeneration also creates ignored iOS metadata; this project only configures desktop builds.

`build/appicon.png` remains the source for the application icons.
The desktop tasks originated from the Wails v3.0.0-beta.29 template and are verified with v3.0.0-beta.33.
See `docs/wails3-migration.md` for platform requirements and test coverage.
