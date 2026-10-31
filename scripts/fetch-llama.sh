#!/bin/sh
# Puts llama.cpp's llama-server into the app's resources, from llama.cpp's own release, checked
# against its SHA-256: app/resources/<platform>/bin/llama-server and the libraries it loads (from
# beside it), and its license in app/resources/<platform>/licenses/.
#
#	scripts/fetch-llama.sh [darwin-arm64 | windows-amd64]   (default: darwin-arm64)
#
# The release is pinned: the one Tomoru's local translation was measured with. On Windows it is
# the Vulkan build, which runs on any maker's GPU, and on the CPU without one.
set -e
TAG=b11960 # llama.cpp v0.8.0
platform=${1:-darwin-arm64}
case $platform in
darwin-arm64)
	SHA256=58e5a5e1d7c2bf66cedd9c29e0323f3e570660a70a1e2156d32eed08f4cc1e3c
	URL=https://github.com/ggml-org/llama.cpp/releases/download/$TAG/llama-$TAG-bin-macos-arm64.tar.gz ;;
windows-amd64)
	SHA256=bac6914b5e83523eb008c6789eaa12669f93fbc196c3b9e8237b6672323b33e9
	URL=https://github.com/ggml-org/llama.cpp/releases/download/$TAG/llama-$TAG-bin-win-vulkan-x64.zip ;;
*)
	echo "unknown platform $platform" >&2
	exit 2 ;;
esac

cd "$(dirname "$0")/.."
res=app/resources/$platform
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "downloading llama.cpp $TAG for $platform"
curl -fL --progress-bar -o "$tmp/llama" "$URL"
echo "$SHA256  $tmp/llama" | sha256sum --strict -c -
mkdir "$tmp/x"
mkdir -p "$res/bin" "$res/licenses"
# An earlier release's files, which this one may not have (b9840's libomp140.x86_64.dll).
rm -f "$res"/bin/llama-server* "$res"/bin/*llama* "$res"/bin/*ggml* "$res"/bin/*mtmd* "$res"/bin/libomp*

if [ "$platform" = windows-amd64 ]; then
	unzip -q "$tmp/llama" -d "$tmp/x"
	src="$tmp/x"
	# llama-server.exe and the libraries it imports, through those they import (objdump of
	# mingw-w64, which builds the engine too), and ggml's backends, which ggml loads as it starts.
	# Cross-compiling, mingw-w64's objdump is x86_64-w64-mingw32-objdump; in MSYS2, objdump.
	objdump=$(command -v x86_64-w64-mingw32-objdump || command -v objdump) || {
		echo "needs mingw-w64's objdump" >&2
		exit 1
	}
	need="llama-server.exe"
	todo="llama-server.exe"
	while [ -n "$todo" ]; do
		next=""
		for f in $todo; do
			"$objdump" -p "$src/$f" >"$tmp/imports" # apart, for set -e to stop at its failure
			for lib in $(awk '/DLL Name/{print $3}' "$tmp/imports"); do
				[ -f "$src/$lib" ] || continue # the system's
				case " $need " in *" $lib "*) ;; *) need="$need $lib"; next="$next $lib" ;; esac
			done
		done
		todo=$next
	done
	for f in "$src"/ggml-cpu-*.dll "$src"/ggml-vulkan.dll; do
		need="$need $(basename "$f")"
	done
	for f in $need; do
		cp "$src/$f" "$res/bin/"
	done
else
	tar -xzf "$tmp/llama" -C "$tmp/x"
	src="$tmp/x/llama-$TAG"
	# llama-server and the libraries it needs, through those they need.
	need="llama-server"
	todo="llama-server"
	while [ -n "$todo" ]; do
		next=""
		for f in $todo; do
			for lib in $(otool -L "$src/$f" | awk '/@rpath\//{sub("@rpath/", "", $1); print $1}'); do
				case " $need " in *" $lib "*) ;; *) need="$need $lib"; next="$next $lib" ;; esac
			done
		done
		todo=$next
	done
	for f in $need; do
		cp "$src/$f" "$res/bin/"
		xattr -c "$res/bin/$f"
	done
fi
scripts/package-licenses.sh "$platform"
echo "put $(echo $need | wc -w | tr -d ' ') files in $res/bin"
