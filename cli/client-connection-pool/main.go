package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	pb "github.com/alibo/go-grpc-pingpong/pkg/pingpong"
	grpcpool "github.com/processout/grpc-go-pool"
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
	// Get the server address from the environment variable
	serverAddress := os.Getenv("SERVER_ADDRESS")
	if serverAddress == "" {
		serverAddress = "localhost:5001" // Assuming Kubernetes service
	}

	// Set up keepalive parameters for the client
	kaParams := keepalive.ClientParameters{
		Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
		Timeout:             2 * time.Second,  // wait 2 seconds for ping ack before considering the connection dead
		PermitWithoutStream: true,             // send pings even without active streams
	}

	// Pool configuration with default values
	poolMinConns := 2
	poolMaxConns := 5
	poolIdleTimeout := 5 * time.Minute

	// Read pool configuration from environment variables (if provided)
	if val := os.Getenv("POOL_MAX_CONNS"); val != "" {
		if v, err := strconv.Atoi(val); err == nil {
			poolMaxConns = v
		}
	}
	if val := os.Getenv("POOL_MIN_CONNS"); val != "" {
		if v, err := strconv.Atoi(val); err == nil {
			poolMinConns = v
		}
	}
	if val := os.Getenv("POOL_IDLE_TIMEOUT"); val != "" {
		if v, err := time.ParseDuration(val); err == nil {
			poolIdleTimeout = v
		}
	}

	// Variables to track pool state
	var totalConnsCreated int
	var mu sync.Mutex

	// Create a function to create new connections
	factory := func() (*grpc.ClientConn, error) {
		conn, err := grpc.Dial(serverAddress,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithKeepaliveParams(kaParams),
			grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
		)
		if err != nil {
			log.Printf("Failed to connect to %s: %v", serverAddress, err)
		} else {
			// Increment the total connections created
			mu.Lock()
			totalConnsCreated++
			mu.Unlock()
		}
		return conn, err
	}

	// Create the connection pool
	pool, err := grpcpool.New(factory, poolMinConns, poolMaxConns, poolIdleTimeout)
	if err != nil {
		log.Fatalf("Failed to create gRPC connection pool: %v", err)
	}

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
			// Get a connection from the pool
			connRes, err := pool.Get(context.Background())
			if err != nil {
				log.Printf("Failed to get connection from pool: %v", err)
				continue
			}

			client := pb.NewPingPongClient(connRes.ClientConn)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			var p peer.Peer
			response, err := client.Ping(ctx, &pb.PingRequest{Message: "Ping"}, grpc.Peer(&p))
			cancel()

			// Return the connection to the pool
			connRes.Close()

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
			mu.Lock()
			fmt.Printf("\nConnection Pool State:\n")
			fmt.Printf("Total Connections Created: %d\n", totalConnsCreated)
			fmt.Printf("Current Idle Connections: %d\n", pool.Available())
			fmt.Printf("Pool Min Connections: %d\n", poolMinConns)
			fmt.Printf("Pool Max Connections: %d\n", poolMaxConns)
			mu.Unlock()
		}
	}
}
