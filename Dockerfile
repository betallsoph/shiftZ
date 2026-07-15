# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.25

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache git ca-certificates

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
	go mod download

COPY . .

RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
	go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/shiftz ./cmd/app

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
	&& adduser -D -u 10001 -g app appuser

WORKDIR /app

COPY --from=builder /out/shiftz /app/shiftz

ENV APP_ADDR=:8088

USER appuser

EXPOSE 8088

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
	CMD wget -qO- http://127.0.0.1:8088/livez || exit 1

ENTRYPOINT ["/app/shiftz"]
