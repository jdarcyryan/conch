![conch](assets/conch_text_2160x540.png)

<div align="center">

[![release](https://img.shields.io/github/v/release/jdarcyryan/conch?style=flat&logo=github&color=2ea44f)](https://github.com/jdarcyryan/conch/releases)
[![release date](https://img.shields.io/github/release-date/jdarcyryan/conch?style=flat&logo=github)](https://github.com/jdarcyryan/conch/releases)
![go version](https://img.shields.io/github/go-mod/go-version/jdarcyryan/conch?style=flat&logo=go&logoColor=white&color=00ADD8)

[![stars](https://img.shields.io/github/stars/jdarcyryan/conch?style=flat&logo=github&color=6f42c1)](https://github.com/jdarcyryan/conch/stargazers)
[![forks](https://img.shields.io/github/forks/jdarcyryan/conch?style=flat&logo=github&color=6f42c1)](https://github.com/jdarcyryan/conch/network/members)
[![contributors](https://img.shields.io/github/contributors/jdarcyryan/conch?style=flat&logo=github&color=6f42c1)](https://github.com/jdarcyryan/conch/graphs/contributors)
[![issues](https://img.shields.io/github/issues/jdarcyryan/conch?style=flat&logo=github&color=6f42c1)](https://github.com/jdarcyryan/conch/issues)
[![license](https://img.shields.io/badge/license-Apache--2.0-6f42c1?style=flat&logo=github)](https://github.com/jdarcyryan/conch/blob/main/LICENSE)

![windows](https://img.shields.io/badge/windows-0078D6?style=flat)
![linux](https://img.shields.io/badge/-FCC624?style=flat&logo=linux&logoColor=black)
![arm](https://img.shields.io/badge/-0091BD?style=flat&logo=arm&logoColor=white)
![intel](https://img.shields.io/badge/-0071C5?style=flat&logo=intel&logoColor=white)
![amd](https://img.shields.io/badge/-ED1C24?style=flat&logo=amd&logoColor=white)
![compatible](https://img.shields.io/badge/compatible-yes-green)

![choco](https://img.shields.io/badge/choco-80B5E3?style=flat&logo=chocolatey&logoColor=white)
![rpm](https://img.shields.io/badge/rpm-EE0000?style=flat&logo=redhat&logoColor=white)
![deb](https://img.shields.io/badge/deb-A81D33?style=flat&logo=debian&logoColor=white)
![apk](https://img.shields.io/badge/apk-0D597F?style=flat&logo=alpinelinux&logoColor=white)

</div>

## Documentation
- [Installation](docs/Installation.md) — install conch on Linux or Windows.
- [Usage](docs/Usage.md) — commands, manifest examples, and environment variable overrides.

## Mission
Conch is a declarative, cross-platform PowerShell environment manager. Describe your PowerShell version and modules in a single TOML manifest, run one command, and get a reproducible, isolated environment — pixi-style ergonomics, purpose-built for PowerShell.

Solving the age-old "it works on my computer" problem, one manifest at a time.

## How it works
Every project gets a manifest at its root — `conch.toml` — that names the PowerShell version, the modules, and any tasks the project needs. `conch install` reads that manifest, decides
what (if anything) is missing from the project's local `.conch/` directory, resolves any new versions against GitHub releases (for PowerShell) or the PSGallery v2 OData feed (for
modules), and materialises the result. The same pipeline gates `conch run TASK` and `conch shell` — both auto-install before they hand control over to `pwsh`, so a fresh checkout is one
command away from a working environment.

The integrity story is the part worth understanding. Downloads pass through a per-user content-addressable cache that is **never trusted as the source of truth**: every cache lookup
needs an externally-supplied SHA-256 to compare against. On a fresh install conch fetches the upstream manifest (`hashes.sha256` from the PowerShell release for the binary, the OData
feed for each module's SHA-512) and verifies the downloaded bytes against that. Once verified, the SHA-256 is recorded in `conch.lock` so subsequent installs can use the lockfile as the
trust anchor and short-circuit the upstream lookup entirely. A bumped version in `conch.toml` makes the lockfile spec-mismatch its manifest, which forces a re-resolve; a tampered cache
file fails its hash check and is replaced.

Once the artefacts are in place, `activate.ps1` is regenerated from the manifest's `[powershell]` and `[preferences]` sections — tasks are deliberately *not* baked in, so editing
`[tasks]` takes effect on the next `conch run` without a re-install. The script anchors itself on `$PSScriptRoot` so the whole project directory is portable, isolates `PSModulePath` to
`.conch/modules` plus the bundled engine modules (without which basic cmdlets like `Add-Member` would be unreachable), and tidies up its scratch variables on exit.

![conch](assets/conch-workflow.png)
