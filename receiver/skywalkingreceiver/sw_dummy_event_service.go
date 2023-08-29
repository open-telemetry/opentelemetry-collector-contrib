// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver"

import (
	event "skywalking.apache.org/repo/goapi/collect/event/v3"
)

type eventService struct {
	event.UnimplementedEventServiceServer
}

func (e *eventService) Collect(_ event.EventService_CollectServer) error {
	return nil
}
