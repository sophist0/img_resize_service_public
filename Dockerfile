# Builder Stage
FROM golang:1.25.0-bookworm AS builder

# Install build dependencies for libvips C-library/CGO
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    pkg-config \
    libvips-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy dependency manifests
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the server binary with CGO enabled
RUN CGO_ENABLED=1 go build -o /app/img_resize_server ./server/main.go

# Runtime Stage
FROM debian:bookworm-slim

# Install runtime dependencies for libvips
RUN apt-get update && apt-get install -y --no-install-recommends \
    libvips42 \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy the built binary
COPY --from=builder /app/img_resize_server /usr/local/bin/img_resize_server

# Copy photo.png since the code expects photo.png to be in the working directory
COPY --from=builder /app/photo.png /app/photo.png

# Set the default listening port environment variable
ENV LISTEN_ADDR=:8080
EXPOSE 8080

# Run the server
ENTRYPOINT ["/usr/local/bin/img_resize_server"]
