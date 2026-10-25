# ---------------------------------------------------------------------------
# Stage 1: build a static binary.
# ---------------------------------------------------------------------------
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Dependencies first, so they stay cached until go.mod/go.sum change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd

# ---------------------------------------------------------------------------
# Stage 2: a small runtime image. Alpine (not distroless) because the ECS
# container health check runs busybox's wget against /healthz.
# ---------------------------------------------------------------------------
FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app && adduser -S app -G app

COPY --from=builder /out/server /usr/local/bin/server

USER app
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/server"]
