FROM debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive

# Install essential packages for building and serving
RUN apt-get update && apt-get install -y \
    python3 \
    curl \
    jq \
    unzip \
    make \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Create directories
RUN mkdir -p /coderunr/packages /coderunr/repo

# Copy scripts
COPY entrypoint.sh mkindex.sh serve.py /

# Make scripts executable
RUN chmod +x /entrypoint.sh /mkindex.sh

WORKDIR /coderunr

EXPOSE 8000

ENTRYPOINT ["bash", "/entrypoint.sh"]
CMD ["--no-build"]
