package services

import (
	"bytes"
	"context"
	"io"
	"log"

	pb "github.com/navruz-rakhimov/tages-project/protos"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	maxImageSize = 1 << 20
	port         = ":5001"
)

type ImageServer struct {
	pb.UnimplementedImageServiceServer
	imageStore             ImageStore
	readImageInfoSemaphore chan struct{}
	uploadImageSempahore   chan struct{}
}

func NewImageServer(imageStore ImageStore, maxReadConns int, maxUploadImageConns int) *ImageServer {
	return &ImageServer{
		imageStore:             imageStore,
		readImageInfoSemaphore: make(chan struct{}, maxReadConns),
		uploadImageSempahore:   make(chan struct{}, maxUploadImageConns),
	}
}

func (server *ImageServer) GetImageInfoList(ctx context.Context, _ *pb.Empty) (*pb.GetImageInfoListResponse, error) {
	select {
	case server.readImageInfoSemaphore <- struct{}{}:
		defer func() {
			<-server.readImageInfoSemaphore
		}()

		imageFullInfoList, err := server.imageStore.GetImagesInfoList()
		if err != nil {
			log.Fatal("Could get full image info list from image store: ", err)
			return nil, err
		}

		return &pb.GetImageInfoListResponse{
			ImageInfos: imageFullInfoList,
		}, nil

	default:
		return nil, status.Errorf(codes.ResourceExhausted, "maximum number of connections for UnaryRPC reached")
	}

}

func (server *ImageServer) UploadImage(stream pb.ImageService_UploadImageServer) error {
	select {
	case server.uploadImageSempahore <- struct{}{}:
		defer func() {
			<-server.uploadImageSempahore
		}()

		req, err := stream.Recv()
		if err != nil {
			return logError(status.Errorf(codes.Unknown, "cannot receive image info"))
		}

		imageType := req.GetInfo().GetImageType()
		imageName := req.GetInfo().GetImageName()
		log.Printf("receive an upload-image request for image with type %s", imageType)

		imageData := bytes.Buffer{}
		imageSize := 0

		for {
			log.Print("waiting to receive more data")

			req, err := stream.Recv()
			if err == io.EOF {
				log.Print("no more data")
				break
			}
			if err != nil {
				return logError(status.Errorf(codes.Unknown, "cannot receive chunk data: %v", err))
			}

			chunk := req.GetChunkData()
			size := len(chunk)

			log.Printf("received a chunk with size: %d", size)

			imageSize += size
			if imageSize > maxImageSize {
				return logError(status.Errorf(codes.InvalidArgument, "image is too large: %d > %d", imageSize, maxImageSize))
			}

			// write slowly
			// time.Sleep(time.Second)

			_, err = imageData.Write(chunk)
			if err != nil {
				return logError(status.Errorf(codes.Internal, "cannot write chunk data: %v", err))
			}
		}

		imageID, err := server.imageStore.Save(imageName, imageType, imageData)
		if err != nil {
			return logError(status.Errorf(codes.Internal, "cannot save image to the store: %v", err))
		}

		res := &pb.UploadImageResponse{
			Id:   imageID,
			Size: uint32(imageSize),
		}

		err = stream.SendAndClose(res)
		if err != nil {
			return logError(status.Errorf(codes.Unknown, "cannot send response: %v", err))
		}

		log.Printf("saved image with name: %s, size: %d", imageName, imageSize)
		return nil

	default:
		return status.Errorf(codes.ResourceExhausted, "maximum number of connections for StreamingRPC reached")
	}
}

func contextError(ctx context.Context) error {
	switch ctx.Err() {
	case context.Canceled:
		return logError(status.Error(codes.Canceled, "request is canceled"))
	case context.DeadlineExceeded:
		return logError(status.Error(codes.DeadlineExceeded, "deadline is exceeded"))
	default:
		return nil
	}
}

func logError(err error) error {
	if err != nil {
		log.Print(err)
	}
	return err
}
