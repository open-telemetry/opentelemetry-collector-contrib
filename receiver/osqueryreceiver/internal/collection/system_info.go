// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collection // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver/internal/collection"

// systemInfoCollection represents the system_info collection.
// https://github.com/osquery/osquery/blob/master/specs/system_info.table
type systemInfoCollection struct{}

func (systemInfoCollection) GetName() string {
	return systemInfoCollectionName
}

func (systemInfoCollection) GetQuery() string {
	return systemInfoCollectionQuery
}

func (systemInfoCollection) RowKey(map[string]string) string {
	return singletonRowKey
}

func newSystemInfoCollection() Collection {
	return systemInfoCollection{}
}
