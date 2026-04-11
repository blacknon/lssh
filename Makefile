# Go コマンド
GOCMD=go
MODULE=GO111MODULE=on
GOBUILD=$(MODULE) $(GOCMD) build -ldflags -w
GOCLEAN=$(GOCMD) clean
GOTEST=$(MODULE) $(GOCMD) test -cover
GOGET=$(GOCMD) get
GOMOD=$(MODULE) $(GOCMD) mod
GOINSTALL=$(MODULE) $(GOCMD) install
COMPLETION_SHELL?=all
COMPLETION_PREFIX?=

# OS別にbuildのコマンド生成
UNAME_S=$(shell uname -s)
BUILDCMD_LSSH=$(GOBUILD) ./cmd/lssh
BUILDCMD_LSCP=$(GOBUILD) ./cmd/lscp
BUILDCMD_LSFTP=$(GOBUILD) ./cmd/lsftp
BUILDCMD_LSSYNC=$(GOBUILD) ./cmd/lssync
BUILDCMD_LSMON=$(GOBUILD) ./cmd/lsmon
BUILDCMD_LSSHELL=$(GOBUILD) ./cmd/lsshell
BUILDCMD_LSPIPE=$(GOBUILD) ./cmd/lspipe

# install path
INSTALL_PATH_LSSH=/usr/local/bin/lssh
INSTALL_PATH_LSCP=/usr/local/bin/lscp
INSTALL_PATH_LSFTP=/usr/local/bin/lsftp
INSTALL_PATH_LSSYNC=/usr/local/bin/lssync
INSTALL_PATH_LSSHELL=/usr/local/bin/lsshell
INSTALL_PATH_LSMON=/usr/local/bin/lsmon
INSTALL_PATH_LSPIPE=/usr/local/bin/lspipe

build:
	# Remove unnecessary dependent libraries
	$(GOMOD) tidy
	# Place dependent libraries under vendor
	$(GOMOD) vendor

	# Build lsshgo
	$(BUILDCMD_LSSH)
	# Build lscp
	$(BUILDCMD_LSCP)
	# Build lsftp
	$(BUILDCMD_LSFTP)
	# Build lssync
	$(BUILDCMD_LSSYNC)
	# Build lsmon
	$(BUILDCMD_LSMON)
	# Build lsshell
	$(BUILDCMD_LSSHELL)
	# Build lspipe
	$(BUILDCMD_LSPIPE)

clean:
	$(GOCLEAN) ./...
	rm -f lssh
	rm -f lscp
	rm -f lsftp
	rm -f lssync
	rm -f lsmon
	rm -f lsshell
	rm -f lspipe

install:
	# rm old binary
	[ -e $(INSTALL_PATH_LSSH) ] && rm $(INSTALL_PATH_LSSH) || true
	[ -e $(INSTALL_PATH_LSCP) ] && rm $(INSTALL_PATH_LSCP) || true
	[ -e $(INSTALL_PATH_LSFTP) ] && rm $(INSTALL_PATH_LSFTP) || true
	[ -e $(INSTALL_PATH_LSSYNC) ] && rm $(INSTALL_PATH_LSSYNC) || true
	[ -e $(INSTALL_PATH_LSSHELL) ] && rm $(INSTALL_PATH_LSSHELL) || true
	[ -e $(INSTALL_PATH_LSMON) ] && rm $(INSTALL_PATH_LSMON) || true
	[ -e $(INSTALL_PATH_LSPIPE) ] && rm $(INSTALL_PATH_LSPIPE) || true

	# copy binary to /usr/local/bin/
	cp lssh $(INSTALL_PATH_LSSH)
	cp lscp $(INSTALL_PATH_LSCP)
	cp lsftp $(INSTALL_PATH_LSFTP)
	cp lssync $(INSTALL_PATH_LSSYNC)
	cp lsshell $(INSTALL_PATH_LSSHELL)
	cp lsmon $(INSTALL_PATH_LSMON)
	cp lspipe $(INSTALL_PATH_LSPIPE)

	# copy template config file
	cp -n example/config.tml ~/.lssh.conf || true

install-completions:
	@set -eu; \
	shell_name="$(COMPLETION_SHELL)"; \
	prefix="$(if $(COMPLETION_PREFIX),$(COMPLETION_PREFIX),/usr/local)"; \
	install_shell() { \
		src_dir="$$1"; \
		dst_dir="$$2"; \
		mkdir -p "$$dst_dir"; \
		for src in "$$src_dir"/*; do \
			[ -f "$$src" ] || continue; \
			cp "$$src" "$$dst_dir/"; \
			printf 'installed %s -> %s\n' "$$src" "$$dst_dir/"; \
		done; \
	}; \
	case "$$shell_name" in \
		bash) install_shell completion/bash "$$prefix/share/bash-completion/completions" ;; \
		zsh) install_shell completion/zsh "$$prefix/share/zsh/site-functions" ;; \
		fish) install_shell completion/fish "$$prefix/share/fish/vendor_completions.d" ;; \
		all) \
			install_shell completion/bash "$$prefix/share/bash-completion/completions"; \
			install_shell completion/zsh "$$prefix/share/zsh/site-functions"; \
			install_shell completion/fish "$$prefix/share/fish/vendor_completions.d"; \
			;; \
		*) echo "unknown COMPLETION_SHELL: $$shell_name" >&2; exit 1 ;; \
	esac

install-completions-user:
	@set -eu; \
	shell_name="$(COMPLETION_SHELL)"; \
	prefix="$(if $(COMPLETION_PREFIX),$(COMPLETION_PREFIX),$$HOME)"; \
	install_shell() { \
		src_dir="$$1"; \
		dst_dir="$$2"; \
		mkdir -p "$$dst_dir"; \
		for src in "$$src_dir"/*; do \
			[ -f "$$src" ] || continue; \
			cp "$$src" "$$dst_dir/"; \
			printf 'installed %s -> %s\n' "$$src" "$$dst_dir/"; \
		done; \
	}; \
	zsh_note=0; \
	case "$$shell_name" in \
		bash) install_shell completion/bash "$$prefix/.local/share/bash-completion/completions" ;; \
		zsh) install_shell completion/zsh "$$prefix/.zfunc"; zsh_note=1 ;; \
		fish) install_shell completion/fish "$$prefix/.config/fish/completions" ;; \
		all) \
			install_shell completion/bash "$$prefix/.local/share/bash-completion/completions"; \
			install_shell completion/zsh "$$prefix/.zfunc"; \
			install_shell completion/fish "$$prefix/.config/fish/completions"; \
			zsh_note=1; \
			;; \
		*) echo "unknown COMPLETION_SHELL: $$shell_name" >&2; exit 1 ;; \
	esac; \
	if [ "$$zsh_note" -eq 1 ]; then \
		printf '%s\n' \
			'zsh note:' \
			'  Add this to ~/.zshrc if ~/.zfunc is not already in fpath:' \
			'    fpath=($$HOME/.zfunc $$fpath)' \
			'    autoload -Uz compinit && compinit'; \
	fi

test:
	$(GOTEST) ./...
