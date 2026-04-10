# Go コマンド
GOCMD=go
MODULE=GO111MODULE=on
GOBUILD=$(MODULE) $(GOCMD) build -ldflags -w
GOCLEAN=$(GOCMD) clean
GOTEST=$(MODULE) $(GOCMD) test -cover
GOGET=$(GOCMD) get
GOMOD=$(MODULE) $(GOCMD) mod
GOINSTALL=$(MODULE) $(GOCMD) install

# OS別にbuildのコマンド生成
UNAME_S=$(shell uname -s)
BUILDCMD_LSSH=$(GOBUILD) ./cmd/lssh
BUILDCMD_LSCP=$(GOBUILD) ./cmd/lscp
BUILDCMD_LSFTP=$(GOBUILD) ./cmd/lsftp
BUILDCMD_LSSYNC=$(GOBUILD) ./cmd/lssync
BUILDCMD_LSMON=$(GOBUILD) ./cmd/lsmon
BUILDCMD_LSSHELL=$(GOBUILD) ./cmd/lsshell

# install path
INSTALL_PATH_LSSH=/usr/local/bin/lssh
INSTALL_PATH_LSCP=/usr/local/bin/lscp
INSTALL_PATH_LSFTP=/usr/local/bin/lsftp
INSTALL_PATH_LSSYNC=/usr/local/bin/lssync
INSTALL_PATH_LSSHELL=/usr/local/bin/lsshell
INSTALL_PATH_LSMON=/usr/local/bin/lsmon

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

clean:
	$(GOCLEAN) ./...
	rm -f lssh
	rm -f lscp
	rm -f lsftp
	rm -f lssync
	rm -f lsmon
	rm -f lsshell

install:
	# rm old binary
	[ -e $(INSTALL_PATH_LSSH) ] && rm $(INSTALL_PATH_LSSH) || true
	[ -e $(INSTALL_PATH_LSCP) ] && rm $(INSTALL_PATH_LSCP) || true
	[ -e $(INSTALL_PATH_LSFTP) ] && rm $(INSTALL_PATH_LSFTP) || true
	[ -e $(INSTALL_PATH_LSSYNC) ] && rm $(INSTALL_PATH_LSSYNC) || true
	[ -e $(INSTALL_PATH_LSSHELL) ] && rm $(INSTALL_PATH_LSSHELL) || true
	[ -e $(INSTALL_PATH_LSMON) ] && rm $(INSTALL_PATH_LSMON) || true

	# copy binary to /usr/local/bin/
	cp lssh $(INSTALL_PATH_LSSH)
	cp lscp $(INSTALL_PATH_LSCP)
	cp lsftp $(INSTALL_PATH_LSFTP)
	cp lssync $(INSTALL_PATH_LSSYNC)
	cp lsshell $(INSTALL_PATH_LSSHELL)
	cp lsmon $(INSTALL_PATH_LSMON)

	# copy template config file
	cp -n example/config.tml ~/.lssh.conf || true

install-completions:
	./scripts/install-completions.sh all --system

install-completions-user:
	./scripts/install-completions.sh all --user

test:
	$(GOTEST) ./...
