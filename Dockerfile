# Build stage
ARG GO_VERSION
FROM golang:${GO_VERSION}-alpine AS builder

# Set to "swagger" to build with /swagger UI enabled (non-production).
ARG BUILD_TAGS=

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code
COPY . .

# Build the API and migration tool
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -tags "$BUILD_TAGS" -o api ./cmd/api && \
    CGO_ENABLED=0 GOOS=linux go build -tags "$BUILD_TAGS" -o migrate ./cmd/migrate

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS and tzdata for timezone support
RUN apk --no-cache add ca-certificates tzdata

# Copy binaries from builder
COPY --from=builder /app/api .
COPY --from=builder /app/migrate .
COPY --from=builder /app/migrations ./migrations

# Expose port
EXPOSE 8080

# Run migrations and start API
CMD ["./api"]
