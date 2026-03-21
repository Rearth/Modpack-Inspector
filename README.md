## Preface

> This project was an experiment of mine to see how far modern AI coding tools can go. 99% of the code has been created
by AI-Models (mostly claude opus 4.6 and chatgpt 5.4). I am not a frontend developer, so I don't understand half of it.
Use this tool at your own risk. Based on my testing, it does what's expected, but I'm not promising anything. I spent about
2 full days on this for the initial version, with multiple iterations, improvements and lots of input and guidance to the AI, but the code itself is written by the AI. 

> This does not mean my minecraft mods will be vibe coded from now on, I actually enjoy writing the java code.

## Overview

Modpack Inspector is a desktop application for inspecting and debugging Minecraft modpacks.

Download the latest release via: https://github.com/Rearth/Modpack-Inspector/releases

It scans the mods in a selected instance, extracts metadata and dependency information from each JAR, enriches that data with Modrinth and CurseForge when available, and presents the result in a searchable desktop UI built with Wails.

The goal is to make modpack maintenance faster: you can see what is installed, how mods depend on each other, which libraries are unused, which config files likely belong to which mods, and where unresolved dependencies are coming from.

<img width="1397" height="899" alt="Screenshot 2026-03-21 175156" src="https://github.com/user-attachments/assets/8b5bdbef-e353-45ad-8dfe-8a549900c726" />
<img width="1395" height="896" alt="Screenshot 2026-03-21 175050" src="https://github.com/user-attachments/assets/a14da78e-5fee-486b-b582-bd838ebce5f3" />
<img width="1396" height="895" alt="Screenshot 2026-03-21 175121" src="https://github.com/user-attachments/assets/993f4159-3b81-49b6-a5f0-792d2b58731c" />


## What The Tool Does

### Mod Scanning

Modpack Inspector reads mod metadata directly from common loader manifests inside JAR files, including:

- `fabric.mod.json`
- `quilt.mod.json`
- `mods.toml`
- `neoforge.mods.toml`

From those manifests it extracts IDs, names, versions, descriptions, authors, provided module IDs, and declared dependencies.

### Online Enrichment

After local scanning, the app attempts to enrich mods with online metadata:

- CurseForge lookup by MurmurHash2 fingerprint
- Modrinth lookup by JAR SHA-1 hash

When those lookups succeed, Modpack Inspector can add:

- project links
- icons
- categories
- longer descriptions
- additional dependency hints from hosted project/file metadata

### Dependency Resolution

Dependencies are resolved against the currently loaded mod list and against modules provided by other JARs.

That lets the app distinguish between:

- dependencies satisfied by another installed mod
- dependencies satisfied by a provided or embedded module
- dependencies that are still unresolved
- unresolved online-only dependencies that could not be matched locally

### Library Detection

The app also computes a `detected library` flag used by the Mods view, the dependency graph, and the unused-library report.

This flag is heuristic, not authoritative metadata from the mod host.

The current logic is:

- if online metadata marks a mod only as `library` or `api and library`, it is treated as a detected library
- if those library tags are mixed with other tags, the app uses embedding similarity on the mod description to decide whether it looks more like shared code or more like a content mod
- if the model is unavailable, the app falls back to a very narrow shared-code description heuristic

This is intentionally conservative because many gameplay mods also carry `API and Library` categories even when they are not actually shared-core dependencies.

### Mixin Inspection

Modpack Inspector extracts mixin information directly from mod JARs by parsing mixin config JSON files and the Java class bytecode of each mixin class.

For each mod, the detail view shows:

- **Outgoing mixins**: classes this mod injects into other mods, grouped by target mod
- **Incoming mixins**: classes other mods inject into this mod, grouped by source mod

Each mixin entry shows the target class and shadowed/overwritten members (methods and fields). Clicking a target or source mod navigates to that mod's detail view.

Mixin data is resolved across the full mod list so cross-mod relationships are visible even when the target mod does not declare the dependency explicitly.

### Search And Exploration

The UI includes a smart search that combines local mod metadata and embedding-based search support when the ONNX model is available.

You can use the app to:

- search mods by name, description, categories, and dependency context
- inspect a mod's dependency list and reverse dependencies
- open a live dependency graph
- identify unused library mods
- inspect unresolved Modrinth or CurseForge dependencies from the mod detail view

### Config Mapping And Editing

The app scans the instance `config` folder and tries to match config files to mods using heuristics and confidence scoring.

You can:

- review suggested config mappings
- override mappings manually
- open configs in the built-in Monaco editor
- save config changes directly from the app

### Live File Watching

After an instance is selected, Modpack Inspector watches the mod and config folders.

That means:

- new or changed mods are rescanned automatically
- removed mods are dropped from the database
- config mappings are recalculated after mod changes
- a manual rescan is usually only needed if you want to force a full refresh

## Tech Stack

### Desktop Shell

- Wails v2.11
- Go backend
- native desktop window with Wails runtime bindings

### Frontend

- React 18
- TypeScript 5
- Vite 5
- Tailwind CSS 4
- Monaco Editor
- `react-force-graph-2d` for the dependency graph
- `@untitled-ui/icons-react` and `lucide-react` for UI icons

