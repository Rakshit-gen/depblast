#!/usr/bin/env bash
# Packs a release archive: depblast, HydraDB's graph-node, and the shared
# libraries graph-node loads, rewritten to load from the archive itself so
# nothing has to be installed on the user's machine.
#
#   scripts/bundle.sh VERSION HYDRA_VERSION GRAPH_NODE OUT_DIR
#
# Builds depblast for the current OS and CPU and writes
# OUT_DIR/depblast-VERSION-OS-ARCH.tar.gz. Needs install_name_tool and
# codesign on macOS, patchelf on Linux.
set -euo pipefail

version=$1 hydra_version=$2 graph_node=$3 out=$4
os=$(go env GOOS) arch=$(go env GOARCH)
name=depblast-$version-$os-$arch
stage=$(mktemp -d)/depblast
mkdir -p "$stage/bin" "$stage/libexec" "$stage/lib" "$out"

go build -trimpath -ldflags "-s -w -X main.buildVersion=$version -X main.hydraVersion=$hydra_version" \
  -o "$stage/bin/depblast" ./cmd/depblast
cp "$graph_node" "$stage/libexec/graph-node"
chmod u+w "$stage/libexec/graph-node"

case $os in
darwin)
  # Every library outside the OS, following each library's own
  # dependencies too (GraphBLAS needs libomp).
  deps() { otool -L "$1" | tail -n +2 | awk '{print $1}' | grep -vE '^(/usr/lib/|/System/|@)' || true; }
  todo=("$stage/libexec/graph-node")
  while ((${#todo[@]})); do
    file=${todo[0]} todo=("${todo[@]:1}")
    for lib in $(deps "$file"); do
      base=$(basename "$lib")
      if [[ ! -e $stage/lib/$base ]]; then
        cp -L "$lib" "$stage/lib/$base"
        chmod u+w "$stage/lib/$base"
        install_name_tool -id "@rpath/$base" "$stage/lib/$base"
        todo+=("$stage/lib/$base")
      fi
      install_name_tool -change "$lib" "@rpath/$base" "$file"
    done
  done
  install_name_tool -add_rpath @executable_path/../lib "$stage/libexec/graph-node"
  for lib in "$stage"/lib/*; do install_name_tool -add_rpath @loader_path "$lib" 2>/dev/null || true; done
  # Editing a binary breaks its signature, and Apple Silicon refuses to
  # run unsigned code, so sign everything again (ad hoc).
  codesign --force --sign - "$stage"/lib/* "$stage/libexec/graph-node"
  ;;
linux)
  # Everything ldd finds except glibc itself, which every distribution has.
  ldd "$stage/libexec/graph-node" | awk '/=> \//{print $3}' |
    grep -vE '/(libc|libm|libdl|libpthread|librt|libgcc_s|ld-linux[^/]*)\.so' |
    while read -r lib; do cp -L "$lib" "$stage/lib/"; done
  patchelf --set-rpath '$ORIGIN/../lib' "$stage/libexec/graph-node"
  for lib in "$stage"/lib/*; do patchelf --set-rpath '$ORIGIN' "$lib"; done
  ;;
*)
  echo "unsupported OS $os" >&2
  exit 1
  ;;
esac

cp LICENSE "$stage/LICENSE"
cat > "$stage/HYDRADB.txt" <<NOTE
libexec/graph-node is HydraDB $hydra_version, built unmodified from
https://github.com/hydra-db/hydradb/tree/$hydra_version
HydraDB is licensed under the GNU Affero General Public License v3.0; its
source and license are at that address. The libraries in lib/ are the ones
graph-node was linked against (SuiteSparse:GraphBLAS and its dependencies),
each under its own license.
NOTE

tar -C "$(dirname "$stage")" -czf "$out/$name.tar.gz" depblast
(cd "$out" && shasum -a 256 "$name.tar.gz" > "$name.tar.gz.sha256")
echo "$out/$name.tar.gz"
