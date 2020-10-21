package ecs

import (
	"encoding/json"
	"log"
	"net/http"
)

type TaskMetaData struct {
	Cluster string
	LaunchType string  // TODO: Change to enum when defined in otel collector conventions
	TaskARN string
	Family string
	AvailabilityZone string
	Containers []Container
}

type Container struct {
	DockerId string
	ContainerARN string
	Type string
	KnownStatus string
	LogDriver string
	LogOptions LogData
}

type LogData struct {
	LogGroup string `json:"awslogs-group"`
	Region string `json:"awslogs-region"`
	Stream string `json:"awslogs-stream"`
}

// Retrieves the metadata for a task running on Amazon ECS
func fetchTaskMetaData(tmde string) (*TaskMetaData, error) {
	ret, err := fetch(tmde + "/task", true)
	return ret.(*TaskMetaData), err
}

// Retrieves the metadata for the Amazon ECS Container the collector is running on
func fetchContainerMetaData(tmde string) (*Container, error) {
	ret, err := fetch(tmde, false)
	return ret.(*Container), err
}

func fetch(tmde string, task bool) (tmdeResp interface{}, err error) {
	resp, err := http.Get(tmde)

	if err != nil {
		log.Panicf("Received error from ECS Task Metadata Endpoint: %v", err)
		return nil, err
	}

	if task {
		tmdeResp = &TaskMetaData{}
	} else {
		tmdeResp = &Container{}
	}

	err = json.NewDecoder(resp.Body).Decode(tmdeResp)
	defer resp.Body.Close()

	if err != nil {
		log.Panicf("Encountered unexpected error reading response from ECS Task Metadata Endpoint: %v", err)
		return nil, err
	}

	return tmdeResp, nil
}
