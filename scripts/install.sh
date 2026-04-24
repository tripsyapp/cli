#!/usr/bin/env bash
# install.sh - Install the Tripsy CLI and MCP server.
#
# Usage:
#   curl -fsSL https://tripsy.app/install_cli | bash
#
# Options via environment:
#   TRIPSY_BIN_DIR   Where to install the binary. Default: ~/.local/bin
#   TRIPSY_VERSION   Specific version to install, without "v". Default: latest
#   TRIPSY_REPO      GitHub repo to install from. Default: tripsyapp/cli

set -euo pipefail

REPO="${TRIPSY_REPO:-tripsyapp/cli}"
BIN_DIR="${TRIPSY_BIN_DIR:-$HOME/.local/bin}"
VERSION="${TRIPSY_VERSION:-}"

if [[ -z "${NO_COLOR:-}" ]] && [[ -t 1 ]]; then
  bold() { printf '\033[1m%s\033[0m' "$1"; }
  green() { printf '\033[32m%s\033[0m' "$1"; }
  red() { printf '\033[31m%s\033[0m' "$1"; }
else
  bold() { printf '%s' "$1"; }
  green() { printf '%s' "$1"; }
  red() { printf '%s' "$1"; }
fi

info() { echo " [OK] $1"; }
step() { echo " => $1"; }
error() {
  echo " $(red "ERROR:") $1" >&2
  exit 1
}

find_sha256_cmd() {
  if command -v sha256sum >/dev/null 2>&1; then
    echo "sha256sum"
  elif command -v shasum >/dev/null 2>&1; then
    echo "shasum -a 256"
  else
    error "No SHA256 tool found. Install sha256sum or shasum."
  fi
}

detect_platform() {
  local os arch

  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$os" in
    darwin) os="darwin" ;;
    linux) os="linux" ;;
    freebsd) os="freebsd" ;;
    openbsd) os="openbsd" ;;
    mingw*|msys*|cygwin*) os="windows" ;;
    *) error "Unsupported OS: $os" ;;
  esac

  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) error "Unsupported architecture: $arch" ;;
  esac

  echo "${os}_${arch}"
}

get_latest_version() {
  local url version

  url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest" 2>/dev/null) || true
  version="${url##*/}"
  version="${version#v}"

  if [[ ! $version =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
    error "Could not determine latest version from GitHub releases. Resolved '${version:-}' from '${url:-}'."
  fi

  echo "$version"
}

verify_checksum() {
  local version="$1"
  local tmp_dir="$2"
  local archive_name="$3"
  local base_url="https://github.com/${REPO}/releases/download/v${version}"
  local expected actual

  step "Verifying checksum..."
  curl -fsSL "${base_url}/checksums.txt" -o "${tmp_dir}/checksums.txt" \
    || error "Failed to download checksums.txt"

  expected=$(awk -v f="$archive_name" '$2 == f || $2 == ("*" f) {print $1; exit}' "${tmp_dir}/checksums.txt")
  actual=$(cd "$tmp_dir" && $(find_sha256_cmd) "$archive_name" | awk '{print $1}')

  [[ -n "$expected" && "$expected" == "$actual" ]] \
    || error "Checksum verification failed for $archive_name"

  info "Checksum verified"
}

download_binary() {
  local version="$1"
  local platform="$2"
  local tmp_dir="$3"
  local ext archive_name url binary_name mcp_binary_name

  if [[ "$platform" == windows_* ]]; then
    ext="zip"
    binary_name="tripsy.exe"
    mcp_binary_name="tripsy-mcp.exe"
  else
    ext="tar.gz"
    binary_name="tripsy"
    mcp_binary_name="tripsy-mcp"
  fi

  archive_name="tripsy_${version}_${platform}.${ext}"
  url="https://github.com/${REPO}/releases/download/v${version}/${archive_name}"

  step "Downloading tripsy v${version} for ${platform}..."
  curl -fsSL "$url" -o "${tmp_dir}/${archive_name}" \
    || error "Failed to download $url"

  verify_checksum "$version" "$tmp_dir" "$archive_name"

  step "Extracting..."
  if [[ "$ext" == "zip" ]]; then
    command -v unzip >/dev/null 2>&1 || error "unzip is required to install Windows archives"
    unzip -q "${tmp_dir}/${archive_name}" -d "$tmp_dir"
  else
    tar -xzf "${tmp_dir}/${archive_name}" -C "$tmp_dir"
  fi

  mkdir -p "$BIN_DIR"
  [[ -f "${tmp_dir}/${binary_name}" ]] || error "Binary not found in archive: ${binary_name}"

  for name in "$binary_name" "$mcp_binary_name"; do
    if [[ ! -f "${tmp_dir}/${name}" ]]; then
      info "${name} not included in this release; skipping"
      continue
    fi
    mv "${tmp_dir}/${name}" "$BIN_DIR/"
    chmod +x "$BIN_DIR/$name"
    info "Installed ${name} to $BIN_DIR/$name"
  done
}

setup_path() {
  if [[ ":$PATH:" == *":$BIN_DIR:"* ]]; then
    return 0
  fi

  step "Adding $BIN_DIR to PATH"

  local shell_rc
  case "${SHELL:-}" in
    */zsh) shell_rc="$HOME/.zshrc" ;;
    */bash) shell_rc="$HOME/.bashrc" ;;
    *) shell_rc="$HOME/.profile" ;;
  esac

  local path_line="export PATH=\"$BIN_DIR:\$PATH\""
  if [[ -f "$shell_rc" ]] && grep -qF "$BIN_DIR" "$shell_rc" 2>/dev/null; then
    info "PATH already configured in $shell_rc"
  else
    {
      echo ""
      echo "# Added by Tripsy CLI installer"
      echo "$path_line"
    } >> "$shell_rc"
    info "Added to $shell_rc"
    info "Run: source $shell_rc"
  fi
}

verify_install() {
  local platform="$1"
  local binary_name="tripsy"
  local mcp_binary_name="tripsy-mcp"
  local installed_version

  if [[ "$platform" == windows_* ]]; then
    binary_name="tripsy.exe"
    mcp_binary_name="tripsy-mcp.exe"
  fi

  installed_version=$("$BIN_DIR/$binary_name" --version 2>/dev/null) \
    || error "Installation failed: tripsy did not run"

  info "$(green "$installed_version")"

  if [[ -x "$BIN_DIR/$mcp_binary_name" ]]; then
    installed_version=$("$BIN_DIR/$mcp_binary_name" --version 2>/dev/null) \
      || error "Installation failed: tripsy-mcp did not run"
    info "$(green "$installed_version")"
  fi
}

main() {
  echo ""
  echo "$(bold "Tripsy CLI installer")"
  echo ""

  command -v curl >/dev/null 2>&1 || error "curl is required but not installed"

  local platform version tmp_dir
  platform=$(detect_platform)

  if [[ -n "$VERSION" ]]; then
    version="$VERSION"
    [[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] \
      || error "Invalid version '${version}'. Expected 1.2.3 or 1.2.3-rc.1."
  else
    version=$(get_latest_version)
  fi

  tmp_dir=$(mktemp -d)
  trap "rm -rf '${tmp_dir}'" EXIT

  download_binary "$version" "$platform" "$tmp_dir"
  setup_path
  verify_install "$platform"

  echo ""
  echo "Next steps:"
  echo "  $(bold "tripsy auth login --username you@example.com")"
  echo "  $(bold "tripsy doctor")"
  echo "  $(bold "tripsy-mcp --version")"
  echo ""
}

main "$@"
