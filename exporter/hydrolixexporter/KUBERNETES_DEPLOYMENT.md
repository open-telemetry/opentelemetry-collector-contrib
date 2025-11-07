# Deploying Hydrolix Exporter to Kubernetes

This guide explains how to build and deploy the custom OpenTelemetry Collector with the Hydrolix exporter to Kubernetes.

## Prerequisites

- Docker installed and running
- `kubectl` configured to access your Kubernetes cluster
- Access to a container registry (GitHub Container Registry, Docker Hub, or private registry)
- Hydrolix credentials

## Step 1: Build the Custom Collector Image

### Option A: Using the Build Script (Recommended)

```bash
cd /Users/murphy/repos/opentelemetry-collector-contrib

# Set your registry and username
export DOCKER_REGISTRY="ghcr.io"  # or "docker.io" for Docker Hub
export DOCKER_USERNAME="your-github-username"
export VERSION="v1.0.0"  # or "latest"

# Run the build script
./build-and-push.sh
```

The script will:
1. Build a Docker image with your Hydrolix exporter
2. Optionally push it to your registry
3. Show you the next steps

### Option B: Manual Build

```bash
cd /Users/murphy/repos/opentelemetry-collector-contrib

# Build the image
docker build -f Dockerfile.custom -t ghcr.io/your-username/otel-collector-hydrolix:latest .

# Push to registry
docker push ghcr.io/your-username/otel-collector-hydrolix:latest
```

## Step 2: Setup Container Registry Authentication

### For GitHub Container Registry (ghcr.io)

1. Create a Personal Access Token:
   - Go to https://github.com/settings/tokens
   - Create a token with `write:packages` permission

2. Login to GitHub Container Registry:
   ```bash
   echo $GITHUB_TOKEN | docker login ghcr.io -u your-username --password-stdin
   ```

3. Make your package public (optional):
   - Go to https://github.com/users/your-username/packages
   - Click on your package
   - Go to "Package settings" → "Change visibility" → "Public"

### For Private Registries

Create a Kubernetes secret:

```bash
kubectl create secret docker-registry regcred \
  --docker-server=ghcr.io \
  --docker-username=your-username \
  --docker-password=$GITHUB_TOKEN \
  -n github-runner
```

## Step 3: Create Hydrolix Credentials Secret

```bash
kubectl create secret generic hydrolix-credentials \
  --from-literal=endpoint="https://argus.hydrolix.live/ingest/event" \
  --from-literal=username="your-hydrolix-username" \
  --from-literal=password="your-hydrolix-password" \
  -n github-runner
```

## Step 4: Update and Deploy the Kubernetes Configuration

1. Edit `k8s-deployment-example.yaml`:
   - Update the `image:` field with your image name
   - If using a private registry, uncomment `imagePullSecrets`
   - Update Hydrolix credentials in the Secret

2. Apply the configuration:

```bash
kubectl apply -f k8s-deployment-example.yaml
```

## Step 5: Verify the Deployment

```bash
# Check if the pod is running
kubectl get pods -n github-runner

# View logs
kubectl logs -n github-runner deployment/runner-incident-monitor -f

# Check for successful metric exports (look for debug logs)
kubectl logs -n github-runner deployment/runner-incident-monitor | grep -i hydrolix
```

Expected log output:
```
successfully sent metrics to Hydrolix (metric_count=15, table=cicd.metrics)
```

## Troubleshooting

### Image Pull Errors

If you see `ImagePullBackOff` or `ErrImagePull`:

1. **Public registry**: Make sure your package is public
2. **Private registry**: Verify imagePullSecrets is configured
3. **Image name**: Double-check the image name and tag

```bash
# Check image pull events
kubectl describe pod <pod-name> -n github-runner

# Test pulling the image locally
docker pull ghcr.io/your-username/otel-collector-hydrolix:latest
```

### Exporter Errors

Check the logs for error messages:

```bash
kubectl logs -n github-runner deployment/runner-incident-monitor | grep -i error
```

Common errors:
- **401/403**: Wrong Hydrolix credentials
- **404**: Wrong endpoint, table, or transform name
- **Connection refused**: Network/firewall issues

### Verifying the Exporter is Being Used

