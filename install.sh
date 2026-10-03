#!/bin/sh
# Installs Moonwell on Linux: downloads the moonwell executable of this script's version from its GitHub release,
# checks it against the release's checksums and puts it in ~/.local/bin. Running it again upgrades.
#
#   curl -fsSL https://github.com/mdlsvensson/moonwell/releases/latest/download/install.sh | sh
set -eu

version="0.9.1"
base="${MOONWELL_INSTALL_BASE:-https://github.com/mdlsvensson/moonwell/releases/download/moonwell@$version}"

fail() {
  echo "error: $1" >&2
  exit 1
}

system="$(uname -s)"
machine="$(uname -m)"
if [ "$system" != "Linux" ] || { [ "$machine" != "x86_64" ] && [ "$machine" != "amd64" ]; }; then
  fail "Moonwell $version has no build for $system on $machine. It is built for Linux and Windows on x86-64."
fi
asset="moonwell-linux-amd64"

if command -v curl >/dev/null 2>&1; then
  download() { curl -fsSL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  download() { wget -q -O "$2" "$1"; }
else
  fail "Installing needs curl or wget."
fi
if command -v sha256sum >/dev/null 2>&1; then
  checksum() { sha256sum "$1" | cut -d ' ' -f 1; }
elif command -v shasum >/dev/null 2>&1; then
  checksum() { shasum -a 256 "$1" | cut -d ' ' -f 1; }
else
  fail "Installing needs sha256sum or shasum, to check the download."
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
download "$base/$asset" "$work/$asset" || fail "Downloading $base/$asset failed."
download "$base/checksums.txt" "$work/checksums.txt" || fail "Downloading $base/checksums.txt failed."

expected="$(awk -v name="$asset" '$2 == name || $2 == "*" name { print $1 }' "$work/checksums.txt")"
[ -n "$expected" ] || fail "checksums.txt of Moonwell $version does not list $asset."
actual="$(checksum "$work/$asset")"
if [ "$actual" != "$expected" ]; then
  fail "The download does not match its checksum (expected $expected, got $actual). Nothing was installed."
fi

bin="$HOME/.local/bin"
mkdir -p "$bin"
chmod +x "$work/$asset"
# Moved into place in one step, so a moonwell that is running keeps its file until it ends.
mv -f "$work/$asset" "$bin/moonwell"
echo "Installed Moonwell $version to $bin/moonwell."

case ":$PATH:" in
  *":$bin:"*) ;;
  *)
    echo "$bin is not on your PATH. Add it once, then open a new terminal:"
    echo "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.profile"
    ;;
esac
