package main

import (
	"log"

	"github.com/navruz-rakhimov/tages-project/client/services"
	"google.golang.org/grpc"
)

const (
	address = "localhost:5001"
)

func main() {
	log.Printf("dial server %s", address)

	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatal("cannot dial server: ", err)
	}
	defer conn.Close()

	imageClient := services.NewImageClient(conn)
	imageClient.UploadImage("./client/tmp/panda.jpg")

	imageClient.GetAndStoreAllImageInfos()
}
