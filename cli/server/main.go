package main

import (
	"context"
	"log"
	"net"
	"time"

	pb "github.com/alibo/go-grpc-pingpong/pkg/pingpong"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/peer"
)

type server struct {
	pb.UnimplementedPingPongServer
}

func (s *server) Ping(ctx context.Context, in *pb.PingRequest) (*pb.PongResponse, error) {
	peerInfo, ok := peer.FromContext(ctx)
	if !ok {
		log.Printf("Received Ping at %v: %s (could not get peer info)", time.Now().Format(time.RFC3339Nano), in.Message)
	} else {
		log.Printf("Received Ping from %v at %v: %s", peerInfo.Addr, time.Now().Format(time.RFC3339Nano), in.Message)
	}
	return &pb.PongResponse{Message: "Pong"}, nil
}

func main() {
	// Set up keepalive parameters
	kaParams := keepalive.ServerParameters{
		MaxConnectionIdle:     15 * time.Second, // max idle time
		MaxConnectionAge:      30 * time.Second, // max age before forcing a shutdown
		MaxConnectionAgeGrace: 5 * time.Second,  // time to allow pending RPCs to complete before closing connections
		Time:                  10 * time.Second, // send pings every 10 seconds if there is activity
		Timeout:               2 * time.Second,  // wait 2 seconds for ping ack before considering the connection dead
	}

	lis, err := net.Listen("tcp", ":5001")
	if err != nil {
		log.Fatalf("Failed to listen on port 5001: %v", err)
	}
	grpcServer := grpc.NewServer(grpc.KeepaliveParams(kaParams))
	pb.RegisterPingPongServer(grpcServer, &server{})
	log.Println("gRPC server is listening on port 5001")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve gRPC server: %v", err)
	}
}
