#!/usr/bin/env bash
# Print the Homebrew formula fields (version / url / sha256) for a tagged
# release. Lightweight on purpose: no goreleaser `brews:` automation, no
# token wiring — you paste the 3 fields into the formula in the separate
# tap repo. See docs/releasing-homebrew.md.
set -euo pipefail

version="${1:-}"
if [[ -z "${version}" ]]; then
  echo "Usage: $0 <version>   (e.g. $0 0.1.0)" >&2
  exit 1
fi
version="${version#v}" # accept v0.1.0 or 0.1.0

repo="thomasmarcelin754/boursocli"
url="https://github.com/${repo}/archive/refs/tags/v${version}.tar.gz"
tmp="$(mktemp -t boursocli-XXXXXX).tar.gz"
trap 'rm -f "${tmp}"' EXIT

curl -fL -o "${tmp}" "${url}" >/dev/null
sha256="$(shasum -a 256 "${tmp}" | awk '{print $1}')"

cat <<EOF
version "${version}"
url "${url}"
sha256 "${sha256}"
EOF
