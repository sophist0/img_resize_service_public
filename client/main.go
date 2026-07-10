package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"connectrpc.com/connect"

	resizev1 "connectrpc_test/gen/resize/v1"
	"connectrpc_test/gen/resize/v1/resizev1connect"
)

func main() {
	protocolFlag := flag.String("protocol", "connect", "Protocol to use (connect, grpc, or grpcweb)")
	loopsFlag := flag.Int("loops", 1, "Number of times to call the Resize the image")

	flag.Parse()

	// Initialize a standard net/http client
	httpClient := &http.Client{}

	var opts []connect.ClientOption
	switch *protocolFlag {
	case "grpc":
		opts = append(opts, connect.WithGRPC())
	case "grpcweb":
		opts = append(opts, connect.WithGRPCWeb())
	}

	log.Printf("Using protocol: %s", *protocolFlag)

	// Create the generated Connect RPC client
	client := resizev1connect.NewResizeServiceClient(
		httpClient,
		"http://localhost:8080",
		opts...,
	)

	for i := 0; i < *loopsFlag; i++ {

		// Make the RPC call
		res, err := client.Resize(
			context.Background(),
			connect.NewRequest(&resizev1.ResizeRequest{
				FilePath:  "photo.png",
				NewHeight: 200,
				NewWidth:  200,
			}),
		)
		if err != nil {
			log.Fatalf("RPC call failed: %v", err)
		}

		log.Printf("Server responded: %s", res.Msg.NewFilePath)
	}
}
