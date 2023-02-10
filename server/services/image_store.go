package services

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/navruz-rakhimov/tages-project/protos"
)

type ImageStore interface {
	Save(imageName string, imageType string, imageData bytes.Buffer) (string, error)
	GetImagesInfoList() ([]*protos.ImageFullInfo, error)
}

type DiskImageStore struct {
	mutex       sync.RWMutex
	imageFolder string
	images      map[string]*ImageInfo
}

func NewDiskImageStore(imageFolder string) *DiskImageStore {
	return &DiskImageStore{
		imageFolder: imageFolder,
		images:      make(map[string]*ImageInfo),
	}
}

func (store *DiskImageStore) Save(imageName string, imageType string, imageData bytes.Buffer) (string, error) {
	imageID, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("cannot generate image id: %w", err)
	}

	imagePath := fmt.Sprintf("%s/%s", store.imageFolder, imageName)

	file, err := os.Create(imagePath)
	if err != nil {
		return "", fmt.Errorf("cannot create image file: %w", err)
	}

	_, err = imageData.WriteTo(file)
	if err != nil {
		return "", fmt.Errorf("cannot write image to file: %w", err)
	}

	store.mutex.Lock()
	defer store.mutex.Unlock()

	store.images[imageID.String()] = &ImageInfo{
		Type: imageType,
		Path: imagePath,
	}

	return imageID.String(), nil
}

func (store *DiskImageStore) GetImagesInfoList() ([]*protos.ImageFullInfo, error) {
	var (
		imageInfoList []*protos.ImageFullInfo
		createdAt     string
		updatedAt     string
	)

	fileInfos, err := ioutil.ReadDir(store.imageFolder)
	if err != nil {
		return nil, fmt.Errorf("cannot read dir: %w", err)
	}

	for _, fileInfo := range fileInfos {
		var fileTime = fileInfo.Sys().(*syscall.Win32FileAttributeData)
		var cTime = time.Unix(0, fileTime.CreationTime.Nanoseconds())
		var uTime = time.Unix(0, fileTime.LastAccessTime.Nanoseconds())

		createdAt = cTime.Format("2006-01-02 15:04:05")
		updatedAt = uTime.Format("2006-01-02 15:04:05")

		imageInfoList = append(imageInfoList, &protos.ImageFullInfo{
			ImageName: fileInfo.Name(),
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		})
	}

	return imageInfoList, nil
}

type ImageInfo struct {
	Type string
	Path string
}