```bash
# The config should show "hydrolix" in exporters
kubectl get configmap otel-incident-config -n github-runner -o yaml

# Check that the custom image is being used
kubectl get deployment runner-incident-monitor -n github-runner -o jsonpath='{.spec.template.spec.containers[0].image}'
```

## Development Workflow

### Quick Iteration Cycle

1. Make changes to the exporter code
2. Rebuild and push:
   ```bash
   ./build-and-push.sh
   ```
3. Force pod restart:
   ```bash
   kubectl rollout restart deployment/runner-incident-monitor -n github-runner
   ```
4. Watch logs:
   ```bash
   kubectl logs -n github-runner deployment/runner-incident-monitor -f
   ```

### Using Latest Tag During Development

If using `imagePullPolicy: Always` with `latest` tag:
- Pod will always pull the newest image on restart
- Good for development
- Not recommended for production (use versioned tags)

### Testing Locally Before Deploying

Test your collector locally with the same config:

```bash
# Save the ConfigMap config to a file
kubectl get configmap otel-incident-config -n github-runner -o jsonpath='{.data.config\.yaml}' > test-config.yaml

# Run locally with Docker
docker run -it --rm \
  -v $(pwd)/test-config.yaml:/etc/otel/config.yaml \
  -e HYDROLIX_ENDPOINT="https://argus.hydrolix.live/ingest/event" \
  -e HYDROLIX_USERNAME="your-username" \
  -e HYDROLIX_PASSWORD="your-password" \
  ghcr.io/your-username/otel-collector-hydrolix:latest \
  --config /etc/otel/config.yaml
```

## Configuration Reference

### Hydrolix Exporter Config

```yaml
exporters:
  hydrolix:
    endpoint: https://argus.hydrolix.live/ingest/event  # Hydrolix ingest endpoint
    hdx_table: cicd.metrics                              # Target table
    hdx_transform: tf_cicd_metrics_v2                    # Transform name
    hdx_username: ${HYDROLIX_USERNAME}                   # From env var or secret
    hdx_password: ${HYDROLIX_PASSWORD}                   # From env var or secret
    timeout: 30s                                         # HTTP timeout
```

### Multiple Exporters

You can export to multiple destinations:

```yaml
exporters:
  hydrolix/metrics:
    endpoint: https://argus.hydrolix.live/ingest/event
    hdx_table: cicd.metrics
    hdx_transform: tf_cicd_metrics_v2
    # ...

  hydrolix/logs:
    endpoint: https://argus.hydrolix.live/ingest/event
    hdx_table: cicd.logs
    hdx_transform: tf_cicd_logs
    # ...

  debug:
    verbosity: detailed

service:
  pipelines:
    metrics:
      receivers: [kubeletstats]
      processors: [batch]
      exporters: [hydrolix/metrics, debug]
```

## Production Considerations

1. **Use versioned tags** instead of `latest`
   ```yaml
   image: ghcr.io/your-username/otel-collector-hydrolix:v1.0.0
   imagePullPolicy: IfNotPresent
   ```

2. **Set resource limits** appropriately
   ```yaml
   resources:
     limits:
       memory: 256Mi
       cpu: 200m
     requests:
       memory: 128Mi
       cpu: 100m
   ```

3. **Enable retry and queuing** for reliability
   ```yaml
   exporters:
     hydrolix:
       # ... other config
       retry_on_failure:
         enabled: true
         initial_interval: 5s
         max_interval: 30s
         max_elapsed_time: 5m
       sending_queue:
         enabled: true
         num_consumers: 10
         queue_size: 1000
   ```

4. **Monitor the collector** itself
   ```yaml
   service:
     telemetry:
       logs:
         level: info
       metrics:
         level: detailed  # Enable collector metrics
         address: :8888
   ```

5. **Use health checks**
   ```yaml
   livenessProbe:
     httpGet:
       path: /
       port: 13133
     initialDelaySeconds: 30
     periodSeconds: 10
   readinessProbe:
     httpGet:
       path: /
       port: 13133
     initialDelaySeconds: 5
     periodSeconds: 10
   ```

## Additional Resources

- [OpenTelemetry Collector Configuration](https://opentelemetry.io/docs/collector/configuration/)
- [Kubernetes Secrets](https://kubernetes.io/docs/concepts/configuration/secret/)
- [GitHub Container Registry](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
