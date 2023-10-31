// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsinfo // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/ecsInfo"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	ecsAgentEndpoint         = "http://%s:51678/v1/metadata"
	ecsAgentTaskInfoEndpoint = "http://%s:51678/v1/tasks"
	taskStatusRunning        = "RUNNING"
	maxHTTPResponseLength    = 5 * 1024 * 1024 // 5MB
)

// There are two formats of ContainerInstance ARN (https://docs.aws.amazon.com/AmazonECS/latest/userguide/ecs-account-settings.html#ecs-resource-ids)
// arn:aws:ecs:region:aws_account_id:container-instance/container-instance-id
// arn:aws:ecs:region:aws_account_id:container-instance/cluster-name/container-instance-id
// This function will return "container-instance-id" for both ARN format

func GetContainerInstanceIDFromArn(arn string) (containerInstanceID string, err error) {
	// When splitting the ARN with ":", the 6th segments could be either:
	// container-instance/47c0ab6e-2c2c-475e-9c30-b878fa7a8c3d or
	// container-instance/cluster-name/47c0ab6e-2c2c-475e-9c30-b878fa7a8c3d
	err = nil
	if splitedList := strings.Split(arn, ":"); len(splitedList) >= 6 {
		// Further splitting tmpResult with "/", it could be splitted into either 2 or 3
		// Characters of "cluster-name" is only allowed to be letters, numbers and hyphens
		tmpResult := strings.Split(splitedList[5], "/")
		if len(tmpResult) == 2 {
			containerInstanceID = tmpResult[1]
			return
		} else if len(tmpResult) == 3 {
			containerInstanceID = tmpResult[2]
			return
		}
	}
	err = errors.New("Can't get ecs container instance id from ContainerInstance arn: " + arn)
	return

}

// Check the channel is closed or not.
func isClosed(ch <-chan bool) bool {
	select {
	case <-ch:
		return true
	default:
	}
	return false
}

type doer interface {
	Do(request *http.Request) (*http.Response, error)
}

func request(ctx context.Context, endpoint string, client doer) ([]byte, error) {
	resp, err := clientGet(ctx, endpoint, client)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("response status not OK: %d", resp.StatusCode)
	}
	if resp.ContentLength >= maxHTTPResponseLength {
		return nil, fmt.Errorf("get response with unexpected length from %s, response length: %d", endpoint, resp.ContentLength)
	}

	var reader io.Reader
	// value -1 indicates that the length is unknown, see https://golang.org/src/net/http/response.go
	// In this case, we read until the limit is reached
	// This might happen with chunked responses from ECS Introspection API
	if resp.ContentLength == -1 {
		reader = io.LimitReader(resp.Body, maxHTTPResponseLength)
	} else {
		reader = resp.Body
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("unable to read response body from %s, error: %w", endpoint, err)
	}

	if len(body) == maxHTTPResponseLength {
		return nil, fmt.Errorf("response from %s, execeeds the maximum length: %v", endpoint, maxHTTPResponseLength)
	}
	return body, nil

}

func clientGet(ctx context.Context, url string, client doer) (resp *http.Response, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}
