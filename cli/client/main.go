package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"sync"
	"time"

	pb "github.com/alibo/go-grpc-pingpong/pkg/pingpong"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/peer"
)

func percentile(latencies []float64, percent float64) float64 {
	if len(latencies) == 0 {
		return 0
	}
	sort.Float64s(latencies)
	index := (percent / 100.0) * float64(len(latencies)-1)
	i := int(index)
	if i >= len(latencies)-1 {
		return latencies[len(latencies)-1]
	}
	return latencies[i] + (latencies[i+1]-latencies[i])*(index-float64(i))
}

func main() {
	// Set DNS re-resolution interval to handle new server pods
	os.Setenv("GRPC_GO_RESOLVER_DNS_MIN_TIME_BETWEEN_RESOLUTIONS", "5")

	// Get the server address from the environment variable
	serverAddress := os.Getenv("SERVER_ADDRESS")
	if serverAddress == "" {
		serverAddress = "dns:///server-headless:5001" // Assuming Kubernetes service
	}

	// Set up keepalive parameters for the client
	kaParams := keepalive.ClientParameters{
		Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
		Timeout:             2 * time.Second,  // wait 2 seconds for ping ack before considering the connection dead
		PermitWithoutStream: true,             // send pings even without active streams
	}

	// Create a context with a timeout for the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use grpc.DialContext with the required options
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

	var latenciesByServer = make(map[string][]float64)
	var totalRequestsByServer = make(map[string]int)
	var latenciesLock sync.Mutex

	statsTicker := time.NewTicker(10 * time.Second)
	defer statsTicker.Stop()

	for {
		select {
		case t := <-ticker.C:
			startTime := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			var p peer.Peer
			response, err := client.Ping(ctx, &pb.PingRequest{Message: "Ping"}, grpc.Peer(&p))
			cancel()

			latency := time.Since(startTime)

			if err != nil {
				log.Printf("Error at %v: %v", t.Format(time.RFC3339Nano), err)
			} else {
				serverAddr := p.Addr.String()
				log.Printf("Received %s from %s at %v, latency: %v", response.Message, serverAddr, t.Format(time.RFC3339Nano), latency)

				latenciesLock.Lock()
				latenciesByServer[serverAddr] = append(latenciesByServer[serverAddr], latency.Seconds()*1000) // in milliseconds
				totalRequestsByServer[serverAddr]++
				latenciesLock.Unlock()
			}
		case <-statsTicker.C:
			latenciesLock.Lock()
			if len(latenciesByServer) > 0 {
				fmt.Printf("\nLatency Stats (last 10s):\n")
				for serverAddr, latencies := range latenciesByServer {
					totalRequests := totalRequestsByServer[serverAddr]
					p50 := percentile(latencies, 50)
					p95 := percentile(latencies, 95)
					p99 := percentile(latencies, 99)
					fmt.Printf("Server %s - Total Requests: %d\n", serverAddr, totalRequests)
					fmt.Printf("P50: %.2f ms, P95: %.2f ms, P99: %.2f ms\n", p50, p95, p99)
				}
				// Reset latencies and totalRequests
				latenciesByServer = make(map[string][]float64)
				totalRequestsByServer = make(map[string]int)
			} else {
				fmt.Println("\nNo latency data collected in the last 10s.")
			}
			latenciesLock.Unlock()

			// Print connection pool status
			fmt.Println("\nConnection Pool Status:")
			printConnectionInfo(conn)
		}
	}
}

// Function to print basic connection information
func printConnectionInfo(conn *grpc.ClientConn) {
	state := conn.GetState()
	fmt.Printf("Connection State: %v\n", state)
	fmt.Printf("Target: %s\n", conn.Target())

	// Since detailed connection info is not directly exposed, we can print the connectivity state
	// of the subchannels if possible.

	cs := conn.GetState()
	fmt.Printf("Connection overall state: %v\n", cs)
}
