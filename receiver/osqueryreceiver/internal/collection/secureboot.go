// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collection // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver/internal/collection"

// secureBootCollection represents the secureboot_info collection.
// https://github.com/osquery/osquery/blob/master/specs/secureboot.table
type secureBootCollection struct{}

func (secureBootCollection) GetName() string {
	return secureBootCollectionName
}

func (secureBootCollection) GetQuery() string {
	return secureBootCollectionQuery
}

func newSecureBootCollection() Collection {
	return secureBootCollection{}
}
