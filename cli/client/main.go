// client.go
package main

import (
	"context"
	"log"
	"os"
	"time"

	pb "github.com/alibo/go-grpc-pingpong/pkg/pingpong"

	"google.golang.org/grpc"
)

func main() {
	// Get the server address from the environment variable
	serverAddress := os.Getenv("SERVER_ADDRESS")
	if serverAddress == "" {
		serverAddress = "localhost:5001" // Default address
	}

	conn, err := grpc.Dial(serverAddress, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("Failed to connect to %s: %v", serverAddress, err)
	}
	defer conn.Close()
	client := pb.NewPingPongClient(conn)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for t := range ticker.C {
		startTime := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		response, err := client.Ping(ctx, &pb.PingRequest{Message: "Ping"})
		cancel()

		latency := time.Since(startTime)
		if err != nil {
			log.Printf("Error at %v: %v", t.Format(time.RFC3339Nano), err)
		} else {
			log.Printf("Received %s at %v, latency: %v", response.Message, t.Format(time.RFC3339Nano), latency)
		}
	}
}
