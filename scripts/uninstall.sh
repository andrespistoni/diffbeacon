#!/usr/bin/env sh

set -eu

usage() {
    cat <<'EOF'
Usage: ./scripts/uninstall.sh [--install-dir DIR]

Removes the DiffBeacon binary installed by scripts/install.sh.
The default directory is $DIFFBEACON_INSTALL_DIR or $HOME/.local/bin.
EOF
}

install_dir=${DIFFBEACON_INSTALL_DIR:-"${HOME:?HOME is not set}/.local/bin"}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --install-dir)
            if [ "$#" -lt 2 ] || [ -z "$2" ]; then
                printf '%s\n' "uninstall.sh: --install-dir requires a directory" >&2
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
            printf '%s\n' "uninstall.sh: unknown argument: $1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

target=$install_dir/diffbeacon
if [ -d "$target" ]; then
    printf '%s\n' "uninstall.sh: refusing to remove directory: $target" >&2
    exit 1
fi

if [ -e "$target" ] || [ -L "$target" ]; then
    rm -f -- "$target"
    printf '%s\n' "Removed $target"
else
    printf '%s\n' "DiffBeacon is not installed at $target"
fi

rmdir "$install_dir" 2>/dev/null || true
