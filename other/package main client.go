package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "path/to/generated/protobuf/fileservice"
)

const serverAddress = "localhost:50051"

func uploadFile(client pb.FileServiceClient, filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Failed to open file: %v\n", err)
		return
	}
	defer file.Close()

	stream, err := client.UploadFile(context.Background())
	if err != nil {
		fmt.Printf("Failed to create upload stream: %v\n", err)
		return
	}

	buffer := make([]byte, 1024)
	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("Failed to read file: %v\n", err)
			return
		}

		req := &pb.UploadFileRequest{
			Filename: filePath,
			Content:  buffer[:n],
		}

		if err := stream.Send(req); err != nil {
			fmt.Printf("Failed to send file chunk: %v\n", err)
			return
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		fmt.Printf("Failed to receive upload response: %v\n", err)
		return
	}

	fmt.Printf("Upload response: %s\n", resp.Message)
}

func downloadFile(client pb.FileServiceClient, filename string, outputPath string) {
	stream, err := client.DownloadFile(context.Background(), &pb.DownloadFileRequest{Filename: filename})
	if err != nil {
		fmt.Printf("Failed to create download stream: %v\n", err)
		return
	}

	outputFile, err := os.Create(outputPath)
	if err != nil {
		fmt.Printf("Failed to create output file: %v\n", err)
		return
	}
	defer outputFile.Close()

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("Failed to receive file chunk: %v\n", err)
			return
		}

		_, err = outputFile.Write(resp.Content)
		if err != nil {
			fmt.Printf("Failed to write to output file: %v\n", err)
			return
		}
	}

	fmt.Printf("File downloaded successfully to %s\n", outputPath)
}

func listFiles(client pb.FileServiceClient) {
	resp, err := client.ListFiles(context.Background(), &pb.ListFilesRequest{})
	if err != nil {
		fmt.Printf("Failed to list files: %v\n", err)
		return
	}

	fmt.Println("Files on server:")
	for _, file := range resp.Files {
		fmt.Printf("Name: %s, Created At: %s, Updated At: %s\n",
			file.Filename, file.CreatedAt, file.UpdatedAt)
	}
}

func main() {
	conn, err := grpc.Dial(serverAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Printf("Failed to connect to server: %v\n", err)
		return
	}
	defer conn.Close()

	client := pb.NewFileServiceClient(conn)

	// Example usage of the client
	fmt.Println("Uploading file...")
	uploadFile(client, "example.txt")

	fmt.Println("Listing files...")
	listFiles(client)

	fmt.Println("Downloading file...")
	downloadFile(client, "example.txt", "downloaded_example.txt")
}
