FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /senda ./cmd/senda

# Runtime: chromedp/headless-shell ships a minimal Debian + Chromium headless-shell
# at /headless-shell/headless-shell. Adds ~170MB versus alpine, but enables the
# template screenshot feature. Set screenshot.enabled=false to skip Chromium use.
FROM chromedp/headless-shell:latest

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /senda /usr/local/bin/senda

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/senda"]
