// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudlogsreceiver/internal"

import (
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/lts/v2/model"
)

// The functions of the following interface should have the same signature as the ones from https://github.com/huaweicloud/huaweicloud-sdk-go-v3/blob/v0.1.110/services/lts/v2/lts_client.go
// Check https://github.com/vektra/mockery on how to install it on your machine.
//
//go:generate mockery --name LtsClient --case=underscore --output=./mocks
type LtsClient interface {
	ListLogs(request *model.ListLogsRequest) (*model.ListLogsResponse, error)
}
