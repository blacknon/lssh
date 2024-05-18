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

# install path
INSTALL_PATH_LSSH=/usr/local/bin/lssh
INSTALL_PATH_LSCP=/usr/local/bin/lscp
INSTALL_PATH_LSFTP=/usr/local/bin/lsftp

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

clean:
	$(GOCLEAN) ./...
	rm -f lssh
	rm -f lscp
	rm -f lsftp

install:
	# rm old binary
	[ -e $(INSTALL_PATH_LSSH) ] && rm $(INSTALL_PATH_LSSH) || true
	[ -e $(INSTALL_PATH_LSCP) ] && rm $(INSTALL_PATH_LSCP) || true
	[ -e $(INSTALL_PATH_LSFTP) ] && rm $(INSTALL_PATH_LSFTP) || true

	# copy binary to /usr/local/bin/
	cp lssh $(INSTALL_PATH_LSSH)
	cp lscp $(INSTALL_PATH_LSCP)
	cp lsftp $(INSTALL_PATH_LSFTP)

	# copy template config file
	cp -n example/config.tml ~/.lssh.conf || true

test:
	$(GOTEST) ./...
