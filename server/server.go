package server

import (
	"context"
	"google.golang.org/grpc/codes"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	pb "fileServer/api"

	"google.golang.org/grpc/status"
)

const storagePath = "C:/Files"

type FileService struct {
	pb.UnimplementedFileServiceServer
	uploadLimit    chan struct{}
	downloadLimit  chan struct{}
	listFilesLimit chan struct{}
	mutex          sync.Mutex
}

func NewFileService() *FileService {
	return &FileService{
		uploadLimit:    make(chan struct{}, 10),
		downloadLimit:  make(chan struct{}, 10),
		listFilesLimit: make(chan struct{}, 100),
	}
}

func (s *FileService) UploadFile(stream pb.FileService_UploadFileServer) error {
	select {
	case s.uploadLimit <- struct{}{}:
		defer func() { <-s.uploadLimit }()
	default:
		return status.Errorf(codes.ResourceExhausted, "Too many upload requests")
	}

	var filename string
	var fileData []byte

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "Failed to receive file chunk: %v", err)
		}

		filename = req.Filename
		fileData = append(fileData, req.Filedata...)
	}

	filePath := filepath.Join(storagePath, filename)
	err := os.WriteFile(filePath, fileData, 0644)
	if err != nil {
		return status.Errorf(codes.Internal, "Failed to save file: %v", err)
	}

	return stream.SendAndClose(&pb.UploadFileResponse{
		Message: "File uploaded successfully",
	})
}

func (s *FileService) ListFiles(ctx context.Context, req *pb.ListFilesRequest) (*pb.ListFilesResponse, error) {
	select {
	case s.listFilesLimit <- struct{}{}:
		defer func() { <-s.listFilesLimit }()
	default:
		return nil, status.Errorf(codes.ResourceExhausted, "Too many requests")
	}

	files, err := os.ReadDir(storagePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to read files: %v", err)
	}

	var fileMetadata []*pb.FileMetadata
	for _, file := range files {
		if !file.IsDir() {
			info, _ := file.Info()
			fileMetadata = append(fileMetadata, &pb.FileMetadata{
				Filename:  file.Name(),
				CreatedAt: info.ModTime().Format(time.RFC3339),
				UpdatedAt: info.ModTime().Format(time.RFC3339),
			})
		}
	}
	return &pb.ListFilesResponse{Files: fileMetadata}, nil
}

func (s *FileService) DownloadFile(req *pb.DownloadFileRequest, stream pb.FileService_DownloadFileServer) error {
	select {
	case s.downloadLimit <- struct{}{}:
		defer func() { <-s.downloadLimit }()
	default:
		return status.Errorf(codes.ResourceExhausted, "Too many download requests")
	}

	filePath := filepath.Join(storagePath, req.Filename)
	file, err := os.Open(filePath)
	if err != nil {
		return status.Errorf(codes.NotFound, "File not found")
	}
	defer file.Close()

	buffer := make([]byte, 1024)
	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return status.Errorf(codes.Internal, "Failed to read file: %v", err)
		}
		if n == 0 {
			break
		}

		if err := stream.Send(&pb.DownloadFileResponse{Filedata: buffer[:n]}); err != nil {
			return status.Errorf(codes.Internal, "Failed to send file chunk: %v", err)
		}
	}
	return nil
}
