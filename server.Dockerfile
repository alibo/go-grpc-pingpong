# Stage 1: Build Stage
FROM golang:1.23-bullseye AS builder

WORKDIR /app

# Copy go.mod and go.sum to download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the server binary
RUN go build -o server ./cli/server

# Stage 2: Runtime Stage
FROM debian:bullseye-slim

WORKDIR /app

# Copy the server binary from the builder stage
COPY --from=builder /app/server .

# Expose the gRPC port
EXPOSE 5001

# Run the server binary
CMD ["./server"]
