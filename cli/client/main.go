package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"sync"
	"time"

	pb "github.com/alibo/go-grpc-pingpong/pkg/pingpong"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

func percentile(latencies []float64, percent float64) float64 {
	if len(latencies) == 0 {
		return 0
	}
	sort.Float64s(latencies)
	rank := percent / 100.0 * float64(len(latencies)-1)
	lower := int(math.Floor(rank))
	upper := int(math.Ceil(rank))
	if lower == upper {
		return latencies[lower]
	}
	lowerValue := latencies[lower]
	upperValue := latencies[upper]
	weight := rank - float64(lower)
	return lowerValue*(1-weight) + upperValue*weight
}

func main() {
	// Get the server address from the environment variable
	serverAddress := os.Getenv("SERVER_ADDRESS")
	if serverAddress == "" {
		serverAddress = "dns:///server-headless:5001" // Default address, assuming Kubernetes service
	}

	// Set up keepalive parameters
	kaParams := keepalive.ClientParameters{
		Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
		Timeout:             2 * time.Second,  // wait 2 seconds for ping ack before considering the connection dead
		PermitWithoutStream: true,             // send pings even without active streams
	}

	// Create a context with a timeout for the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use grpc.DialContext with the updated credentials options
	conn, err := grpc.DialContext(ctx, serverAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithKeepaliveParams(kaParams),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		log.Fatalf("Failed to connect to %s: %v", serverAddress, err)
	}
	defer conn.Close()

	client := pb.NewPingPongClient(conn)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var latencies []float64
	var latenciesLock sync.Mutex
	var totalRequests int
	statsTicker := time.NewTicker(10 * time.Second)
	defer statsTicker.Stop()

	for {
		select {
		case t := <-ticker.C:
			startTime := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			response, err := client.Ping(ctx, &pb.PingRequest{Message: "Ping"})
			cancel()

			latency := time.Since(startTime)
			latenciesLock.Lock()
			latencies = append(latencies, latency.Seconds()*1000) // in milliseconds
			totalRequests++
			latenciesLock.Unlock()

			if err != nil {
				log.Printf("Error at %v: %v", t.Format(time.RFC3339Nano), err)
			} else {
				log.Printf("Received %s at %v, latency: %v", response.Message, t.Format(time.RFC3339Nano), latency)
			}
		case <-statsTicker.C:
			latenciesLock.Lock()
			if len(latencies) > 0 {
				p50 := percentile(latencies, 50)
				p95 := percentile(latencies, 95)
				p99 := percentile(latencies, 99)
				fmt.Printf("\nLatency Stats (last 10s): Total Requests: %d\n", totalRequests)
				fmt.Printf("P50: %.2f ms, P95: %.2f ms, P99: %.2f ms\n", p50, p95, p99)
				// Reset latencies and totalRequests
				latencies = nil
				totalRequests = 0
			} else {
				fmt.Println("\nNo latency data collected in the last 10s.")
			}
			latenciesLock.Unlock()
		}
	}
}
