// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor"

import (
	"fmt"
	"regexp"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// containerMetadata is the container metadata document returned by the ECS task
// metadata endpoint (and the Docker per-container metadata endpoint).
// See https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-metadata-endpoint-v4.html
type containerMetadata struct {
	ContainerARN  string         `json:"ContainerARN"`
	CreatedAt     time.Time      `json:"CreatedAt"`
	DesiredStatus string         `json:"DesiredStatus"`
	DockerID      string         `json:"DockerId"`
	DockerName    string         `json:"DockerName"`
	Image         string         `json:"Image"`
	ImageID       string         `json:"ImageID"`
	KnownStatus   string         `json:"KnownStatus"`
	Labels        map[string]any `json:"Labels"`
	Limits        struct {
		CPU    int `json:"CPU"`
		Memory int `json:"Memory"`
	} `json:"Limits"`
	Name      string             `json:"Name"`
	Networks  []containerNetwork `json:"Networks"`
	Ports     []containerPort    `json:"Ports"`
	StartedAt time.Time          `json:"StartedAt"`
	Type      string             `json:"Type"`
	Volumes   []containerVolume  `json:"Volumes"`
}

// containerNetwork describes a container network interface reported by ECS.
type containerNetwork struct {
	IPv4Addresses []string `json:"IPv4Addresses"`
	NetworkMode   string   `json:"NetworkMode"`
}

// containerPort describes a published container port reported by ECS.
type containerPort struct {
	ContainerPort int    `json:"ContainerPort"`
	HostIP        string `json:"HostIp"`
	HostPort      int    `json:"HostPort"`
	Protocol      string `json:"Protocol"`
}

// containerVolume describes a container volume mount reported by ECS.
type containerVolume struct {
	Destination string `json:"Destination"`
	Source      string `json:"Source"`
}

// ecsLabelsRe matches the reserved ECS-managed Docker labels, which are promoted
// to dedicated aws.ecs.* attributes rather than emitted under the labels.* prefix.
var ecsLabelsRe = regexp.MustCompile(`^com\.amazonaws\.ecs.*`)

// flat returns the metadata as a flat key/value map suitable for use as resource
// attributes. ECS-managed labels are promoted to aws.ecs.* keys; any remaining
// (user-defined) labels are emitted under the labels.* prefix.
func (m *containerMetadata) flat() map[string]any {
	flattened := make(map[string]any)
	labels := m.Labels
	if labels == nil {
		labels = make(map[string]any)
	}

	// Attributes that map to OpenTelemetry semantic conventions.
	flattened[string(conventions.AWSECSContainerARNKey)] = m.ContainerARN
	flattened[string(conventions.AWSECSTaskARNKey)] = labels["com.amazonaws.ecs.task-arn"]
	flattened[string(conventions.AWSECSTaskFamilyKey)] = labels["com.amazonaws.ecs.task-definition-family"]
	flattened[string(conventions.AWSECSTaskRevisionKey)] = labels["com.amazonaws.ecs.task-definition-version"]
	flattened[string(conventions.ContainerIDKey)] = m.DockerID
	flattened[string(conventions.ContainerNameKey)] = m.Name
	flattened[string(conventions.ContainerImageNameKey)] = m.Image
	flattened[string(conventions.ContainerImageIDKey)] = m.ImageID

	// ECS-specific attributes that have no semantic-convention equivalent are
	// namespaced under aws.ecs.* to avoid polluting the top-level attribute space.
	flattened["aws.ecs.cluster"] = labels["com.amazonaws.ecs.cluster"]
	flattened["aws.ecs.container.name"] = labels["com.amazonaws.ecs.container-name"]
	flattened["aws.ecs.task.known_status"] = m.KnownStatus
	flattened["aws.ecs.task.desired_status"] = m.DesiredStatus
	flattened["aws.ecs.container.docker_name"] = m.DockerName
	flattened["aws.ecs.container.cpu_limit"] = m.Limits.CPU
	flattened["aws.ecs.container.memory_limit"] = m.Limits.Memory
	flattened["aws.ecs.container.type"] = m.Type

	// Timestamps are only emitted when present; ECS may omit them.
	if !m.CreatedAt.IsZero() {
		flattened["aws.ecs.container.created_at"] = m.CreatedAt.Format(time.RFC3339Nano)
	}
	if !m.StartedAt.IsZero() {
		flattened["aws.ecs.container.started_at"] = m.StartedAt.Format(time.RFC3339Nano)
	}

	// add networks
	for i, nw := range m.Networks {
		flattened[fmt.Sprintf("aws.ecs.container.network.%d.mode", i)] = nw.NetworkMode
		for ind, ipv4 := range nw.IPv4Addresses {
			flattened[fmt.Sprintf("aws.ecs.container.network.%d.ipv4_address.%d", i, ind)] = ipv4
		}
	}

	// add ports
	for i, p := range m.Ports {
		flattened[fmt.Sprintf("aws.ecs.container.port.%d.container_port", i)] = p.ContainerPort
		flattened[fmt.Sprintf("aws.ecs.container.port.%d.host_ip", i)] = p.HostIP
		flattened[fmt.Sprintf("aws.ecs.container.port.%d.host_port", i)] = p.HostPort
		flattened[fmt.Sprintf("aws.ecs.container.port.%d.protocol", i)] = p.Protocol
	}

	// add volumes
	for i, vol := range m.Volumes {
		flattened[fmt.Sprintf("aws.ecs.container.volume.%d.destination", i)] = vol.Destination
		flattened[fmt.Sprintf("aws.ecs.container.volume.%d.source", i)] = vol.Source
	}

	// add user-defined (non-ECS) labels under the container.label.* namespace
	for key, value := range labels {
		if !ecsLabelsRe.MatchString(key) {
			flattened[fmt.Sprintf("container.label.%s", key)] = value
		}
	}

	return flattened
}

// buildAttributes renders the metadata into a pcommon.Map once, so the enrichment
// hot path can copy the pre-built attributes onto each resource instead of
// re-flattening and stringifying the metadata on every telemetry item. Values
// that ECS did not provide (nil) or that render empty are omitted.
func (m *containerMetadata) buildAttributes() pcommon.Map {
	attrs := pcommon.NewMap()
	for k, v := range m.flat() {
		if v == nil {
			continue
		}
		s := fmt.Sprintf("%v", v)
		if s == "" {
			continue
		}
		attrs.PutStr(k, s)
	}
	return attrs
}
