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
	# Build lsftp
	$(GOBUILD) ./cmd/lsftp

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
