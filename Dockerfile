FROM golang:1.26.7-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=0.2.0-pilot
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/BCSoftware-LLC/permits-agent/internal/mcpserver.Version=${VERSION}" -o /out/permits-agent-server ./cmd/permits-agent-server && \
    CGO_ENABLED=0 go build -trimpath -o /out/abc-agent ./cmd/abc-agent

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/permits-agent-server /usr/local/bin/permits-agent-server
COPY --from=build /out/abc-agent /usr/local/bin/abc-agent
ENV ABC_AGENT_CACHE=/data/abc
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/permits-agent-server"]
CMD ["--listen", "0.0.0.0:8080", "--database", "/data/private/cases.sqlite", "--keys", "/run/secrets/permits-keys.json"]
