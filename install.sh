#!/bin/sh
# AgentManager installer — https://github.com/kevinelliott/agentmanager
#
#   curl -fsSL https://kevinelliott.github.io/agentmanager/install.sh | sh
#
# Downloads the release archive for this OS/architecture, verifies its SHA-256
# against the release's checksums.txt, and installs the `agentmgr` binary.
#
# Options (flags, or the environment variable in brackets):
#   --version <tag>   install a specific release, e.g. v1.6.0   [AGENTMGR_VERSION]
#   --dir <path>      install directory                         [AGENTMGR_INSTALL_DIR]
#   --help            show this help
#
# Pass flags through the pipe with `sh -s --`:
#   curl -fsSL https://kevinelliott.github.io/agentmanager/install.sh | sh -s -- --version v1.6.0
#
# Without --dir, the binary goes to /usr/local/bin when that is writable and to
# ~/.local/bin otherwise. The script never calls sudo.

set -eu

REPO="kevinelliott/agentmanager"
BINARY="agentmgr"
PROJECT="agentmanager"
GITHUB="https://github.com"

say() { printf '%s\n' "$*"; }
info() { printf '%s==>%s %s\n' "$BOLD" "$RESET" "$*"; }
warn() { printf '%swarning:%s %s\n' "$YELLOW" "$RESET" "$*" >&2; }
die() {
	printf '%serror:%s %s\n' "$RED" "$RESET" "$*" >&2
	exit 1
}

usage() {
	cat <<'USAGE'
AgentManager installer

  curl -fsSL https://kevinelliott.github.io/agentmanager/install.sh | sh

Options (flags, or the environment variable in brackets):
  --version <tag>   install a specific release, e.g. v1.6.0   [AGENTMGR_VERSION]
  --dir <path>      install directory                         [AGENTMGR_INSTALL_DIR]
  --help            show this help

Pass flags through the pipe with `sh -s --`:
  curl -fsSL https://kevinelliott.github.io/agentmanager/install.sh | sh -s -- --version v1.6.0

Without --dir, the binary goes to /usr/local/bin when that is writable and to
~/.local/bin otherwise. The script never calls sudo.
USAGE
}

setup_colors() {
	if [ -t 2 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-dumb}" != "dumb" ]; then
		BOLD=$(printf '\033[1m') RED=$(printf '\033[31m') YELLOW=$(printf '\033[33m')
		GREEN=$(printf '\033[32m') RESET=$(printf '\033[0m')
	else
		BOLD="" RED="" YELLOW="" GREEN="" RESET=""
	fi
}

have() { command -v "$1" >/dev/null 2>&1; }

# download <url> <destination>
download() {
	if have curl; then
		curl -fsSL --proto '=https' --tlsv1.2 --retry 3 -o "$2" "$1"
	elif have wget; then
		wget -q --https-only -O "$2" "$1"
	else
		die "curl or wget is required"
	fi
}

# Resolve the newest release tag by following the /releases/latest redirect.
# This avoids the GitHub API and its unauthenticated rate limit.
latest_version() {
	url="$GITHUB/$REPO/releases/latest"
	if have curl; then
		final=$(curl -fsSLI --proto '=https' --tlsv1.2 -o /dev/null -w '%{url_effective}' "$url") || return 1
	else
		final=$(wget -q --https-only --max-redirect=5 --server-response --spider "$url" 2>&1 |
			sed -n 's/^ *[Ll]ocation: *//p' | tail -n 1 | tr -d '\r') || return 1
	fi
	tag=${final##*/}
	case "$tag" in
	v[0-9]*) printf '%s' "$tag" ;;
	*) return 1 ;;
	esac
}

detect_os() {
	case "$(uname -s)" in
	Darwin) printf 'darwin' ;;
	Linux) printf 'linux' ;;
	MINGW* | MSYS* | CYGWIN*)
		die "this script does not support Windows shells; download the .zip from $GITHUB/$REPO/releases/latest"
		;;
	*) die "unsupported operating system: $(uname -s)" ;;
	esac
}

detect_arch() {
	case "$(uname -m)" in
	x86_64 | amd64) printf 'amd64' ;;
	arm64 | aarch64) printf 'arm64' ;;
	*) die "unsupported architecture: $(uname -m)" ;;
	esac
}

sha256_of() {
	if have sha256sum; then
		sha256sum "$1" | cut -d ' ' -f 1
	elif have shasum; then
		shasum -a 256 "$1" | cut -d ' ' -f 1
	elif have openssl; then
		openssl dgst -sha256 "$1" | sed 's/^.*= *//'
	else
		die "sha256sum, shasum, or openssl is required to verify the download"
	fi
}

