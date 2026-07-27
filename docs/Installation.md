# Installation

Conch ships prebuilt packages for Windows and Linux on amd64 and arm64. Every release also attaches portable archives (`.zip` / `.tar.gz`) and a `checksums.txt` — see the [releases page](https://github.com/jdarcyryan/conch/releases) for a specific version.

## Linux

The install script detects your distribution and architecture, downloads the matching package (`deb`, `rpm`, or `apk`) from the latest release, and installs it:

```sh
curl -fsSL https://github.com/jdarcyryan/conch/releases/latest/download/install.sh | sudo sh
```

Supported families: Debian/Ubuntu (`deb`), Fedora/RHEL/CentOS (`rpm`), and Alpine (`apk`), on amd64 and arm64.

## Windows

Install from the Chocolatey community repository:

```powershell
choco install conch --source https://community.chocolatey.org/api/v2
```

To pin a specific version, add `--version <version>` — each version matches a [GitHub release](https://github.com/jdarcyryan/conch/releases) tag.

## Verify

```sh
conch --version
```
