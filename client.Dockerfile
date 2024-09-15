# Stage 1: Build Stage
FROM golang:1.23-bullseye AS builder

WORKDIR /app

# Copy go.mod and go.sum to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the client binary
RUN go build -o client cli/client

# Stage 2: Runtime Stage
FROM debian:bullseye-slim

WORKDIR /app

# Set the default server address (can be overridden at runtime)
ENV SERVER_ADDRESS=server:5001

# Copy the client binary from the builder stage
COPY --from=builder /app/client .

# Run the client binary
CMD ["./client"]
