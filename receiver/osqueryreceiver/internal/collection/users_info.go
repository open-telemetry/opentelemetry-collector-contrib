// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collection // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver/internal/collection"

import "runtime"

var userCollectionQueryMap = map[string]string{
	"linux":   userCollectionQueryLinux,
	"darwin":  userCollectionQueryDarwin,
	"windows": userCollectionQueryWindows,
}

// userCollection represents the users_info collection.
type userCollection struct{}

func (userCollection) GetName() string {
	return userCollectionName
}

func (userCollection) GetQuery() string {
	return userCollectionQueryMap[runtime.GOOS]
}

func (userCollection) RowKey(row map[string]string) string {
	return row["username"]
}

func newUserCollection() Collection {
	return userCollection{}
}