### Backend And Data

- Go 1.25
- SQLite via `modernc.org/sqlite`
- filesystem watching via `fsnotify`
- TOML + JSON manifest parsing

### Search / ML

- ONNX Runtime via `github.com/yalue/onnxruntime_go`
- `all-MiniLM-L6-v2` sentence embedding model
- text-only fallback behavior when the model is unavailable

## User Guide

### Starting The App

1. Launch Modpack Inspector.
2. On first start, choose a Minecraft instance from the detected list, or browse to one manually.
3. Wait for the initial scan to finish.
4. Use the left navigation to switch between Mods, Graph, and Settings.

### Choosing An Instance

An instance should point at a Minecraft profile folder that contains at least:

- a `mods` folder
- usually a `config` folder

If your launcher is not auto-detected, open Settings or the first-run wizard and add custom launcher roots.

### Browsing Mods

In the Mods view you can:

- search for mods with the smart search bar
- filter by category
- filter by the `Detected Library` pill directly from mod cards
- inspect detected libraries and unused libraries
- click a mod card to open its details

Inside the mod detail view you can:

- inspect dependencies and reverse dependencies
- view outgoing and incoming mixins with target classes and member details
- open linked config files
- view unresolved online dependency warnings when local resolution fails

### Using The Graph

The Graph view shows dependency relationships between mods.

You can use it to:

- spot missing dependencies
- inspect optional edges
- identify clusters of related mods
- click a node to jump back to that mod in the Mods view

### Editing Configs

When a config file is linked to a mod, you can open it in the built-in editor.

The editor supports:

- syntax-aware Monaco editing for common config formats
- direct save from the UI
- hover access to the full resolved file path

### Rescanning

Modpack Inspector already watches file changes.

If you add, remove, or replace a mod while the app is open, it should update automatically.

Use the Scan button when you want to force a full rescan of the whole pack.

## Developer Guide

### Prerequisites

You need:

- Go 1.25+
- Node.js with npm
- Wails CLI v2
- a working C toolchain, because `onnxruntime_go` uses CGO

Platform notes:

- Windows: MinGW-w64 / GCC or another working Windows C toolchain
- Linux: GTK 3 and WebKitGTK development packages
- macOS: Xcode Command Line Tools (`xcode-select --install`)

Example Linux packages:

```bash
sudo apt-get update
sudo apt-get install -y build-essential libgtk-3-dev libwebkit2gtk-4.0-dev
```

Example macOS bootstrap:

```bash
xcode-select --install
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Known working setup in this workspace:

- `CGO_ENABLED=1`
- MinGW-w64 / GCC from WinLibs
- `CC` pointing at the GCC executable

Example Windows PowerShell setup:

```powershell
$env:CGO_ENABLED = "1"
$env:CC = "C:\Users\Darkp\AppData\Local\Microsoft\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64\bin\gcc.exe"
$env:PATH = "C:\Users\Darkp\AppData\Local\Microsoft\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64\bin;" + $env:PATH
```

Install the Wails CLI if needed:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

On Linux and macOS, the default compiler on your PATH is usually enough:

```bash
export CGO_ENABLED=1
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

### Install Dependencies

From the project root:

```bash
cd modpacktool
npm --prefix frontend install
```

Or let Wails install the frontend dependencies automatically on first run.

### Running In Development

Standard development flow:

```bash
cd modpacktool
wails dev
```

That starts:

- the Go backend
- the Vite dev server
- the desktop app shell

On Linux, `wails dev` still depends on the GTK/WebKit packages above.

On macOS, Wails uses the system toolchain from Xcode Command Line Tools.

### Browser Live Preview

During `wails dev`, Wails exposes a browser-accessible preview, typically at:

```text
http://localhost:34115
```

That is useful when you want to inspect the frontend quickly in a browser while still having access to Wails-bound methods.

### Useful Commands

Run tests:

```bash
go test ./...
```

Build the frontend only:

```bash
cd frontend
npm run build
```

Build the desktop app:

```bash
wails build
```

### Project Layout

High-level structure:

- `main.go` — Wails app bootstrap and window config
- `app.go` — Wails-bound backend methods, scanning, enrichment, watcher behavior
- `internal/db` — SQLite models and queries
- `internal/scanner` — JAR scanning, config scanning, watcher implementation
- `internal/api` — Modrinth and CurseForge clients
- `internal/resolver` — dependency resolution and graph construction
- `internal/embeddings` — ONNX embedding engine and search support
- `frontend/src/components` — desktop UI components
- `frontend/src/hooks` — frontend data hooks

### Notes For Contributors

- The first run may download ONNX Runtime and the embedding model into the user data directory.
- The app should still function when embeddings are unavailable; search falls back gracefully.
- Wails hot reload can sometimes transiently restart the app while the database is still initializing.
- If you change backend methods exposed to the frontend, make sure the generated Wails bindings stay in sync.

## Production Build

Create a redistributable desktop build with:

```powershell
wails build
```

The packaged output is written under the Wails build output directory, typically in `build/bin`.
