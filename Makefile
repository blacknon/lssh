# Go コマンド
GOCMD=go
MODULE=GO111MODULE=on
GOBUILD=$(MODULE) $(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(MODULE) $(GOCMD) mod
GOINSTALL=$(MODULE) $(GOCMD) install

build:
	# 依存ライブラリの不要なものを削除
	$(GOMOD) tidy
	# 依存ライブラリをvendor配下に配置
	$(GOMOD) vendor
	$(GOBUILD) ./cmd/lssh
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
	go test -cover ./...
