package services

import (
	"bytes"
	"context"
	"io"
	"log"

	"golang.org/x/sync/semaphore"

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
	imageStore       ImageStore
	readImageInfoSem *semaphore.Weighted
	uploadImageSem   *semaphore.Weighted
}

func NewImageServer(imageStore ImageStore, maxReadConns int64, maxUploadImageConns int64) *ImageServer {
	return &ImageServer{
		imageStore:       imageStore,
		readImageInfoSem: semaphore.NewWeighted(maxReadConns),
		uploadImageSem:   semaphore.NewWeighted(maxUploadImageConns),
	}
}

func (server *ImageServer) GetImageInfoList(ctx context.Context, _ *pb.Empty) (*pb.GetImageInfoListResponse, error) {
	if err := server.readImageInfoSem.Acquire(context.Background(), 1); err != nil {
		return nil, err
	}
	defer func() {
		server.readImageInfoSem.Release(1)
	}()

	imageFullInfoList, err := server.imageStore.GetImagesInfoList()
	if err != nil {
		log.Fatal("Could get full image info list from image store: ", err)
		return nil, err
	}
	return &pb.GetImageInfoListResponse{
		ImageInfos: imageFullInfoList,
	}, nil
}

func (server *ImageServer) UploadImage(stream pb.ImageService_UploadImageServer) error {
	if err := server.uploadImageSem.Acquire(context.Background(), 1); err != nil {
		return err
	}
	defer func() {
		server.uploadImageSem.Release(1)
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
