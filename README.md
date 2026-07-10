# img_resize_service

## Run Server

### Not containerized and without Prometheus and Grafana

    go run ./server/main.go

### Containerized and with Prometheus and Grafana

    docker compose up

Can see Prometheus metrics at http://localhost:9090 and setup Grafana dashboards at http://localhost:3000.

## Run Client

Can use different protocols to connect to the server. By default, it uses connect. The loops parameter default is 1 and is only for stress testing.

### Connect HTTP

    go run client/resize_main.go --protocol=connect --loops=1

### Connect gRPC

    go run client/resize_main.go --protocol=grpc --loops=1

### Connect gRPC-web

    go run client/resize_main.go --protocol=grpcweb --loops=1

## Run Tests

Run tests:

    go test ./...

Run server tests:

    go test -v ./server/...
