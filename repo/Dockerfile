FROM debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive

# Install only essential packages for a web server
RUN apt-get update && apt-get install -y \
    python3 \
    python3-pip \
    curl \
    jq \
    unzip \
    && rm -rf /var/lib/apt/lists/*

# Create directories
RUN mkdir -p /coderunr/packages /coderunr/repo

# Copy scripts
ADD entrypoint.sh mkindex.sh serve.py /

# Make scripts executable
RUN chmod +x /entrypoint.sh /mkindex.sh

WORKDIR /coderunr

EXPOSE 8000

ENTRYPOINT ["bash", "/entrypoint.sh"]
CMD ["--no-build"]
