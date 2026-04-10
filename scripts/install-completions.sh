#!/bin/sh

set -eu

usage() {
    cat <<'EOF'
Usage:
  scripts/install-completions.sh [bash|zsh|fish|all] [--user|--system] [--prefix DIR]

Examples:
  scripts/install-completions.sh all --user
  sudo scripts/install-completions.sh all --system
  scripts/install-completions.sh zsh --prefix "$HOME/.local"
EOF
}

shell_name="all"
scope="user"
prefix=""

while [ "$#" -gt 0 ]; do
    case "$1" in
        bash|zsh|fish|all)
            shell_name="$1"
            ;;
        --user)
            scope="user"
            ;;
        --system)
            scope="system"
            ;;
        --prefix)
            shift
            [ "$#" -gt 0 ] || { echo "missing value for --prefix" >&2; exit 1; }
            prefix="$1"
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "unknown argument: $1" >&2
            usage >&2
            exit 1
            ;;
    esac
    shift
done

repo_root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)

if [ -z "$prefix" ]; then
    if [ "$scope" = "system" ]; then
        prefix="/usr/local"
    else
        prefix="$HOME"
    fi
fi

install_shell() {
    src_dir=$1
    dst_dir=$2

    mkdir -p "$dst_dir"
    for src in "$src_dir"/*; do
        [ -f "$src" ] || continue
        cp "$src" "$dst_dir/"
        printf 'installed %s -> %s\n' "$src" "$dst_dir/"
    done
}

install_bash() {
    if [ "$scope" = "system" ]; then
        dst="$prefix/share/bash-completion/completions"
    else
        dst="$prefix/.local/share/bash-completion/completions"
    fi
    install_shell "$repo_root/completion/bash" "$dst"
}

install_zsh() {
    if [ "$scope" = "system" ]; then
        dst="$prefix/share/zsh/site-functions"
    else
        dst="$prefix/.zfunc"
    fi
    install_shell "$repo_root/completion/zsh" "$dst"

    if [ "$scope" = "user" ]; then
        cat <<EOF
zsh note:
  Add this to ~/.zshrc if ~/.zfunc is not already in fpath:
    fpath=(\$HOME/.zfunc \$fpath)
    autoload -Uz compinit && compinit
EOF
    fi
}

install_fish() {
    if [ "$scope" = "system" ]; then
        dst="$prefix/share/fish/vendor_completions.d"
    else
        dst="$prefix/.config/fish/completions"
    fi
    install_shell "$repo_root/completion/fish" "$dst"
}

case "$shell_name" in
    bash)
        install_bash
        ;;
    zsh)
        install_zsh
        ;;
    fish)
        install_fish
        ;;
    all)
        install_bash
        install_zsh
        install_fish
        ;;
esac
