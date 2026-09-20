VERSION ?= 0.2.0-pilot
LDFLAGS = -s -w -X github.com/BCSoftware-LLC/permits-agent/internal/cli.Version=$(VERSION) -X github.com/BCSoftware-LLC/permits-agent/internal/mcpserver.Version=$(VERSION)

.PHONY: check test build audit release
check:
	@test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal; exit 1)
	go vet ./...
	go test -race ./...
	go mod verify
	node --check internal/platform/web/app.js
	node --check internal/platform/web/analytics.js
	node --test tests/*.test.mjs

build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o bin/abc-agent ./cmd/abc-agent
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o bin/abc-agent-mcp ./cmd/abc-agent-mcp
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o bin/permits-agent-server ./cmd/permits-agent-server

test:
	go test ./...

audit:
	go run golang.org/x/vuln/cmd/govulncheck@v1.7.0 ./...

release:
	@test -z "$$(git status --porcelain --untracked-files=normal)" || (echo "Commit the reviewed source before packaging a release"; exit 1)
	mkdir -p dist
	@set -eu; for os in darwin linux; do \
	  for arch in arm64 amd64; do \
	    for name in abc-agent abc-agent-mcp permits-agent-server; do \
	      CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags '$(LDFLAGS)' -o dist/$$name-$$os-$$arch ./cmd/$$name; \
	    done; \
	  done; \
	done
	cp LICENSE THIRD_PARTY_NOTICES.md README.md dist/
	printf '{"version":"%s","source_commit":"%s","source_tree":"%s"}\n' '$(VERSION)' "$$(git rev-parse HEAD)" "$$(git rev-parse HEAD^{tree})" > dist/release-manifest.json
	cd dist && shasum -a 256 abc-agent-* permits-agent-server-* LICENSE THIRD_PARTY_NOTICES.md README.md release-manifest.json > SHA256SUMS
