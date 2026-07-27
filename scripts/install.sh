#!/usr/bin/env sh

set -eu

usage() {
    cat <<'EOF'
Usage: ./scripts/install.sh [--install-dir DIR]

Installs the bundled DiffBeacon binary, or builds it when run from a checkout.
The default directory is $DIFFBEACON_INSTALL_DIR or $HOME/.local/bin.
EOF
}

install_dir=${DIFFBEACON_INSTALL_DIR:-"${HOME:?HOME is not set}/.local/bin"}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --install-dir)
            if [ "$#" -lt 2 ] || [ -z "$2" ]; then
                printf '%s\n' "install.sh: --install-dir requires a directory" >&2
                exit 2
            fi
            install_dir=$2
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            printf '%s\n' "install.sh: unknown argument: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

script_dir=$(CDPATH= cd "$(dirname "$0")" && pwd)
bundled_binary=$script_dir/diffbeacon

if ! command -v git >/dev/null 2>&1; then
	printf '%s\n' "install.sh: required command not found: git" >&2
	exit 1
fi
if [ ! -f "$bundled_binary" ] && ! command -v go >/dev/null 2>&1; then
	printf '%s\n' "install.sh: bundled binary not found and required command not found: go" >&2
	exit 1
fi

temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/diffbeacon-install.XXXXXX")
trap 'rm -rf -- "$temp_dir"' EXIT HUP INT TERM

if [ -f "$bundled_binary" ]; then
	printf '%s\n' "Installing bundled DiffBeacon..."
	cp "$bundled_binary" "$temp_dir/diffbeacon"
else
	repo_root=$(dirname "$script_dir")
	version=$(tr -d '\r\n' < "$repo_root/VERSION")
	printf '%s\n' "Building DiffBeacon..."
	(
		cd "$repo_root"
		CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=$version -s -w" -o "$temp_dir/diffbeacon" ./cmd/diffbeacon
	)
fi

mkdir -p "$install_dir"
if command -v install >/dev/null 2>&1; then
    install -m 0755 "$temp_dir/diffbeacon" "$install_dir/diffbeacon"
else
    cp "$temp_dir/diffbeacon" "$install_dir/diffbeacon"
    chmod 0755 "$install_dir/diffbeacon"
fi

"$install_dir/diffbeacon" --version

printf '%s\n' "Installed DiffBeacon at $install_dir/diffbeacon"
case ":${PATH:-}:" in
    *":$install_dir:"*) ;;
    *) printf '%s\n' "Add $install_dir to PATH to run 'diffbeacon' from any directory." ;;
esac
