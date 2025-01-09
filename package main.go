package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"google.golang.org/grpc"
	// "google.golang.org/grpc/credentials/insecure"
	// "google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	pb "golang/file_service"
)

const (
	storageDir           = "./storage"
	maxUploadConnections = 10
	maxListConnections   = 100
)

type server struct {
	pb.UnimplementedFileServiceServer
	uploadSem chan struct{}
	listSem   chan struct{}
	mu        sync.Mutex
}

func NewServer() *server {
	return &server{
		uploadSem: make(chan struct{}, maxUploadConnections),
		listSem:   make(chan struct{}, maxListConnections),
	}
}

// UploadFile handles file uploads
func (s *server) UploadFile(stream pb.FileService_UploadFileServer) error {
	s.uploadSem <- struct{}{}
	defer func() { <-s.uploadSem }()

	var filename string
	var fileBuffer bytes.Buffer

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(500, "failed to receive file: %v", err)
		}

		filename = req.Filename
		fileBuffer.Write(req.Content)
	}

	if filename == "" {
		return status.Error(400, "filename is required")
	}

	filePath := filepath.Join(storageDir, filename)
	err := os.WriteFile(filePath, fileBuffer.Bytes(), 0644)
	if err != nil {
		return status.Errorf(500, "failed to save file: %v", err)
	}

	return stream.SendAndClose(&pb.UploadFileResponse{
		Message: "File uploaded successfully",
	})
}

// DownloadFile handles file downloads
func (s *server) DownloadFile(req *pb.DownloadFileRequest, stream pb.FileService_DownloadFileServer) error {
	s.uploadSem <- struct{}{}
	defer func() { <-s.uploadSem }()

	filePath := filepath.Join(storageDir, req.Filename)
	file, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return status.Error(404, "file not found")
		}
		return status.Errorf(500, "failed to open file: %v", err)
	}
	defer file.Close()

	buffer := make([]byte, 1024)
	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(500, "failed to read file: %v", err)
		}

		if err := stream.Send(&pb.DownloadFileResponse{
			Content: buffer[:n],
		}); err != nil {
			return status.Errorf(500, "failed to send file chunk: %v", err)
		}
	}

	return nil
}

// ListFiles lists all uploaded files
func (s *server) ListFiles(ctx context.Context, req *pb.ListFilesRequest) (*pb.ListFilesResponse, error) {
	s.listSem <- struct{}{}
	defer func() { <-s.listSem }()

	files, err := os.ReadDir(storageDir)
	if err != nil {
		return nil, status.Errorf(500, "failed to list files: %v", err)
	}

	var fileInfos []*pb.FileInfo
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			return nil, status.Errorf(500, "failed to get file info: %v", err)
		}

		fileInfos = append(fileInfos, &pb.FileInfo{
			Filename:  file.Name(),
			CreatedAt: info.ModTime().Format(time.RFC3339),
			UpdatedAt: info.ModTime().Format(time.RFC3339),
		})
	}

	return &pb.ListFilesResponse{Files: fileInfos}, nil
}

func main() {
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		fmt.Printf("Failed to create storage directory: %v\n", err)
		return
	}

	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		fmt.Printf("Failed to listen: %v\n", err)
		return
	}

	grpcServer := grpc.NewServer()
	pb.RegisterFileServiceServer(grpcServer, NewServer())

	fmt.Println("Server is running on port 50051...")
	if err := grpcServer.Serve(listener); err != nil {
		fmt.Printf("Failed to serve: %v\n", err)
	}
}
