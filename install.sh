#!/bin/sh
# Installs depblast and the HydraDB server it runs:
#
#   curl -fsSL https://raw.githubusercontent.com/Rakshit-gen/depblast/main/install.sh | sh
#
# Puts everything in ~/.depblast (or $DEPBLAST_HOME) and links
# ~/.local/bin/depblast to it. Running it again upgrades in place and keeps
# your indexed data. Set DEPBLAST_VERSION=v0.1.0 to install a given
# release instead of the latest.
set -eu

repo=Rakshit-gen/depblast
home=${DEPBLAST_HOME:-$HOME/.depblast}
bindir=${DEPBLAST_BIN_DIR:-$HOME/.local/bin}
version=${DEPBLAST_VERSION:-latest}

fail() {
	echo "depblast install: $*" >&2
	exit 1
}

case $(uname -s) in
Darwin) os=darwin ;;
Linux) os=linux ;;
*) fail "$(uname -s) is not supported (macOS and Linux are)" ;;
esac
case $(uname -m) in
arm64 | aarch64) arch=arm64 ;;
x86_64 | amd64) arch=amd64 ;;
*) fail "$(uname -m) CPUs are not supported" ;;
esac
if [ "$os-$arch" = darwin-amd64 ]; then
	fail "Intel Macs are not supported yet, only Apple Silicon"
fi

if [ "$version" = latest ]; then
	base=https://github.com/$repo/releases/latest/download
else
	base=https://github.com/$repo/releases/download/$version
fi
base=${DEPBLAST_DOWNLOAD_BASE:-$base}
archive=depblast-$os-$arch.tar.gz

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
echo "downloading $archive ($version)"
curl -fSL --progress-bar "$base/$archive" -o "$tmp/$archive" || fail "download failed: $base/$archive"
curl -fsSL "$base/$archive.sha256" -o "$tmp/$archive.sha256" || fail "checksum download failed"

want=$(awk '{print $1}' "$tmp/$archive.sha256")
if command -v sha256sum >/dev/null 2>&1; then
	got=$(sha256sum "$tmp/$archive" | awk '{print $1}')
else
	got=$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')
fi
[ "$want" = "$got" ] || fail "checksum mismatch: expected $want, got $got"

tar -C "$tmp" -xzf "$tmp/$archive"

# An older depblast may have its HydraDB running from the files about to
# be replaced; stop it first. The next command starts the new one.
if [ -x "$home/bin/depblast" ]; then
	DEPBLAST_HOME=$home "$home/bin/depblast" server stop >/dev/null 2>&1 || true
fi
mkdir -p "$home"
for d in bin libexec lib; do
	rm -rf "${home:?}/$d"
	mv "$tmp/depblast/$d" "$home/$d"
done
cp "$tmp/depblast/LICENSE" "$tmp/depblast/HYDRADB.txt" "$home/"

mkdir -p "$bindir"
ln -sf "$home/bin/depblast" "$bindir/depblast"
echo "installed $("$home/bin/depblast" version)"
echo "  $bindir/depblast -> $home/bin/depblast"

case ":$PATH:" in
*":$bindir:"*) ;;
*)
	echo
	echo "$bindir is not on your PATH. Add this line to your shell profile:"
	echo "  export PATH=\"$bindir:\$PATH\""
	;;
esac
echo
echo "try: depblast index ~/code && depblast audit"
