package main

import (
	"fileServer/server"
	"fmt"
	"log"
	"net"

	pb "fileServer/api"
	"google.golang.org/grpc"
)

func main() {
	listener, err := net.Listen("tcp", ":44041")
	if err != nil {
		log.Fatalf("Failed to listen on port 44041: %v", err)
	}

	s := grpc.NewServer()
	fileService := server.NewFileService()
	pb.RegisterFileServiceServer(s, fileService)

	fmt.Println("gRPC server listening on port 44041")
	if err := s.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
