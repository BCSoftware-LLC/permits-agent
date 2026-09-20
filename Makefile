VERSION ?= 0.1.0
LDFLAGS = -s -w -X github.com/BCSoftware-LLC/permits-agent/internal/cli.Version=$(VERSION) -X github.com/BCSoftware-LLC/permits-agent/internal/mcpserver.Version=$(VERSION)

.PHONY: check test build audit release
check:
	@test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal; exit 1)
	go vet ./...
	go test -race ./...
	go mod verify

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o bin/abc-agent ./cmd/abc-agent
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o bin/abc-agent-mcp ./cmd/abc-agent-mcp

test:
	go test ./...

audit:
	go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...

release:
	mkdir -p dist
	@set -eu; for os in darwin linux; do \
	  for arch in arm64 amd64; do \
	    for name in abc-agent abc-agent-mcp; do \
	      CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags '$(LDFLAGS)' -o dist/$$name-$$os-$$arch ./cmd/$$name; \
	    done; \
	  done; \
	done
	cd dist && shasum -a 256 abc-agent-* > SHA256SUMS
