#!/bin/sh
# Bootstrap only. The executable owns all installation and service changes.
set -eu

if [ "${1:-}" = "--binary" ]; then
    if [ "$#" -lt 2 ]; then
        echo "usage: scripts/install.sh --binary /absolute/path/outpost [install flags]" >&2
        exit 2
    fi
    outpost_binary=$2
    shift 2
    case "$outpost_binary" in
        /*) ;;
        *) echo "--binary must be an absolute path" >&2; exit 2 ;;
    esac
    if [ ! -x "$outpost_binary" ]; then
        echo "Outpost binary is not executable: $outpost_binary" >&2
        exit 1
    fi
    exec "$outpost_binary" install "$@"
fi

outpost_release=latest
if [ "${1:-}" = "--release" ]; then
    if [ "$#" -lt 2 ]; then
        echo "usage: scripts/install.sh --release TAG [install flags]" >&2
        exit 2
    fi
    case "$2" in
        v[0-9]* ) ;;
        *) echo "--release must be a version tag" >&2; exit 2 ;;
    esac
    case "$2" in
        *[!A-Za-z0-9._-]* ) echo "--release contains unsupported characters" >&2; exit 2 ;;
    esac
    outpost_release=$2
    shift 2
fi

if [ "$(uname -s)" != Linux ]; then
    echo "Release bootstrap currently supports Linux" >&2
    exit 1
fi
case "$(uname -m)" in
    x86_64) outpost_arch=amd64 ;;
    aarch64) outpost_arch=arm64 ;;
    *) echo "Unsupported architecture; build from source and use --binary" >&2; exit 1 ;;
esac
for command_name in curl sha256sum mktemp; do
    if ! command -v "$command_name" >/dev/null 2>&1; then
        echo "Install $command_name, then rerun this bootstrap" >&2
        exit 1
    fi
done

outpost_asset="outpost-linux-$outpost_arch"
if [ "$outpost_release" = latest ]; then
    outpost_url="https://github.com/felixlheureux/outpost/releases/latest/download/$outpost_asset"
else
    outpost_url="https://github.com/felixlheureux/outpost/releases/download/$outpost_release/$outpost_asset"
fi
outpost_tmp=$(mktemp -d)
trap 'rm -rf "$outpost_tmp"' EXIT HUP INT TERM
if ! curl -fsSL --retry 2 "$outpost_url" -o "$outpost_tmp/$outpost_asset"; then
    echo "No supported release asset was found. Build locally and use scripts/install.sh --binary /absolute/path/outpost." >&2
    exit 1
fi
if ! curl -fsSL --retry 2 "$outpost_url.sha256" -o "$outpost_tmp/$outpost_asset.sha256"; then
    echo "The release checksum is unavailable; installation stopped" >&2
    exit 1
fi
(cd "$outpost_tmp" && sha256sum -c "$outpost_asset.sha256")
chmod 0700 "$outpost_tmp/$outpost_asset"
"$outpost_tmp/$outpost_asset" install "$@"
