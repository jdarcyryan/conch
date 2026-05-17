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

. /etc/os-release
FAMILY=" ${ID:-} ${ID_LIKE:-} "

SUDO=""
[ "$(id -u)" -eq 0 ] || SUDO=sudo

if echo "$FAMILY" | grep -qE '(debian|ubuntu)'; then
    PKG="conch_${VERSION}_${ARCH}.deb"
    curl -fsSL "${BASE}/${PKG}" -o "/tmp/${PKG}"
    $SUDO dpkg -i "/tmp/${PKG}"
elif echo "$FAMILY" | grep -qE '(fedora|rhel|centos)'; then
    PKG="conch_${VERSION}_${ARCH}.rpm"
    curl -fsSL "${BASE}/${PKG}" -o "/tmp/${PKG}"
    $SUDO dnf install -y "/tmp/${PKG}"
elif echo "$FAMILY" | grep -q 'alpine'; then
    PKG="conch_${VERSION}_${ARCH}.apk"
    curl -fsSL "${BASE}/${PKG}" -o "/tmp/${PKG}"
    $SUDO apk add --allow-untrusted "/tmp/${PKG}"
else
    echo "install.sh: unsupported distro ${ID:-unknown}" >&2
    exit 1
fi
