# Isolate build stage
FROM buildpack-deps:bookworm AS isolate
RUN apt-get update && \
    apt-get install -y --no-install-recommends git libcap-dev && \
    rm -rf /var/lib/apt/lists/* && \
    git clone https://github.com/envicutor/isolate.git /tmp/isolate/ && \
    cd /tmp/isolate && \
    git checkout af6db68042c3aa0ded80787fbb78bc0846ea2114 && \
    make -j$(nproc) install && \
    rm -rf /tmp/*

# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w -s' -o server ./cmd/server

# Final stage
FROM debian:bookworm-slim

ENV DEBIAN_FRONTEND=noninteractive

# Install runtime dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        bash \
        curl \
        libcap2-bin \
        libc6 \
        libpcre3 \
        libseccomp2 \
        libevent-2.1-7 \
        libgmp10 \
        libblas3 \
        liblapack3 \
        libfftw3-double3 \
        libglpk40 \
        libarpack2 \
        libreadline8 \
        libncurses6 \
        libedit2 \
        cgroup-tools \
        && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/* && \
    apt-get autoremove -y && \
    apt-get autoclean -y

# Copy isolate from build stage
COPY --from=isolate /usr/local/bin/isolate /usr/local/bin/isolate
COPY --from=isolate /usr/local/etc/isolate /usr/local/etc/isolate

# Create user for security
RUN useradd -M coderunr

# Create directories
RUN mkdir -p /coderunr/packages && \
    chown -R coderunr:coderunr /coderunr

# Copy binary and entrypoint from builder stage
COPY --from=builder /app/server /usr/local/bin/server
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

# Set permissions
RUN chmod +x /usr/local/bin/server && \
    chmod +x /usr/local/bin/docker-entrypoint.sh

# Set working directory
WORKDIR /coderunr

# Set environment variables
ENV CODERUNR_DATA_DIRECTORY=/coderunr
ENV CODERUNR_BIND_ADDRESS=0.0.0.0:2000
ENV CODERUNR_LOG_LEVEL=info

# Expose port
EXPOSE 2000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:2000/health || exit 1

# Run the server via entrypoint script
ENTRYPOINT ["docker-entrypoint.sh"]