# Pick the install directory: an explicit choice, else /usr/local/bin when
# writable, else ~/.local/bin.
choose_dir() {
	if [ -n "$INSTALL_DIR" ]; then
		printf '%s' "$INSTALL_DIR"
	elif [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
		printf '/usr/local/bin'
	else
		[ -n "${HOME:-}" ] || die "HOME is not set; pass --dir <path>"
		printf '%s/.local/bin' "$HOME"
	fi
}

main() {
	setup_colors

	VERSION="${AGENTMGR_VERSION:-}"
	INSTALL_DIR="${AGENTMGR_INSTALL_DIR:-}"

	while [ $# -gt 0 ]; do
		case "$1" in
		--version)
			[ $# -ge 2 ] || die "--version needs a value"
			VERSION="$2"
			shift 2
			;;
		--version=*)
			VERSION="${1#*=}"
			shift
			;;
		--dir)
			[ $# -ge 2 ] || die "--dir needs a value"
			INSTALL_DIR="$2"
			shift 2
			;;
		--dir=*)
			INSTALL_DIR="${1#*=}"
			shift
			;;
		-h | --help)
			usage
			exit 0
			;;
		*) die "unknown option: $1 (try --help)" ;;
		esac
	done

	have tar || die "tar is required"
	os=$(detect_os)
	arch=$(detect_arch)

	if [ -z "$VERSION" ]; then
		VERSION=$(latest_version) ||
			die "could not determine the latest release; pass --version <tag> (see $GITHUB/$REPO/releases)"
	fi
	case "$VERSION" in
	v*) ;;
	*) VERSION="v$VERSION" ;;
	esac
	number=${VERSION#v}

	archive="${PROJECT}_${number}_${os}_${arch}.tar.gz"
	base="$GITHUB/$REPO/releases/download/$VERSION"

	tmp=$(mktemp -d 2>/dev/null || mktemp -d -t agentmgr)
	trap 'rm -rf "$tmp"' EXIT
	trap 'exit 130' INT
	trap 'exit 143' TERM

	info "Downloading $BINARY $VERSION for $os/$arch"
	download "$base/$archive" "$tmp/$archive" ||
		die "no release archive for $os/$arch at $base/$archive
       See $GITHUB/$REPO/releases for available builds, or install with:
         go install github.com/$REPO/cmd/agentmgr@latest"
	download "$base/checksums.txt" "$tmp/checksums.txt" ||
		die "could not download checksums.txt for $VERSION"

	info "Verifying checksum"
	want=$(awk -v f="$archive" '$2 == f || $2 == "*" f { print $1; exit }' "$tmp/checksums.txt")
	[ -n "$want" ] || die "$archive is not listed in checksums.txt"
	got=$(sha256_of "$tmp/$archive")
	[ "$want" = "$got" ] || die "checksum mismatch for $archive
       expected $want
       got      $got"

	tar -xzf "$tmp/$archive" -C "$tmp" "$BINARY" ||
		die "could not extract $BINARY from $archive"
	chmod 755 "$tmp/$BINARY"

	# Run the new binary before installing it, so a build that cannot start on
	# this machine never replaces a working one.
	"$tmp/$BINARY" version >/dev/null 2>&1 ||
		die "the downloaded $BINARY does not run on this system ($os/$arch)"

	dir=$(choose_dir)
	mkdir -p "$dir" 2>/dev/null || die "cannot create $dir; pass --dir <path>"
	[ -w "$dir" ] || die "$dir is not writable; pass --dir <path> or re-run with access to it"

	# Stage next to the destination and rename, so the swap is atomic and an
	# agentmgr that is currently running is not truncated mid-write.
	staged="$dir/.$BINARY.$$"
	cp "$tmp/$BINARY" "$staged" || die "could not write to $dir"
	chmod 755 "$staged"
	mv -f "$staged" "$dir/$BINARY" || {
		rm -f "$staged"
		die "could not install to $dir/$BINARY"
	}

	printf '%s%s %s installed to %s%s\n' "$GREEN" "$BINARY" "$VERSION" "$dir/$BINARY" "$RESET"

	case ":${PATH:-}:" in
	*":$dir:"*) ;;
	*)
		warn "$dir is not on your PATH. Add it with:"
		say "    export PATH=\"$dir:\$PATH\""
		;;
	esac

	resolved=$(command -v "$BINARY" 2>/dev/null || true)
	if [ -n "$resolved" ] && [ "$resolved" != "$dir/$BINARY" ]; then
		warn "another $BINARY comes first on your PATH: $resolved"
	fi

	say ""
	say "Next steps:"
	say "    $BINARY doctor        # check your setup"
	say "    $BINARY agent list    # see the agents you have installed"
}

# Everything is defined before anything runs, so a download that is cut off
# part-way cannot execute a truncated script.
main "$@"
