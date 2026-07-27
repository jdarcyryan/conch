# Usage

Conch materialises a reproducible, isolated PowerShell environment from a single TOML manifest — `conch.toml` — at the root of your project.

## Quick start

```sh
conch init       # write a starter conch.toml in the current directory
conch install    # resolve, download, and extract everything declared in conch.toml
conch shell      # launch an interactive PowerShell session inside the environment
```

`conch install` reads the manifest, materialises the PowerShell version and modules into the project-local `.conch/` directory, and pins every resolved version and hash in `conch.lock`. It also adds `.conch/` to your `.gitignore` if it is not already listed.

## Commands

| Command | Description |
| ------- | ----------- |
| `conch init` | Write a starter `conch.toml` in the current directory |
| `conch install` | Resolve, download, and extract everything declared in `conch.toml` |
| `conch run TASK [args...]` | Run a task defined in `conch.toml` inside the project's environment |
| `conch shell` | Launch an interactive PowerShell session inside the project's environment |
| `conch summary` | Print a summary of the project's manifest |
| `conch tasks` | Show every task defined in `conch.toml` with its full body |

`conch run` and `conch shell` auto-install before handing control to `pwsh`, so a fresh checkout is one command away from a working environment.

`conch summary` and `conch tasks` accept `--format json|yaml|toml|xml` for machine-readable output, and every command accepts `--min-ui` for plain, non-interactive output.

## Manifest examples

The [`examples/`](../examples) folder is the source of truth for the manifest schema, from a [minimal manifest](../examples/01-minimal.toml) up to a [full-featured one](../examples/08-full.toml) covering project metadata, platform restrictions, version specifiers (exact pins, ranges, wildcards, `latest`/`*`), tasks, and PowerShell preferences.

## Environment variable overrides

Module resolution and download default to the public PowerShell Gallery. Two optional environment variables redirect conch to an alternative NuGet v2 feed:

| Variable | Description |
| -------- | ----------- |
| `CONCH_NUGET_URL` | Base URL of a NuGet v2 feed to use instead of the PSGallery, e.g. `https://nuget.example.com/api/v2` |
| `CONCH_NUGET_API_KEY` | API key sent (as `X-NuGet-ApiKey`) with requests to the feed named by `CONCH_NUGET_URL` |

If `CONCH_NUGET_URL` is not set, conch falls back to the PSGallery and `CONCH_NUGET_API_KEY` is ignored — the key is never sent to the public gallery.

```sh
export CONCH_NUGET_URL="https://nuget.example.com/api/v2"
export CONCH_NUGET_API_KEY="<your-api-key>"
conch install
```
