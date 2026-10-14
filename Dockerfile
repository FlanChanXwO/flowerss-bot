# syntax=docker/dockerfile:1.7
FROM golang:1.26-alpine AS builder

# The pinned go-sqlite3 version's bundled C source is incompatible with modern
# musl. Link to Alpine's maintained SQLite library instead.
RUN apk add --no-cache gcc musl-dev sqlite-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY main.go ./
COPY internal ./internal
COPY pkg ./pkg
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -tags 'libsqlite3 sqlite_omit_load_extension' -trimpath -ldflags="-s -w" -o /out/flowerss-bot .

FROM alpine:3.23
RUN apk add --no-cache ca-certificates sqlite-libs
COPY --from=builder /out/flowerss-bot /usr/local/bin/flowerss-bot
WORKDIR /root/.flowerss
VOLUME ["/root/.flowerss"]
ENTRYPOINT ["/usr/local/bin/flowerss-bot"]
