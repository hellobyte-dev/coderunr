#!/bin/bash

CGROUP_FS="/sys/fs/cgroup"

# Check if cgroup v2 is available
if [ ! -e "$CGROUP_FS" ]; then
  echo "Cannot find $CGROUP_FS. Please make sure your system is using cgroup v2"
  exit 1
fi

if [ -e "$CGROUP_FS/unified" ]; then
  echo "Combined cgroup v1+v2 mode is not supported. Please make sure your system is using pure cgroup v2"
  exit 1
fi

if [ ! -e "$CGROUP_FS/cgroup.subtree_control" ]; then
  echo "Cgroup v2 not found. Please make sure cgroup v2 is enabled on your system"
  exit 1
fi

# Initialize cgroup for isolate (if running as root)
if [ "$(id -u)" = "0" ]; then
    echo "Setting up cgroup for isolate..."
    cd /sys/fs/cgroup && \
    mkdir -p isolate/ && \
    echo 1 > isolate/cgroup.procs && \
    echo '+cpuset +cpu +io +memory +pids' > cgroup.subtree_control && \
    cd isolate && \
    mkdir -p init && \
    echo 1 > init/cgroup.procs && \
    echo '+cpuset +memory' > cgroup.subtree_control
    echo "Cgroup initialized successfully"
    
    # Change ownership of data directory
    chown -R coderunr:coderunr /coderunr
    
    # Switch to coderunr user and exec server
    exec su -- coderunr -c 'ulimit -n 65536 && server'
else
    # Running as non-root user
    echo "Starting CodeRunr server as non-root user..."
    ulimit -n 65536
    exec server
fi