VERSION ?= dev
GOFLAGS := -trimpath
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test release clean

build:
	mkdir -p bin
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/kvmt ./cmd/kvmt

test:
	go test ./...

release: clean
	mkdir -p dist
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/kvmt-darwin-arm64 ./cmd/kvmt
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/kvmt-windows-amd64.exe ./cmd/kvmt
	cd dist && shasum -a 256 kvmt-darwin-arm64 kvmt-windows-amd64.exe > checksums.txt

clean:
	rm -rf bin dist
