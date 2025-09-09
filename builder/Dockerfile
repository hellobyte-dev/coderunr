FROM ghcr.io/hellobyte-dev/coderunr/api:latest

# Bake pre-installed packages into the image
# Expect build context to contain the packages directory content
# Ensure correct ownership so entrypoint doesn't need to chown these at runtime
ADD --chown=coderunr:coderunr . /coderunr/packages/
