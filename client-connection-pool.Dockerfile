# Stage 1: Build Stage
FROM golang:1.23-bullseye AS builder

WORKDIR /app

# Copy go.mod and go.sum to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the client binary
RUN go build -o client ./cli/client-connection-pool

# Stage 2: Runtime Stage
FROM debian:bullseye-slim

WORKDIR /app

ENV SERVER_ADDRESS=dns:///server-headless:5001
ENV POOL_MAX_CONNS=250
ENV POOL_MIN_CONNS=20
ENV POOL_IDLE_TIMEOUT=10s

# Copy the client binary from the builder stage
COPY --from=builder /app/client .

# Run the client binary
CMD ["./client"]
