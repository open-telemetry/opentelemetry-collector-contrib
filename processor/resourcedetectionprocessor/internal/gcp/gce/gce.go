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

// Package gce provides a detector that loads resource information from
// the GCE metatdata
package gce

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/gcp"
)

// TypeStr is type of detector.
const TypeStr = "gce"

var _ internal.Detector = (*Detector)(nil)

type Detector struct {
	metadata gcp.Metadata
}

func NewDetector(component.ProcessorCreateSettings, internal.DetectorConfig) (internal.Detector, error) {
	return &Detector{metadata: &gcp.MetadataImpl{}}, nil
}

func (d *Detector) Detect(context.Context) (resource pdata.Resource, schemaURL string, err error) {
	res := pdata.NewResource()

	if !d.metadata.OnGCE() {
		return res, "", nil
	}

	attr := res.Attributes()
	cloudErr := multierr.Combine(d.initializeCloudAttributes(attr)...)
	hostErr := multierr.Combine(d.initializeHostAttributes(attr)...)
	return res, conventions.SchemaURL, multierr.Append(cloudErr, hostErr)
}

func (d *Detector) initializeCloudAttributes(attr pdata.AttributeMap) []error {
	attr.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderGCP)
	attr.InsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformGCPComputeEngine)

	var errors []error

	projectID, err := d.metadata.ProjectID()
	if err != nil {
		errors = append(errors, err)
	} else {
		attr.InsertString(conventions.AttributeCloudAccountID, projectID)
	}

	zone, err := d.metadata.Zone()
	if err != nil {
		errors = append(errors, err)
	} else {
		attr.InsertString(conventions.AttributeCloudAvailabilityZone, zone)
	}

	return errors
}

func (d *Detector) initializeHostAttributes(attr pdata.AttributeMap) []error {
	var errors []error

	hostname, err := d.metadata.Hostname()
	if err != nil {
		errors = append(errors, err)
	} else {
		attr.InsertString(conventions.AttributeHostName, hostname)
	}

	instanceID, err := d.metadata.InstanceID()
	if err != nil {
		errors = append(errors, err)
	} else {
		attr.InsertString(conventions.AttributeHostID, instanceID)
	}

	hostType, err := d.metadata.Get("instance/machine-type")
	if err != nil {
		errors = append(errors, err)
	} else {
		attr.InsertString(conventions.AttributeHostType, hostType)
	}

	return errors
}
