// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collection // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver/internal/collection"

import "fmt"

// Collection is a predefined, named osquery query.
type Collection interface {
	GetName() string
	GetQuery() string
	// RowKey returns the value that identifies row within this collection's
	// result set across collection cycles, used to detect new vs. modified
	// rows. Collections whose query returns at most one row can return a
	// constant.
	RowKey(row map[string]string) string
}

// New returns the Collection registered under name, or an error if name is unknown.
func New(name string) (Collection, error) {
	switch name {
	case systemInfoCollectionName:
		return newSystemInfoCollection(), nil
	case packageInfoCollectionName:
		return newPackageInfoCollection(), nil
	case osInfoCollectionName:
		return newOSInfoCollection(), nil
	case secureBootCollectionName:
		return newSecureBootCollection(), nil
	case userCollectionName:
		return newUserCollection(), nil
	default:
		return nil, fmt.Errorf("unknown collection %q", name)
	}
}
