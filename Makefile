# Go コマンド
GOCMD=go
MODULE=GO111MODULE=on
GOBUILD=$(MODULE) $(GOCMD) build
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
	# copy lssh binary to /usr/local/bin/
	cp lssh /usr/local/bin/
	# copy lscp binary to /usr/local/bin/
	cp lscp /usr/local/bin/
	# copy lsftp binary to /usr/local/bin/
	cp lsftp /usr/local/bin/
	cp -n example/config.tml ~/.lssh.conf || true

test:
	$(GOTEST) ./...
