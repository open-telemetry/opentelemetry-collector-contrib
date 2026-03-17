// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/xpdata/entity"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
)

func addEntityRefsToResource(resource pcommon.Resource) {
	entityRefs := entity.ResourceEntityRefs(resource)

	serviceRef := entityRefs.AppendEmpty()
	serviceRef.SetType("service")
	serviceRef.SetSchemaUrl("https://opentelemetry.io/schemas/1.21.0")
	serviceRef.IdKeys().Append(string(conventions.ServiceNameKey))
	serviceRef.DescriptionKeys().Append(string(conventions.ServiceVersionKey))

	if _, exists := resource.Attributes().Get(string(conventions.HostNameKey)); exists {
		hostRef := entityRefs.AppendEmpty()
		hostRef.SetType("host")
		hostRef.IdKeys().Append(string(conventions.HostNameKey))
	}
}

func SetupResourceWithEntityRefs(resource pcommon.Resource) {
	resource.Attributes().PutStr(string(conventions.ServiceNameKey), "my-service")
	resource.Attributes().PutStr(string(conventions.ServiceVersionKey), "1.0.0")
	resource.Attributes().PutStr(string(conventions.HostNameKey), "server-01")

	addEntityRefsToResource(resource)
}
