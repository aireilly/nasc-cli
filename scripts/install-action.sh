#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 Aidan Reilly and the nasc contributors
#
# Installs a nasc release binary for the GitHub Action. Verifies the archive
# against the release checksums.txt before adding nasc to PATH.
set -euo pipefail

repo="aireilly/nasc-cli"
version="${NASC_VERSION:-}"
if [ -z "$version" ]; then
  # Default to the release that matches the action's tag, so a Dependabot or
  # Renovate bump of `uses:` also bumps the binary. A branch or SHA ref falls
  # back to the latest release.
  case "${NASC_ACTION_REF:-}" in
    v[0-9]*.*.*) version="$NASC_ACTION_REF" ;;
    *) version=latest ;;
  esac
fi
auth=()
if [ -n "${GH_TOKEN:-}" ]; then
  auth=(-H "Authorization: Bearer ${GH_TOKEN}")
fi

if [ "$version" = "latest" ]; then
  # github.com redirects /releases/latest to /releases/tag/<tag>.
  url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${repo}/releases/latest" || true)
  version="${url##*/tag/}"
  [ "$version" = "$url" ] && version=""
  if [ -z "$version" ]; then
    # Fall back to the REST API, which needs the token on busy runners.
    version=$(curl -fsSL ${auth[@]+"${auth[@]}"} "https://api.github.com/repos/${repo}/releases/latest" |
      sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1 || true)
  fi
  if [ -z "$version" ]; then
    echo "::error title=nasc::Could not resolve the latest nasc release."
    exit 1
  fi
  echo "::warning title=nasc::Using the latest nasc release (${version}). Set 'version: ${version}' to pin it."
fi
case "$version" in v*) ;; *) version="v${version}" ;; esac
v="${version#v}"

case "${RUNNER_OS:-$(uname -s)}" in
  Linux) os=linux ;;
  macOS | Darwin) os=darwin ;;
  Windows) os=windows ;;
  *) echo "::error title=nasc::Unsupported runner OS ${RUNNER_OS:-}"; exit 1 ;;
esac
case "${RUNNER_ARCH:-$(uname -m)}" in
  X64 | x86_64) arch=amd64 ;;
  ARM64 | arm64 | aarch64) arch=arm64 ;;
  *) echo "::error title=nasc::Unsupported runner architecture ${RUNNER_ARCH:-}"; exit 1 ;;
esac

base="https://github.com/${repo}/releases/download/${version}"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -fsSL ${auth[@]+"${auth[@]}"} -o "$tmp/checksums.txt" "$base/checksums.txt"

archive=""
# Releases before v0.2.0 used the goreleaser default name.
for name in "nasc_${v}_${os}_${arch}.tar.gz" "nasc-cli_${v}_${os}_${arch}.tar.gz"; do
  if grep -q " ${name}\$" "$tmp/checksums.txt"; then
    archive="$name"
    break
  fi
done
if [ -z "$archive" ]; then
  echo "::error title=nasc::No ${os}/${arch} archive in nasc ${version}."
  exit 1
fi
curl -fsSL ${auth[@]+"${auth[@]}"} -o "$tmp/$archive" "$base/$archive"

if command -v sha256sum >/dev/null; then
  (cd "$tmp" && grep " ${archive}\$" checksums.txt | sha256sum -c - >/dev/null)
else
  (cd "$tmp" && grep " ${archive}\$" checksums.txt | shasum -a 256 -c - >/dev/null)
fi

bin="nasc"
[ "$os" = windows ] && bin="nasc.exe"
dest="${RUNNER_TOOL_CACHE:-$HOME/.cache}/nasc/${v}/${arch}"
mkdir -p "$dest"
tar -xzf "$tmp/$archive" -C "$dest" "$bin"
echo "$dest" >>"${GITHUB_PATH:-/dev/null}"
echo "Installed nasc ${version} to ${dest}"
