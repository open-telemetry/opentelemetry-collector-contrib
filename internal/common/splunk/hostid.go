// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package splunk

import (
	"fmt"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
)

// HostIDKey represents a host identifier.
type HostIDKey string

const (
	// AWS
	AWS HostIDKey = "AWSUniqueId"
	// GCP
	GCP HostIDKey = "gcp_id"
)

// HostID is a unique key and value (usually used as a dimension) to uniquely identify a host.
type HostID struct {
	// Key is the key name/type.
	Key HostIDKey
	// Value is the unique ID.
	ID string
}

// resourceAttributesToDimensions will return a set of dimension from the
// resource attributes, including a cloud host id (AWSUniqueId, gcp_id, etc.)
// if it can be constructed from the provided metadata.
func ResourceToHostID(res pdata.Resource) (HostID, bool) {
	var cloudAccount, region, hostID, provider string

	if attr, ok := res.Attributes().Get(conventions.AttributeCloudAccount); ok {
		cloudAccount = attr.StringVal()
	}
	if attr, ok := res.Attributes().Get(conventions.AttributeCloudRegion); ok {
		region = attr.StringVal()
	}
	if attr, ok := res.Attributes().Get(conventions.AttributeHostID); ok {
		hostID = attr.StringVal()
	}
	if attr, ok := res.Attributes().Get(conventions.AttributeCloudProvider); ok {
		provider = attr.StringVal()
	}

	switch provider {
	// TODO: Should these be defined as constants in resourcedetector module that we import or somewhere else?
	case "ec2":
		if hostID == "" || region == "" || cloudAccount == "" {
			break
		}
		return HostID{
			Key: AWS,
			ID:  fmt.Sprintf("%s_%s_%s", hostID, region, cloudAccount),
		}, true
	case "gce":
		if cloudAccount == "" || hostID == "" {
			break
		}
		return HostID{
			Key: GCP,
			ID:  fmt.Sprintf("%s_%s", cloudAccount, hostID),
		}, true
	}

	return HostID{}, false
}
