# Go コマンド
GOCMD=go
MODULE=GO111MODULE=on
GOBUILD=$(MODULE) $(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(MODULE) $(GOCMD) test -cover
GOGET=$(GOCMD) get
GOMOD=$(MODULE) $(GOCMD) mod
GOINSTALL=$(MODULE) $(GOCMD) install

build:
	# Remove unnecessary dependent libraries
	$(GOMOD) tidy
	# Place dependent libraries under vendor
	$(GOMOD) vendor
	# Build lssh
	$(GOBUILD) ./cmd/lssh
	# Build lscp
	$(GOBUILD) ./cmd/lscp

clean:
	$(GOCLEAN) ./...
	rm -f lssh
	rm -f lscp

install:
	cp lssh /usr/local/bin/
	cp lscp /usr/local/bin/
	cp -n example/config.tml ~/.lssh.conf || true

test:
	$(GOTEST) ./...
