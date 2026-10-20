<div align="center">

<img src="build/appicon.png" width="88" alt="wd-reader icon" />

# wd-reader

[简体中文](README.md) | **English**

**A quiet moment to read.**

A lightweight, distraction-free desktop ebook reader for Windows, macOS and Linux. Keep a local library, read in a mini window, use a transparent background, or let pages turn and scroll automatically.

[![Release](https://img.shields.io/github/v/release/lbk-ones/wd-reader?style=flat-square)](https://github.com/lbk-ones/wd-reader/releases/latest)
[![Downloads](https://img.shields.io/github/downloads/lbk-ones/wd-reader/total?style=flat-square)](https://github.com/lbk-ones/wd-reader/releases)
![Platforms](https://img.shields.io/badge/platform-Windows%20%7C%20macOS%20%7C%20Linux-456653?style=flat-square)
![Wails](https://img.shields.io/badge/Wails-v3.0.0--beta.33-DF0000?style=flat-square)

[Download](https://github.com/lbk-ones/wd-reader/releases) · [Screenshots](#screenshots) · [Getting started](#getting-started) · [Development and builds](#development-and-builds) · [Report an issue](https://github.com/lbk-ones/wd-reader/issues)

</div>

![wd-reader grid library with title search, layout controls and local books](images/screenshots/bookshelf-grid.jpg)

## Features

| Feature | What you can do |
| --- | --- |
| Local library | Add books or drop files into the window, switch between List and Grid, search titles or pinyin initials, and return to recent books |
| Text layout | Choose Compact, Standard, Comfort or Large presets, or adjust text size and line height individually |
| Reading colors | Choose Paper, Warm, Green or Night, or customize background and text colors |
| Flexible window | Keep the reader on top, use transparency, hide the title bar, hide content when the pointer leaves, or read in a 1–4 line mini window |
| Quick hiding | Minimize or restore the window with `Ctrl / ⌘ + Space` |
| Chapter navigation | Detect chapters, search contents, move between chapters, or split text by paragraph count or a custom regular expression |
| Auto reading | Turn pages on a timer or scroll continuously; advance at chapter end and pause while using settings and other panels |
| Reading history | Save chapters, reading positions and preferences automatically and continue next time |
| Everyday controls | Click to turn pages, scroll with the wheel, use the context menu and keyboard shortcuts, or open built-in help |
| Chinese and English | Follow the system language on first launch and save your choice of Simplified Chinese or English |

### Interface language

Open **App settings → Language** from the bottom-right corner of the library, or use the language selector beside the **Reading preferences** heading while reading. Select **简体中文** or **English**. Changes apply immediately and persist across restarts.

On first launch, English systems use English; other system languages default to Simplified Chinese. Book titles, chapter names, text and custom window titles keep their original language. Clearing reading history preserves the language preference; clearing all cache restores the first-launch language rules. See the [internationalization notes](docs/i18n.md) for implementation details (in Chinese).

## Download and installation

Visit **[GitHub Releases](https://github.com/lbk-ones/wd-reader/releases)** and select the asset for your operating system and processor. Available packages depend on the release.

| System | Architecture | Package and setup |
| --- | --- | --- |
| Windows | x64 / ARM64 | Download and fully extract the matching `.zip`, then run `wdreader.exe`. Keep the included `books` folder |
| macOS | Intel / Apple Silicon | Open the matching `.dmg` and drag the app into Applications |
| Linux | x64 | Install the `.deb`, or make the `.AppImage` executable and run it |

In filenames, `amd64` means x64 / Intel; `arm64` means Windows ARM or Apple Silicon.

Windows requires the WebView2 Runtime. Linux uses GTK4 / WebKitGTK 6, and AppImage also needs the corresponding system runtime dependencies. macOS builds target version 12.0 or later; the current packaging tasks use ad-hoc signing without Developer ID notarization.

## Screenshots

Some screenshots show earlier layouts and Chinese interface text. The current home screen uses a compact toolbar, and the grid fits three columns in the default 400px window, adapting to narrower windows.

### Default window size

These screenshots use the default **400 × 690** window. The library adapts to the available width, text wraps to fit, and preferences scroll within their panel.

| List library | Reading | Preferences |
| :---: | :---: | :---: |
| <img src="images/screenshots/compact-bookshelf-list.jpg" width="240" alt="List library in the default 400×690 window" /> | <img src="images/screenshots/compact-reading.jpg" width="240" alt="Warm reading colors in the default window" /> | <img src="images/screenshots/compact-preferences.jpg" width="240" alt="Scrollable reading preferences in the default window" /> |

<details>
<summary><strong>Grid library and contents in a small window</strong></summary>

| Earlier grid layout | Chapter contents |
| :---: | :---: |
| <img src="images/screenshots/compact-bookshelf-grid.jpg" width="280" alt="Earlier two-column grid layout in a 400×690 window" /> | <img src="images/screenshots/compact-catalog.jpg" width="280" alt="Chapter contents beside the text in a small window" /> |

</details>

### Library and reading

Use List for quick scanning or Grid for browsing. Both layouts support title and pinyin-initial searches, and your layout choice is saved.

<details>
<summary><strong>List library: importing, search and everyday controls</strong></summary>

![List library with import actions, search, layout switching, help and app settings](images/screenshots/bookshelf-list.jpg)

</details>

The Warm palette pairs a centered chapter heading with a bottom bar showing the chapter title, number and overall chapter progress. Click the current title to open contents.

![Warm reading view with chapter heading, navigation and progress](images/screenshots/reading-warm.jpg)

<details>
<summary><strong>Night palette: dark background and light text</strong></summary>

![Night reading palette and chapter navigation](images/screenshots/reading-night.jpg)

</details>

### Contents and chapter detection

Search chapter titles, jump to a chapter and see the current position in the contents panel. Smart detection filters empty chapters and merges duplicate content while preserving chapters with the same title but different text.

![Searchable chapter contents with the current chapter highlighted](images/screenshots/chapter-catalog.jpg)

<details>
<summary><strong>Custom chapter detection for each book</strong></summary>

If automatic detection does not work well for a book, split by paragraph count or enter a regular expression. Applying the rule prompts you to clear the book’s saved chapter position.

![Custom chapter regular expression and reading preferences](images/screenshots/chapter-settings.jpg)

</details>

### Reading preferences

The preferences panel groups the most-used controls:

- **Text and colors:** four text-size / line-height presets, four palettes and custom colors.
- **Reading and window:** click to turn, progress, always on top, auto hide, transparency, title bar and mini-mode line count.
- **Auto reading:** page intervals of 1–3600 seconds or scrolling at 5–100 pixels per second. Enabling either mode disables the other.
- **Chapter detection:** smart detection, paragraph counts or a custom regular expression.

![Reading preferences for typography, colors, window behavior, auto reading and chapter detection](images/screenshots/reading-preferences.jpg)

<details>
<summary><strong>Built-in help and keyboard shortcuts</strong></summary>

Press `F1` to open help. Browse Getting started, Reading preferences and Shortcuts. The instructions are available offline.

![Built-in help explaining F1–F9, Esc, Enter and the boss key](images/screenshots/keyboard-guide.jpg)

</details>

## Getting started

1. Click **Add books** or drop local files into the library window. You can also enter a direct file URL under **Download books**.
2. Double-click a book, or focus it with the keyboard and press `Enter`.
3. Click the text to turn pages, scroll with the mouse wheel, use the bottom bar to change chapters, or click the chapter title to open contents.
4. Open **Reading preferences** with the gear button to adjust colors, text, window behavior and auto reading.

### Supported formats

| Format | Processing | Notes |
| --- | --- | --- |
| TXT | Local | Supports common Chinese encodings and UTF-16 text with a byte-order mark; chapter detection is configurable |
| EPUB | Local | Extracts text for reading, with a focus on text layout |
| MOBI / AZW / AZW3 | Local | Extracts text; encrypted or incompatible files may not open, and large files can take longer |
| DOC / DOCX / PDF / HTML / HTMLZ | Online conversion | Depends on the existing conversion service and its format support. This path uploads files, and the current service uses HTTP |

TXT and EPUB are recommended for everyday reading. This reader focuses on text; conversion of PDF and other formats may not preserve the original page layout.

### Mini mode, transparency and hiding

- **Mini reader:** choose 1–4 lines in preferences and enable Mini mode. Drag the six-dot handle to move the window; press `Esc` or click the expand icon to exit.
- **Mini context menu:** right-click the text to open all actions in a separate always-on-top window. Page and chapter navigation keep mini mode; contents, preferences and other panels restore the normal window. Press `Esc` or focus another window to dismiss the menu.
- **Transparent mode:** use a transparent background and white text over other windows. Choosing a color preset turns transparency off.
- **Auto hide:** hide reading content when the pointer leaves the window and reveal it when the pointer returns.
- **Hidden title bar:** right-click the text and choose Title bar to restore it, or Preferences to reopen settings.

Auto page turn defaults to 30 seconds; auto scroll defaults to 20 pixels per second. Both pause in settings, contents or help, while loading books, or when content is hidden. Manual activity restarts the page-turn timer and briefly pauses scrolling.

### Keyboard shortcuts

| Key | Action |
| --- | --- |
| `F1` | Open help |
| `F2` | Toggle contents |
| `F3` / `F4` | Previous / next page |
| `F5` / `F6` | Previous / next chapter |
| `F7` | Toggle auto page turn |
| `F8` | Toggle mini mode |
| `F9` | Toggle auto scroll |
| `Esc` | Close help; exit mini mode while reading |
| `Enter` | Open the focused book in the library |
| `Ctrl + Space` / `⌘ + Space` | Boss key: minimize / restore the window |

`F2`–`F9` mainly apply while reading and pause when editing inputs or using other panels. The boss key requires successful system shortcut registration and may conflict with other apps. Linux support depends on the desktop environment.

### Where are books and preferences stored?

| Data | Location / behavior |
| --- | --- |
| Windows books | `books/` in the working directory at startup; launching from the repository root uses its `books/` folder |
| macOS / Linux books | `~/books/` |
| Reading positions and preferences | Local WebView localStorage, using the `Wd-` prefix |

**Clear reading history** removes recent books, saved chapters and reading positions while preserving typography, colors, language and library layout. **Clear all cache** also removes preferences. When moving to another device, book files and WebView reading data must be handled separately.

## Development and builds

Built with **Go + Wails 3 + React + Ant Design + Vite**. Keep the Wails Go module, CLI and frontend runtime aligned at `v3.0.0-beta.33`.

### Requirements

- Go **1.26+**.
- Node.js **24 LTS** and npm.
- The `wails3` CLI, with Go’s binary directory on `PATH`.
- Windows: WebView2 Runtime.
- macOS: Xcode Command Line Tools.
- Linux: a C compiler, pkg-config, GTK4, WebKitGTK 6 and other native dependencies. See the [Linux CI workflow](.github/workflows/release-linux.yaml).

### Run locally

```sh
git clone https://github.com/lbk-ones/wd-reader.git
cd wd-reader
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.33
wails3 dev
```

The first run installs frontend dependencies and generates Go bindings. Run the following commands from the repository root:

```sh
# Build a production binary for the current system
wails3 build

# Specify the architecture and application version
wails3 build ARCH=arm64 VERSION=2.0.0
```

Output goes to `bin/`: `wdreader.exe` on Windows and `wdreader` on macOS / Linux.

### Package a release

Package on the matching operating system. First update version metadata:

```sh
node scripts/set-version.mjs 2.0.0
```

```sh
# macOS: create .app and .dmg; additional files use name=path
wails3 task darwin:package:dmg ARCH=arm64 VERSION=2.0.0 DMG_FILES="fix-app=build/dmg/fix-app,fix-app_zh=build/dmg/fix-app_zh"

# Linux: create a DEB or AppImage
wails3 task linux:create:deb VERSION=2.0.0
wails3 task linux:create:appimage VERSION=2.0.0
```

See the [Windows workflow](.github/workflows/release-windows.yaml) for portable ZIP assembly and the [macOS workflow](.github/workflows/release-macos.yaml) for the complete DMG process. All three platform workflows can run when a GitHub Release is published or be triggered manually with a version tag.

### Checks and tests

```sh
go test ./...
go vet ./...
npm --prefix frontend ci
npm --prefix frontend test
npm --prefix frontend run build
```

See the [Wails 3 migration notes](docs/wails3-migration.md) for framework migration, platform compatibility and validation, and the [changelog](changelog.md) for changes (in Chinese).

## Feedback and contributions

Report bugs and suggestions through [Issues](https://github.com/lbk-ones/wd-reader/issues). Include your operating system, processor architecture, application version, reproduction steps and relevant logs. For parsing issues, provide a minimal sample without private content if possible.

Before submitting code, run the checks relevant to your change. Include screenshots for interface changes and reproducible samples for reading or parsing changes.

Thanks to [Wails](https://github.com/wailsapp/wails), [React](https://github.com/facebook/react), [Ant Design](https://github.com/ant-design/ant-design) and the other open-source dependencies used by this project.
