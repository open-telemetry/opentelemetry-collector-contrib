package awsecsattributesprocessor

import (
	"context"
	"fmt"
	"regexp"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

const (
	ecsContainerMetadataURIv4 = "ECS_CONTAINER_METADATA_URI_V4"
	ecsContainerMetadataURI   = "ECS_CONTAINER_METADATA_URI"
)

var ecsMetadataReg = regexp.MustCompile(fmt.Sprintf("(?:%s|%s)=(.*)",
	ecsContainerMetadataURI, ecsContainerMetadataURIv4))

type endpointsFn func(logger *zap.Logger, ctx context.Context) (map[string][]string, error)

// getEndpoints - retrieve ECS Metadata data endpoints via the Docker API
func getEndpoints(logger *zap.Logger, ctx context.Context) (map[string][]string, error) {
	m := make(map[string][]string)

	// Initialize Docker client
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return m, fmt.Errorf("failed to create Docker client: %w", err)
	}

	defer cli.Close() // if we don't close the client, we will leak connections

	// Get the list of all containers

	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return m, fmt.Errorf("failed to fetch Docker containers: %w", err)
	}

	logger.Debug("list containers", zap.Any("containers", containers))

	for _, container := range containers {
		// Fetch detailed container information
		containerInfo, err := cli.ContainerInspect(ctx, container.ID)
		if err != nil {
			return m, fmt.Errorf("failed to inspect Docker container %s: %w", container.ID, err)
		}

		logger.Debug("containerInfo", zap.String("container", container.ID), zap.Any("containerInfo.Config.Env", containerInfo.Config.Env))
		var endpoints []string
		for _, env := range containerInfo.Config.Env {

			// use regex to match ECS_CONTAINER_METADATA_URI_V4 and ECS_CONTAINER_METADATA_URI
			for _, n := range []string{ecsContainerMetadataURIv4, ecsContainerMetadataURI} {
				reg := regexp.MustCompile(fmt.Sprintf(`^%s=(.*$)`, n))
				if !reg.MatchString(env) {
					logger.Debug("skipping container does not match regex", zap.String("container", container.ID), zap.Any("env", env))

					continue
				}

				matches := reg.FindStringSubmatch(env)
				if len(matches) < 2 {
					logger.Debug("cannot find string submatch", zap.String("container", container.ID), zap.Any("env", env))
					continue
				}

				endpoints = append(endpoints, matches[1])
			}
		}

		// only record if there are endpoints
		if len(endpoints) > 0 {
			m[container.ID] = endpoints
		}
	}

	return m, nil
}

type ContainerData struct {
	Container types.Container
	Endpoints []string
}

type containerDataFn func(ctx context.Context) (map[string]ContainerData, error)

func getContainerId(rlog *plog.ResourceLogs, sources ...string) string {
	var id string
	for _, s := range sources {
		if v, ok := rlog.Resource().Attributes().Get(s); ok {
			id = v.AsString()
			break
		}
	}

	// strip any unneeded values for eg. file extension
	id = idReg.FindString(id)
	return id
}
