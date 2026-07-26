// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collection // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver/internal/collection"

// osInfoCollection represents the os_info collection.
// https://github.com/osquery/osquery/blob/master/specs/os_version.table
type osInfoCollection struct{}

func (osInfoCollection) GetName() string {
	return osInfoCollectionName
}

func (osInfoCollection) GetQuery() string {
	return osInfoCollectionQuery
}

func (osInfoCollection) RowKey(map[string]string) string {
	return singletonRowKey
}

func newOSInfoCollection() Collection {
	return osInfoCollection{}
}
