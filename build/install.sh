#!/bin/sh
# conch installer for Linux. The build step splats the goreleaser
# snapshot version into VERSION so each release ships a version-pinned
# copy of this script next to its packages.
set -eu

VERSION="__CONCH_VERSION__"
BASE="https://github.com/jdarcyryan/conch/releases/download/${VERSION}"

if [ "$(uname -s)" != "Linux" ]; then
    echo "install.sh: Linux only (got $(uname -s))" >&2
    exit 1
fi

if [ "$(uname -m)" = "x86_64" ]; then
    ARCH=amd64
elif [ "$(uname -m)" = "aarch64" ]; then
    ARCH=arm64
else
    echo "install.sh: unsupported architecture $(uname -m)" >&2
    exit 1
fi

if [ ! -r /etc/os-release ]; then
    echo "install.sh: /etc/os-release missing or unreadable" >&2
    exit 1
fi
# Source os-release in a subshell only: the file defines VERSION (and
# other generic names) itself, and sourcing it inline clobbers the
# release version splatted in above — turning the download URL into
# e.g. .../conch_22.04.3 LTS (Jammy Jellyfish)_amd64.deb, which curl
# rejects as a bad/illegal format.
OS_ID=$(. /etc/os-release && printf '%s' "${ID:-}")
OS_ID_LIKE=$(. /etc/os-release && printf '%s' "${ID_LIKE:-}")
FAMILY=" ${OS_ID} ${OS_ID_LIKE} "

# Pick the package extension up front so the download / install lines
# can stay generic. dpkg, rpm, and apk are the lowest-common-denominator
# tools — installed by default on every member of each family — so the
# script does not depend on apt/dnf/yum being present.
if echo "$FAMILY" | grep -qE '(debian|ubuntu)'; then
    EXT=deb
elif echo "$FAMILY" | grep -qE '(fedora|rhel|centos)'; then
    EXT=rpm
elif echo "$FAMILY" | grep -q 'alpine'; then
    EXT=apk
else
    echo "install.sh: unsupported distro ${OS_ID:-unknown}" >&2
    exit 1
fi

SUDO=""
[ "$(id -u)" -eq 0 ] || SUDO=sudo

# Stage the package in a private temp dir and register cleanup against
# every termination path — normal exit, error under `set -e`, or one of
# the common signals. The trap fires before the script process leaves,
# so the downloaded package never lingers on disk even if dpkg/rpm/apk
# rejects it.
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT HUP INT TERM

PKG="conch_${VERSION}_${ARCH}.${EXT}"
PKG_PATH="${TMP_DIR}/${PKG}"
URL="${BASE}/${PKG}"

if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$URL" -o "$PKG_PATH"
elif command -v wget >/dev/null 2>&1; then
    wget -q "$URL" -O "$PKG_PATH"
else
    echo "install.sh: neither curl nor wget is installed" >&2
    exit 1
fi

case "$EXT" in
    deb) $SUDO dpkg -i "$PKG_PATH" ;;
    rpm) $SUDO rpm -i "$PKG_PATH" ;;
    apk) $SUDO apk add --allow-untrusted "$PKG_PATH" ;;
esac
