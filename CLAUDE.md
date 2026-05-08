# CLAUDE.md

Project context for Claude Code working on **conch**.

## Project overview

Conch is a declarative, cross-platform PowerShell environment manager written in Go. It lets a user describe a PowerShell environment — the PowerShell version itself plus any number of PowerShell modules — in a single TOML manifest in the root of the project, named conch.toml, and then materialises that environment locally in a reproducible, isolated way.

Think of it as `pixi` but specifically for PowerShell:

- Pixi is a Rust-based package and environment manager built on top of the Conda ecosystem. It uses a TOML manifest to describe a project's dependencies, tasks, and target platforms, and produces a lockfile so the same environment can be reconstructed on any machine.
- Conch borrows the same shape of solution but its "packages" are the **portable PowerShell distribution** and **PowerShell modules**.
- The user authors a manifest, runs a single command, and ends up with a self-contained PowerShell environment they can activate, run tasks against, and share via the lockfile.

The name to use throughout the codebase, package metadata, manifests, and user-facing strings is **`conch`** (lowercase).

## Pixi-style ergonomics

The CLI surface and manifest concepts should feel familiar to anyone who has used pixi, cargo, or npm:

- **A single declarative manifest** in TOML, checked into source control.
- **A lockfile** pinning resolved versions and integrity hashes.
- **Per-project isolated environments** — each project gets its own PowerShell installation and module path, separate from any system PowerShell, in a local folder called .conch.
- **Automatic gitignore** - once the .conch environment folder is created, if the gitignore does not contain this reference, it is added by default.
- **Tasks and aliases** defined in the manifest, runnable inside the environment.

When designing the CLI surface or manifest schema, the default heuristic is: *what would pixi do?* Diverge only where PowerShell-specific concerns require it.

## Manifest schema is examples-driven

The manifest is TOML, living in conch.toml at the root of the project. **Do not invent or assume field names.** Example manifests live in `examples/` and are the source of truth for the schema. Parsers, validators, and tests must be written against those files, and any new manifest features need a corresponding example added there first.

A few semantic rules the parser must enforce regardless of exact field names:

- **`*` and `latest` are valid version specifiers** for both the PowerShell version and any module version. Both resolve to the newest available version at install time.
- **Environments target all supported OS/arch combinations by default.** A manifest with no platform constraints must work on every OS/arch conch supports. Override sections in the manifest narrow the set, e.g. restricting to a specific OS, a specific architecture, or a specific OS/arch pair. The lockfile records the resolved set per platform.

## Target platforms and architectures

Defined by `goreleaser.yaml`. Conch must build, test, and run cleanly on:

| OS      | Architectures |
| ------- | ------------- |
| windows | amd64, arm64  |
| linux   | amd64, arm64  |

When mapping Go's `runtime.GOOS` / `runtime.GOARCH` to PowerShell's release artefact naming:

- `windows` → `win`; `linux` stays `linux`.
- `amd64` → `x64`, `arm64` → `arm64`.
- Any OS/arch combination outside this matrix should produce a clean fatal error, not a panic.

## Caching and integrity

Conch maintains a single shared cache in a well-known per-user location (e.g. under the user's cache directory) for:

- **PowerShell portable archives** (`.zip` / `.tar.gz`).
- **Module packages** (`.nupkg`).

The install pipeline must always check the cache before reaching for the network:

1. If a file with the expected name is already present in the cache, hash it and compare against the expected SHA-256.
2. If the hash matches, use the cached file.
3. If the file is missing or the hash does not match, download it, verify the hash, and write it to the cache.

This makes installs cheap and deterministic across projects, and means a corrupted or tampered cache file is automatically replaced rather than trusted.

## Environment isolation

A conch environment must shadow the host's global PowerShell module setup completely. Inside an activated environment:

- Only modules installed by conch into that environment are discoverable.
- Modules installed system-wide or in the user's profile must not be importable.
- `PSModulePath` (and any related lookup variables) is rewritten to point exclusively at the environment's module directory.

This is non-negotiable — reproducibility is the whole point. A script that runs in one user's conch environment must behave identically in another user's, regardless of what either of them has installed globally.

## Repository layout and Go conventions

Conch follows idiomatic Go layout. When adding code:

- **Small, focused packages.** One clear responsibility per package. Names are short, lowercase, no underscores or camelCase, and meaningful in isolation.
- **`cmd/`** for binaries — one subdirectory per binary, each `main` thin and doing nothing but wiring flags and dispatching.
- **`internal/`** is preferred over `pkg/` for everything that is not a deliberately public API.
- **No god packages.** Split files when concerns drift apart.
- **Avoid premature abstraction.** Interfaces are defined by the consumer, not the producer. No interface until there are at least two implementations or a real testing seam needs one.
- **Errors are values.** Wrap with `fmt.Errorf("doing X: %w", err)` and let the CLI entry point decide how to present them. `log.Fatalf` is acceptable in `main`, not in library code.
- **Standard library first.** Justified dependencies: a CLI framework, a TOML parser, anything else only when stdlib genuinely falls short.
- **Tests live next to code** (`foo.go` + `foo_test.go`). Table-driven by default.
- **Formatting is non-negotiable.** Code passes `gofmt` and `go vet` cleanly. `golangci-lint` is the target linter.

## Prior art: the `psenv` prototype

A previous prototype called `psenv` exists. It is **not** the source of truth, but it solved several concrete problems conch needs to solve again:

- Detecting OS/arch and mapping to PowerShell's release naming scheme.
- Building the download URL for a given version, OS, and architecture.
- Fetching the official hashes file, decoding it (it is published as **UTF-16 LE with BOM** — naive `string(body)` produces garbage), and locating the line for a given filename.
- Resolving `latest` / `*` to a concrete version.

When porting these ideas, do not copy `log.Fatalf` calls into library packages — convert them to wrapped errors. The prototype was a flat `package main`; conch splits this functionality across `internal/` packages.

## Build and development

- Local build: `make build` runs `go run ./build`, orchestrating `go-winres` + GoReleaser snapshot. Output lands in `.output/`.
- The native binary for the current host is copied to `.output/conch` (or `.output/conch.exe`) as a convenience.
- `.syso` files are generated and cleaned by the build script — never committed.
- Snapshot version is pinned at `0.1.0` until real tags exist.

Day-to-day: `go build ./...`, `go test ./...`, `go vet ./...`, `gofmt -w .`. `go mod tidy` runs as part of the build hook.

## What Claude should do

- Treat `examples/` as the manifest contract. Write parsers and tests against those files; do not invent field names.
- Prefer small, focused changes that fit the layout. If a change has no obvious home, propose a new package rather than dumping into `main`.
- Mirror pixi's naming and ergonomics where it makes sense — users coming from pixi should not be surprised.
- Always go through the cache for downloads, with hash verification on both cached and freshly fetched files.
- Preserve environment isolation rigorously; any change that could let host-installed modules leak in is a bug.
- Do not embed remote URLs (release endpoints, hash files, module gallery, etc.) in code or docs unprompted — those go into a dedicated configuration layer later.
- Fail loudly on unsupported OS/arch combinations with useful messages.