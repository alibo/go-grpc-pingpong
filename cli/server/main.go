// server.go
package main

import (
	"context"
	"log"
	"net"
	"time"

	pb "github.com/alibo/go-grpc-pingpong/pkg/pingpong"

	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedPingPongServer
}

func (s *server) Ping(ctx context.Context, in *pb.PingRequest) (*pb.PongResponse, error) {
	log.Printf("Received Ping at %v: %s", time.Now().Format(time.RFC3339Nano), in.Message)
	return &pb.PongResponse{Message: "Pong"}, nil
}

func main() {
	lis, err := net.Listen("tcp", ":5001")
	if err != nil {
		log.Fatalf("Failed to listen on port 5001: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterPingPongServer(grpcServer, &server{})
	log.Println("gRPC server is listening on port 5001")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve gRPC server: %v", err)
	}
}
