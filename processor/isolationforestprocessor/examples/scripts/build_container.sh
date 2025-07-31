#!/bin/bash

set -e

# Configuration
IMAGE_NAME="otelcontribcol-isolation-forest"
IMAGE_TAG="v0.131.0-$(git rev-parse --short HEAD)"
REGISTRY="your-registry.com"

echo "Building OpenTelemetry Collector container for x86_64..."

# Check if buildx is available
if ! docker buildx version >/dev/null 2>&1; then
    echo "❌ Docker buildx is required but not available"
    echo "Please install Docker buildx or update Docker to a newer version"
    exit 1
fi

# Create and use buildx builder if it doesn't exist
if ! docker buildx inspect multiarch-builder >/dev/null 2>&1; then
    echo "Creating buildx builder..."
    docker buildx create --name multiarch-builder --use
else
    docker buildx use multiarch-builder
fi

# Build for x86_64 (linux/amd64)
echo "Building container..."
docker buildx build \
    --platform linux/amd64 \
    --tag ${IMAGE_NAME}:${IMAGE_TAG} \
    --tag ${IMAGE_NAME}:latest \
    --load \
    .

echo "✅ Container built successfully!"
echo "📦 Image: ${IMAGE_NAME}:${IMAGE_TAG}"

# Test the container
echo "🧪 Testing container..."
if docker run --rm ${IMAGE_NAME}:${IMAGE_TAG} --help >/dev/null 2>&1; then
    echo "✅ Container test passed"
else
    echo "❌ Container test failed"
    exit 1
fi

echo "🎉 Build complete!"
echo ""
echo "📤 To push to registry:"
echo "  docker tag ${IMAGE_NAME}:${IMAGE_TAG} ${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"
echo "  docker push ${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"
echo "  docker push ${REGISTRY}/${IMAGE_NAME}:latest"
echo ""
echo "🚀 To run locally:"
echo "  docker run --rm -p 4317:4317 -p 4318:4318 -p 8888:8888 ${IMAGE_NAME}:${IMAGE_TAG} --config=/etc/otelcol/config.yaml"
