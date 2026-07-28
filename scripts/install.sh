#!/usr/bin/env sh

set -eu

usage() {
    cat <<'EOF'
Usage: ./scripts/install.sh [--install-dir DIR]

Installs the bundled DiffBeacon binary, builds it from a checkout, or downloads
the latest GitHub Release when executed as a standalone script.
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

case "${0##*/}" in
	install.sh)
		script_dir=$(CDPATH= cd "$(dirname "$0")" && pwd)
		bundled_binary=$script_dir/diffbeacon
		repo_root=$(dirname "$script_dir")
		source_tree=$repo_root/go.mod
		;;
	*)
		bundled_binary=
		repo_root=
		source_tree=
		;;
esac

if ! command -v git >/dev/null 2>&1; then
	printf '%s\n' "install.sh: required command not found: git" >&2
	exit 1
fi
if [ ! -f "$bundled_binary" ] && [ -n "$source_tree" ] && [ -f "$source_tree" ] && ! command -v go >/dev/null 2>&1; then
	printf '%s\n' "install.sh: source checkout detected and required command not found: go" >&2
	exit 1
fi

temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/diffbeacon-install.XXXXXX")
trap 'rm -rf -- "$temp_dir"' EXIT HUP INT TERM

if [ -f "$bundled_binary" ]; then
	printf '%s\n' "Installing bundled DiffBeacon..."
	cp "$bundled_binary" "$temp_dir/diffbeacon"
elif [ -n "$source_tree" ] && [ -f "$source_tree" ]; then
	printf '%s\n' "Building DiffBeacon..."
	(
		cd "$repo_root"
		CGO_ENABLED=0 go build -trimpath -o "$temp_dir/diffbeacon" ./cmd/diffbeacon
	)
else
	if ! command -v curl >/dev/null 2>&1; then
		printf '%s\n' "install.sh: standalone installation requires curl" >&2
		exit 1
	fi

	case "$(uname -s)" in
		Linux) platform=linux ;;
		Darwin) platform=darwin ;;
		*)
			printf '%s\n' "install.sh: unsupported operating system: $(uname -s)" >&2
			exit 1
			;;
	esac
	case "$(uname -m)" in
		x86_64|amd64) architecture=amd64 ;;
		arm64|aarch64) architecture=arm64 ;;
		*)
			printf '%s\n' "install.sh: unsupported architecture: $(uname -m)" >&2
			exit 1
			;;
	esac

	repository=https://github.com/andrespistoni/diffbeacon
	latest_url=$repository/releases/latest
	resolved_url=$(curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 --output /dev/null --write-out '%{url_effective}' "$latest_url")
	tag=${resolved_url##*/}
	if ! printf '%s\n' "$tag" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
		printf '%s\n' "install.sh: could not resolve a stable release from $resolved_url" >&2
		exit 1
	fi
	version=${tag#v}

	archive=diffbeacon_${version}_${platform}_${architecture}.tar.gz
	download_base=$repository/releases/download/$tag
	printf '%s\n' "Downloading DiffBeacon $version for $platform/$architecture..."
	curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 --output "$temp_dir/$archive" "$download_base/$archive"
	curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 --output "$temp_dir/SHA256SUMS" "$download_base/SHA256SUMS"

	expected=$(awk -v file="$archive" '$2 == file { print $1; exit }' "$temp_dir/SHA256SUMS")
	if [ -z "$expected" ]; then
		printf '%s\n' "install.sh: checksum not found for $archive" >&2
		exit 1
	fi
	if command -v sha256sum >/dev/null 2>&1; then
		actual=$(sha256sum "$temp_dir/$archive" | awk '{ print $1 }')
	elif command -v shasum >/dev/null 2>&1; then
		actual=$(shasum -a 256 "$temp_dir/$archive" | awk '{ print $1 }')
	else
		printf '%s\n' "install.sh: sha256sum or shasum is required" >&2
		exit 1
	fi
	if [ "$actual" != "$expected" ]; then
		printf '%s\n' "install.sh: SHA-256 mismatch for $archive" >&2
		exit 1
	fi

	mkdir "$temp_dir/package"
	tar -xzf "$temp_dir/$archive" -C "$temp_dir/package" diffbeacon
	cp "$temp_dir/package/diffbeacon" "$temp_dir/diffbeacon"
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
