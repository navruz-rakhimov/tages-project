package services

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/navruz-rakhimov/tages-project/protos"
	"google.golang.org/grpc"
)

type ImageClient struct {
	service protos.ImageServiceClient
}

// NewImageClient returns a new image client
func NewImageClient(cc *grpc.ClientConn) *ImageClient {
	service := protos.NewImageServiceClient(cc)
	return &ImageClient{service}
}

func (imageClient *ImageClient) GetAndStoreAllImageInfos() {

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	imagesInfoListResponse, err := imageClient.service.GetImageInfoList(ctx, &protos.Empty{})
	if err != nil {
		log.Fatal("failed to get image info list:", err)
		return
	}

	imagesInfoList := imagesInfoListResponse.GetImageInfos()
	bytes, err := json.Marshal(imagesInfoList)

	file, err := os.Create("./client/tmp/images_info.json")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	file.Write(bytes)
	file.Sync()
}

// UploadImage calls upload image RPC
func (imageClient *ImageClient) UploadImage(imagePath string) {
	_, imageName := filepath.Split(imagePath)

	file, err := os.Open(imagePath)
	if err != nil {
		log.Fatal("cannot open image file: ", err)
	}
	defer file.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := imageClient.service.UploadImage(ctx)
	if err != nil {
		log.Fatal("cannot upload image: ", err)
	}

	req := &protos.UploadImageRequest{
		Data: &protos.UploadImageRequest_Info{
			Info: &protos.ImageInfo{
				ImageType: filepath.Ext(imagePath),
				ImageName: imageName,
			},
		},
	}

	err = stream.Send(req)
	if err != nil {
		log.Fatal("cannot send image info to server: ", err, stream.RecvMsg(nil))
	}

	reader := bufio.NewReader(file)
	buffer := make([]byte, 1024)

	for {
		n, err := reader.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal("cannot read chunk to buffer: ", err)
		}

		req := &protos.UploadImageRequest{
			Data: &protos.UploadImageRequest_ChunkData{
				ChunkData: buffer[:n],
			},
		}

		err = stream.Send(req)
		if err != nil {
			log.Fatal("cannot send chunk to server: ", err)
		}
	}

	res, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatal("cannot receive response: ", err)
	}

	log.Printf("image uploaded with id: %s, size: %d", res.GetId(), res.GetSize())
}
