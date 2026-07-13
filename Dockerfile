# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /shiftz ./cmd/app

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
	&& adduser -D -u 10001 -g app appuser

WORKDIR /app

COPY --from=builder /shiftz /app/shiftz

USER appuser

EXPOSE 8080

ENTRYPOINT ["/app/shiftz"]
