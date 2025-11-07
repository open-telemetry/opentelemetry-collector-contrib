#!/bin/bash
set -e

# Configuration
REGISTRY=${DOCKER_REGISTRY:-"ghcr.io"}  # or docker.io for Docker Hub
USERNAME=${DOCKER_USERNAME:-"your-username"}
IMAGE_NAME="otel-collector-hydrolix"
VERSION=${VERSION:-"latest"}
FULL_IMAGE="${REGISTRY}/${USERNAME}/${IMAGE_NAME}:${VERSION}"

echo "=================================="
echo "Building Custom OTel Collector"
echo "=================================="
echo "Registry: $REGISTRY"
echo "Image: $FULL_IMAGE"
echo ""

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker and try again."
    exit 1
fi

# Build the Docker image
echo "📦 Building Docker image..."
docker build -f Dockerfile.custom -t "$FULL_IMAGE" .

echo "✅ Build complete!"
echo ""

# Ask if user wants to push
read -p "Push to registry? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]
then
    echo "🚀 Pushing to $REGISTRY..."

    # Login to registry (if not already logged in)
    if [[ $REGISTRY == "ghcr.io" ]]; then
        echo "Logging in to GitHub Container Registry..."
        echo "You'll need a GitHub Personal Access Token with 'write:packages' permission"
        echo "Create one at: https://github.com/settings/tokens"
        docker login ghcr.io -u "$USERNAME"
    elif [[ $REGISTRY == "docker.io" ]]; then
        echo "Logging in to Docker Hub..."
        docker login
    fi

    docker push "$FULL_IMAGE"
    echo "✅ Pushed successfully!"
else
    echo "Skipping push. You can push later with:"
    echo "  docker push $FULL_IMAGE"
fi

echo ""
echo "=================================="
echo "Next Steps"
echo "=================================="
echo "Update your Kubernetes deployment to use:"
echo "  image: $FULL_IMAGE"
echo ""
echo "If using a private registry, create an image pull secret:"
echo "  kubectl create secret docker-registry regcred \\"
echo "    --docker-server=$REGISTRY \\"
echo "    --docker-username=$USERNAME \\"
echo "    --docker-password=<your-token> \\"
echo "    -n github-runner"
echo ""
echo "Then add to your deployment:"
echo "  imagePullSecrets:"
echo "    - name: regcred"
