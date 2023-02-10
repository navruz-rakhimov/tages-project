package main

import (
	"fmt"
	"log"
	"net"
	"os"

	"github.com/navruz-rakhimov/tages-project/protos"
	"github.com/navruz-rakhimov/tages-project/server/services"
	"google.golang.org/grpc"
)

const (
	port = ":5001"
)

func main() {
	currentDir, _ := os.Getwd()
	fmt.Println(currentDir)
	tmpDir := currentDir + "\\server\\tmp"

	imageStore := services.NewDiskImageStore(tmpDir)
	imageServer := services.NewImageServer(imageStore)

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen %v", err)
	}

	s := grpc.NewServer()
	protos.RegisterImageServiceServer(s, imageServer)
	log.Printf("Sever listening at %v", lis.Addr())

	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
