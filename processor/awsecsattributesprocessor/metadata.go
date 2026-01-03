package awsecsattributesprocessor

import (
	"fmt"
	"regexp"
	"time"
)

type Metadata struct {
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
	Name      string    `json:"Name"`
	Networks  []Network `json:"Networks"`
	Ports     []Port    `json:"Ports"`
	StartedAt time.Time `json:"StartedAt"`
	Type      string    `json:"Type"`
	Volumes   []Volume  `json:"Volumes"`
}

type Network struct {
	IPv4Addresses []string `json:"IPv4Addresses"`
	NetworkMode   string   `json:"NetworkMode"`
}

type Port struct {
	ContainerPort int    `json:"ContainerPort"`
	HostIP        string `json:"HostIp"`
	HostPort      int    `json:"HostPort"`
	Protocol      string `json:"Protocol"`
}

type Volume struct {
	Destination string `json:"Destination"`
	Source      string `json:"Source"`
}

var ecslabelsRe = regexp.MustCompile(`^com\.amazonaws\.ecs.*`)

func (m *Metadata) Flat() map[string]any {
	flattened := make(map[string]any)

	flattened["aws.ecs.container.arn"] = m.ContainerARN
	flattened["aws.ecs.task.known.status"] = m.KnownStatus
	flattened["aws.ecs.cluster"] = m.Labels["com.amazonaws.ecs.cluster"]
	flattened["aws.ecs.container.name"] = m.Labels["com.amazonaws.ecs.container-name"]
	flattened["aws.ecs.task.arn"] = m.Labels["com.amazonaws.ecs.task-arn"]
	flattened["aws.ecs.task.definition.family"] = m.Labels["com.amazonaws.ecs.task-definition-family"]
	flattened["aws.ecs.task.definition.version"] = m.Labels["com.amazonaws.ecs.task-definition-version"]
	flattened["created.at"] = m.CreatedAt.Format(time.RFC3339Nano)
	flattened["desired.status"] = m.DesiredStatus
	flattened["docker.id"] = m.DockerID
	flattened["docker.name"] = m.DockerName
	flattened["image"] = m.Image
	flattened["image.id"] = m.ImageID
	flattened["limits.cpu"] = m.Limits.CPU
	flattened["limits.memory"] = m.Limits.Memory
	flattened["name"] = m.Name
	flattened["started.at"] = m.StartedAt.Format(time.RFC3339Nano)
	flattened["type"] = m.Type

	// add networks
	for i, network := range m.Networks {
		flattened[fmt.Sprintf("networks.%d.network.mode", i)] = network.NetworkMode
		for ind, ipv4 := range network.IPv4Addresses {
			flattened[fmt.Sprintf("networks.%d.ipv4.addresses.%d", i, ind)] = ipv4
		}
	}

	// add ports
	for i, port := range m.Ports {
		flattened[fmt.Sprintf("ports.%d.container.port", i)] = port.ContainerPort
		flattened[fmt.Sprintf("ports.%d.host.ip", i)] = port.HostIP
		flattened[fmt.Sprintf("ports.%d.host.port", i)] = port.HostPort
		flattened[fmt.Sprintf("ports.%d.protocol", i)] = port.Protocol
	}

	// add volumes
	for i, volume := range m.Volumes {
		flattened[fmt.Sprintf("volumes.%d.destination", i)] = volume.Destination
		flattened[fmt.Sprintf("volumes.%d.source", i)] = volume.Source
	}

	// add user/non-ecs labels
	for key, value := range m.Labels {
		if !ecslabelsRe.MatchString(key) {
			flattened[fmt.Sprintf("labels.%s", key)] = value
		}
	}

	return flattened
}
