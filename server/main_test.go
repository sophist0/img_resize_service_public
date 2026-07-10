package main

import (
	resizev1 "connectrpc_test/gen/resize/v1"
	"connectrpc_test/gen/resize/v1/resizev1connect"
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"connectrpc.com/connect"
	"github.com/davidbyttow/govips/v2/vips"
	"github.com/prometheus/client_golang/prometheus"
)

var test_img = "test_data/photo.png"

// Helper to query prometheus default gatherer for recorded metrics
func getMetricCount(protocol, status string) int {
	metricFamilies, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return 0
	}
	for _, mf := range metricFamilies {
		if mf.GetName() == "rpc_duration_seconds" {
			for _, m := range mf.GetMetric() {
				labels := m.GetLabel()
				matchProtocol := false
				matchStatus := false
				for _, l := range labels {
					if l.GetName() == "protocol" && l.GetValue() == protocol {
						matchProtocol = true
					}
					if l.GetName() == "status" && l.GetValue() == status {
						matchStatus = true
					}
				}
				if matchProtocol && matchStatus {
					if m.GetHistogram() != nil {
						return int(m.GetHistogram().GetSampleCount())
					}
				}
			}
		}
	}
	return 0
}

func TestMain(m *testing.M) {
	// Initialize govips
	vips.Startup(nil)

	// Check if test_data/photo.png exists
	if _, err := os.Stat(test_img); os.IsNotExist(err) {
		log.Fatalf("test_data/photo.png not found: %v", err)
	}

	// Run tests
	code := m.Run()

	// Clean up created files
	os.Remove("thumbnail.jpg")

	vips.Shutdown()
	os.Exit(code)
}

func TestNewMetricsInterceptor_Failure(t *testing.T) {
	interceptor := NewMetricsInterceptor()
	handler := interceptor(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		return nil, fmt.Errorf("mock error")
	})

	req := connect.NewRequest(&resizev1.ResizeRequest{
		// FilePath: "photo.png",
		FilePath: test_img,
	})

	initialCount := getMetricCount("unknown", "error")

	_, err := handler(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error from handler, got nil")
	}

	finalCount := getMetricCount("unknown", "error")
	if finalCount <= initialCount {
		t.Errorf("Expected error metric count to increase, got initial=%d, final=%d", initialCount, finalCount)
	}
}

func TestResizeServiceProtocols(t *testing.T) {
	// Set up handlers
	mux := http.NewServeMux()
	resize := &ResizeServer{}
	pathResize, handlerResize := resizev1connect.NewResizeServiceHandler(
		resize,
		connect.WithInterceptors(NewMetricsInterceptor()),
	)
	mux.Handle(pathResize, handlerResize)

	// Set up local test server using TLS for HTTP/2 support (required for standard gRPC client)
	server := httptest.NewTLSServer(mux)
	defer server.Close()

	tests := []struct {
		name     string
		protocol string
		opts     []connect.ClientOption
	}{
		{
			name:     "Connect protocol (HTTP)",
			protocol: "connect",
			opts:     nil, // defaults to Connect
		},
		{
			name:     "gRPC-Web protocol",
			protocol: "grpcweb",
			opts:     []connect.ClientOption{connect.WithGRPCWeb()},
		},
		{
			name:     "gRPC protocol",
			protocol: "grpc",
			opts:     []connect.ClientOption{connect.WithGRPC()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Create client with test server's TLS client configuration
			client := resizev1connect.NewResizeServiceClient(
				server.Client(),
				server.URL,
				tt.opts...,
			)

			// Make RPC call
			res, err := client.Resize(
				context.Background(),
				connect.NewRequest(&resizev1.ResizeRequest{
					FilePath:  test_img,
					NewHeight: 200,
					NewWidth:  200,
				}),
			)
			if err != nil {
				t.Fatalf("Resize call failed for protocol %s: %v", tt.protocol, err)
			}

			// Validate response payload
			if res.Msg.NewFilePath != "thumbnail.jpg" {
				t.Errorf("Expected NewFilePath to be 'thumbnail.jpg', got %q", res.Msg.NewFilePath)
			}

			// Verify thumbnail.jpg file was written and has contents
			fi, err := os.Stat("thumbnail.jpg")
			if err != nil {
				t.Errorf("Expected thumbnail.jpg to exist: %v", err)
			} else if fi.Size() == 0 {
				t.Errorf("Expected thumbnail.jpg to have non-zero size")
			}
		})
	}
}
