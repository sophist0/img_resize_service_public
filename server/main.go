package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/davidbyttow/govips/v2/vips"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	resizev1 "connectrpc_test/gen/resize/v1"
	"connectrpc_test/gen/resize/v1/resizev1connect"
)

/*
Note on protocols supported by Connect-RPC:
- "connect": A simple, REST-friendly protocol running over standard HTTP/1.1 or HTTP/2.
             Supports JSON or Protobuf payloads. Uses standard HTTP status codes.
- "grpc": The standard gRPC protocol running over HTTP/2. Requires HTTP Trailers.
- "grpcweb": Browser-compatible adaptation of gRPC. Encodes trailers inside the body.
*/

// Define Prometheus Histogram to measure RPC request duration.
var rpcDurationHistogram = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "rpc_duration_seconds",
		Help:    "Histogram of RPC latency in seconds, partitioned by protocol and status.",
		Buckets: prometheus.LinearBuckets(0, 0.01, 10),
	},
	[]string{"protocol", "status"},
)

func init() {
	prometheus.MustRegister(rpcDurationHistogram)
}

// NewMetricsInterceptor creates a Connect-RPC Unary Interceptor to collect response times.
func NewMetricsInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			duration := time.Since(start).Seconds()

			protocol := req.Peer().Protocol
			if protocol == "" {
				protocol = "unknown"
			}

			status := "success"
			if err != nil {
				status = "error"
			}

			rpcDurationHistogram.WithLabelValues(protocol, status).Observe(duration)
			return resp, err
		}
	}
}

// ResizeServer implements the generated ResizeServiceHandler interface.
type ResizeServer struct{}

func (s *ResizeServer) Resize(
	ctx context.Context,
	req *connect.Request[resizev1.ResizeRequest],
) (*connect.Response[resizev1.ResizeResponse], error) {
	log.Printf("Resize Request FilePath: %s", req.Msg.FilePath)
	log.Printf("Resize Request FilePath: %d", req.Msg.NewHeight)
	log.Printf("Resize Request FilePath: %d", req.Msg.NewWidth)

	image, err := vips.NewThumbnailFromFile(req.Msg.FilePath, int(req.Msg.NewWidth), int(req.Msg.NewHeight), vips.Interesting(vips.AlignCenter))
	if err != nil {
		log.Fatal(err)
	}

	buf, _, err := image.ExportJpeg(&vips.JpegExportParams{Quality: 85})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Saving as thumbnail.jpg")
	os.WriteFile("thumbnail.jpg", buf, 0644)
	fmt.Println("Saved thumbnail.jpg")

	res := connect.NewResponse(&resizev1.ResizeResponse{
		NewFilePath: "thumbnail.jpg",
	})
	return res, nil
}

func main() {
	vips.Startup(nil)
	defer vips.Shutdown()

	mux := http.NewServeMux()

	// ResizeServiceHandler returns the path pattern and the standard http.Handler
	resize := &ResizeServer{}
	pathResize, handlerResize := resizev1connect.NewResizeServiceHandler(
		resize,
		connect.WithInterceptors(NewMetricsInterceptor()),
	)
	mux.Handle(pathResize, handlerResize)

	// Expose the /metrics endpoint for Prometheus scraping
	mux.Handle("/metrics", promhttp.Handler())

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("Server starting on %s...", addr)

	// Use h2c so clients can use unencrypted HTTP/2 connections
	err := http.ListenAndServe(
		addr,
		h2c.NewHandler(mux, &http2.Server{}),
	)
	if err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
