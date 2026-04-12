VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -s -w -X github.com/bootcraft-cn/cli/internal/version.Version=$(VERSION) -X github.com/bootcraft-cn/cli/internal/version.Commit=$(COMMIT)

.PHONY: build test lint install clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/bootcraft ./cmd/bootcraft

test:
	go test ./... -v

lint:
	go vet ./...

install: build
	cp bin/bootcraft /usr/local/bin/bootcraft

clean:
	rm -rf bin/
