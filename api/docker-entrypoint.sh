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
    
    # Smart ownership handling for performance optimization
    if [ "${SKIP_CHOWN_PACKAGES:-false}" = "true" ]; then
        echo "Skipping package ownership change (SKIP_CHOWN_PACKAGES=true)"
        # Only ensure base directory ownership
        chown coderunr:coderunr /coderunr
    elif [ -d /coderunr/packages ]; then
        # Check if packages directory needs ownership fix
        PACKAGES_OWNER=$(stat -c '%U' /coderunr/packages 2>/dev/null || echo "unknown")
        if [ "$PACKAGES_OWNER" != "coderunr" ]; then
            if [ ! -f /coderunr/.ownership_fixed ]; then
                echo "Fixing ownership of packages directory (owner: $PACKAGES_OWNER -> coderunr)"
                echo "This may take a moment for pre-built images..."
                chown -R coderunr:coderunr /coderunr
                touch /coderunr/.ownership_fixed
                echo "Ownership fix completed"
            else
                echo "Packages ownership already fixed (found .ownership_fixed marker)"
            fi
        else
            echo "Packages directory already has correct ownership (coderunr)"
            # Ensure base directory is correct
            chown coderunr:coderunr /coderunr
        fi
    else
        # For empty data directories, quick ownership fix
        echo "Setting up empty data directory ownership"
        chown -R coderunr:coderunr /coderunr
    fi
    
    # Switch to coderunr user and exec server
    exec su -- coderunr -c 'ulimit -n 65536 && server'
else
    # Running as non-root user
    echo "Starting CodeRunr server as non-root user..."
    ulimit -n 65536
    exec server
fi