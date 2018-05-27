# Go コマンド
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get

build:
	$(GOBUILD) lssh.go
	$(GOBUILD) lscp.go
clean:
	$(GOCLEAN)
	rm -f lssh
	rm -f lscp
install:
	cp lssh /usr/local/bin/
	cp lscp /usr/local/bin/
	cp -n example/config.tml ~/.lssh.conf || true
