VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -s -w -X github.com/bootcraft-cn/cli/internal/version.Version=$(VERSION) -X github.com/bootcraft-cn/cli/internal/version.Commit=$(COMMIT)

.PHONY: build test lint install clean snapshot release-check

build:
	go build -ldflags "$(LDFLAGS)" -o bin/bootcraft ./cmd/bootcraft

test:
	go test ./... -v

lint:
	go vet ./...

install: build
	cp bin/bootcraft /usr/local/bin/bootcraft

# 本地跑一次 goreleaser snapshot,产出多平台二进制到 dist/(无需 tag,无需 push)
# 需要 brew install goreleaser
snapshot:
	goreleaser release --snapshot --clean

# 校验 .goreleaser.yaml 语法
release-check:
	goreleaser check

clean:
	rm -rf bin/ dist/
