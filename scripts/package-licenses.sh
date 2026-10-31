#!/bin/sh
# Keep checked-in notices with the resources that MyGo copies into a release.
# This never downloads a model, binary, or license.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

check_sources() {
    (cd third_party/licenses && sha256sum --strict -c ../SHA256SUMS >/dev/null)
    sha256sum --strict -c third_party/DEPENDENCIES.sha256 >/dev/null
    expected=$(awk 'NR > 1 { print $1 }' third_party/SOURCES.tsv | LC_ALL=C sort)
    actual=$(find third_party/licenses -type f -exec basename {} \; | LC_ALL=C sort)
    hashed=$(awk '{ print $2 }' third_party/SHA256SUMS | LC_ALL=C sort)
    if [ "$expected" != "$actual" ] || [ "$expected" != "$hashed" ]; then
        echo "Third-party notice files, SOURCES.tsv and SHA256SUMS disagree." >&2
        exit 1
    fi
}

usage() {
    echo "Usage: scripts/package-licenses.sh --check | [--release-check] <darwin-arm64|windows-amd64>" >&2
    exit 2
}

mode=copy
case ${1:-} in
    --check) [ "$#" -eq 1 ] || usage; check_sources; echo "Third-party source notices verified."; exit 0 ;;
    --release-check) mode=release; shift ;;
esac
[ "$#" -eq 1 ] || usage
platform=$1
case $platform in darwin-arm64|windows-amd64) ;; *) usage ;; esac
check_sources
dest="$root/app/resources/$platform/licenses"

if [ "$mode" = copy ]; then
    mkdir -p "$dest"
    # Update only managed notices. Unknown files are reported by the release check.
    for file in "$root"/third_party/licenses/*; do
        cp "$file" "$dest/"
    done
    cp LICENSE "$dest/Tomoru-GPL-3.0.txt"
    cp third_party/SOURCES.tsv third_party/SHA256SUMS third_party/REVIEW.md third_party/REVIEW_zh.md third_party/README.md third_party/README_zh.md "$dest/"
    echo "Packaged third-party notices in $dest"
    echo "Before publishing, run scripts/package-licenses.sh --release-check $platform (see third_party/REVIEW.md)." >&2
    exit 0
fi

[ -d "$dest" ] || { echo "Missing $dest; run scripts/package-licenses.sh $platform." >&2; exit 1; }
(cd "$dest" && sha256sum --strict -c "$root/third_party/SHA256SUMS" >/dev/null)
cmp LICENSE "$dest/Tomoru-GPL-3.0.txt"
for file in SOURCES.tsv SHA256SUMS REVIEW.md REVIEW_zh.md README.md README_zh.md; do cmp "third_party/$file" "$dest/$file"; done
expected=$( { awk '{ print $2 }' third_party/SHA256SUMS; printf '%s\n' Tomoru-GPL-3.0.txt SOURCES.tsv SHA256SUMS REVIEW.md REVIEW_zh.md README.md README_zh.md; } | LC_ALL=C sort)
actual=$(find "$dest" -type f -exec basename {} \; | LC_ALL=C sort)
[ "$expected" = "$actual" ] || { echo "Unexpected files in $dest; review stale notices before publishing." >&2; exit 1; }

blocked=0
if grep -Eq '^replace github.com/egoist/mygo => \.\./' app/go.mod; then
    echo "Release blocked: publish the MyGo replacement source and use a public pinned dependency (third_party/REVIEW.md)." >&2
    blocked=1
fi
if [ "$platform" = windows-amd64 ] && [ -f "$root/app/resources/$platform/bin/libomp140.x86_64.dll" ]; then
    echo "Release blocked: the pinned upstream libomp140.x86_64.dll comes from Visual Studio debug_nonredist; see third_party/REVIEW.md." >&2
    blocked=1
fi
if [ "$blocked" -ne 0 ]; then exit 1; fi
echo "Packaged notices match their sources; complete the remaining provenance review in third_party/REVIEW.md."
