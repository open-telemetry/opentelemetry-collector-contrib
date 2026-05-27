// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsdevicepodcorrelationprocessor/internal/kubelet"

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
)

const (
	DefaultSocketPath      = "/var/lib/kubelet/pod-resources/kubelet.sock"
	connectionTimeout      = 10 * time.Second
	defaultRefreshInterval = 10 * time.Second
)

// ContainerInfo holds Kubernetes pod/container metadata for a device.
type ContainerInfo struct {
	PodName       string
	ContainerName string
	Namespace     string
}

// Client connects to the Kubelet Pod Resources API and maintains
// an in-memory cache of device-to-pod mappings.
type Client struct {
	conn            *grpc.ClientConn
	listerClient    podresourcesapi.PodResourcesListerClient
	resourceNames   map[string]struct{}
	mu              sync.RWMutex
	deviceToPod     map[deviceKey]ContainerInfo
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	socketPath      string
	refreshInterval time.Duration
	logger          *zap.Logger
}

type deviceKey struct {
	DeviceID     string
	ResourceName string
}

// ClientOption configures the Client.
type ClientOption func(*Client)

// WithSocketPath sets a custom kubelet socket path.
func WithSocketPath(path string) ClientOption {
	if path == "" {
		path = DefaultSocketPath
	}
	return func(c *Client) { c.socketPath = path }
}

// NewClient creates a new Kubelet Pod Resources API client.
func NewClient(logger *zap.Logger, opts ...ClientOption) *Client {
	c := &Client{
		socketPath:      DefaultSocketPath,
		refreshInterval: defaultRefreshInterval,
		resourceNames:   make(map[string]struct{}),
		deviceToPod:     make(map[deviceKey]ContainerInfo),
		logger:          logger,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Start connects to the kubelet socket and begins periodic polling.
func (c *Client) Start(ctx context.Context) error {
	conn, err := grpc.NewClient("passthrough:"+c.socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "unix", addr)
		}),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to kubelet pod resources socket %s: %w", c.socketPath, err)
	}
	c.conn = conn
	c.listerClient = podresourcesapi.NewPodResourcesListerClient(conn)

	pollCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(1)
	go c.pollLoop(pollCtx)
	return nil
}

// Stop cancels the polling loop, waits for it to exit, and closes the gRPC connection.
func (c *Client) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	c.wg.Wait()
	if c.conn != nil {
		c.conn.Close()
	}
}

// AddResourceName registers a Kubernetes extended resource name to track.
func (c *Client) AddResourceName(resourceName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.resourceNames[resourceName] = struct{}{}
}

// GetContainerInfo looks up the pod/container that owns the given device.
func (c *Client) GetContainerInfo(deviceID, resourceName string) *ContainerInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key := deviceKey{DeviceID: deviceID, ResourceName: resourceName}
	if info, ok := c.deviceToPod[key]; ok {
		return &info
	}
	return nil
}

func (c *Client) pollLoop(ctx context.Context) {
	defer c.wg.Done()
	ticker := time.NewTicker(c.refreshInterval)
	defer ticker.Stop()

	c.refresh(ctx)

	for {
		select {
		case <-ticker.C:
			c.refresh(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) refresh(ctx context.Context) {
	// Snapshot resourceNames under read lock to avoid racing with AddResourceName.
	c.mu.RLock()
	trackedResources := make(map[string]struct{}, len(c.resourceNames))
	for k, v := range c.resourceNames {
		trackedResources[k] = v
	}
	c.mu.RUnlock()

	if len(trackedResources) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, connectionTimeout)
	defer cancel()

	resp, err := c.listerClient.List(ctx, &podresourcesapi.ListPodResourcesRequest{})
	if err != nil {
		c.logger.Error("Failed to list pod resources from kubelet", zap.Error(err))
		return
	}

	newMap := make(map[deviceKey]ContainerInfo)
	for _, pod := range resp.GetPodResources() {
		for _, container := range pod.GetContainers() {
			for _, device := range container.GetDevices() {
				if _, tracked := trackedResources[device.GetResourceName()]; !tracked {
					continue
				}
				info := ContainerInfo{
					PodName:       pod.GetName(),
					Namespace:     pod.GetNamespace(),
					ContainerName: container.GetName(),
				}
				for _, deviceID := range device.GetDeviceIds() {
					newMap[deviceKey{DeviceID: deviceID, ResourceName: device.GetResourceName()}] = info
				}
			}
		}
	}

	c.mu.Lock()
	c.deviceToPod = newMap
	c.mu.Unlock()
}
