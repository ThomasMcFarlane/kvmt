VERSION ?= dev
GOFLAGS := -trimpath
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test release clean

build:
	mkdir -p bin
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/kvmvidswitcher ./cmd/kvmvidswitcher

test:
	go test ./...

release: clean
	mkdir -p dist
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/kvmvidswitcher-darwin-arm64 ./cmd/kvmvidswitcher
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/kvmvidswitcher-windows-amd64.exe ./cmd/kvmvidswitcher
	cd dist && shasum -a 256 kvmvidswitcher-darwin-arm64 kvmvidswitcher-windows-amd64.exe > checksums.txt

clean:
	rm -rf bin dist
