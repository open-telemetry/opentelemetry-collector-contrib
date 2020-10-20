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
package gce // import "cloud.google.com/go/compute/metadata"

import (
	"context"

	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

const (
	TypeStr = "gce"
)

var _ internal.Detector = (*Detector)(nil)

type Detector struct {
	metadata gceMetadata
}

func NewDetector() (internal.Detector, error) {
	return &Detector{metadata: &gceMetadataImpl{}}, nil
}

func (d *Detector) Detect(context.Context) (pdata.Resource, error) {
	res := pdata.NewResource()
	res.InitEmpty()

	if !d.metadata.OnGCE() {
		return res, nil
	}

	attr := res.Attributes()

	var errors []error
	errors = append(errors, d.initializeCloudAttributes(attr)...)
	errors = append(errors, d.initializeHostAttributes(attr)...)
	return res, componenterror.CombineErrors(errors)
}

func (d *Detector) initializeCloudAttributes(attr pdata.AttributeMap) []error {
	attr.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderGCP)

	var errors []error

	projectID, err := d.metadata.ProjectID()
	if err != nil {
		errors = append(errors, err)
	} else {
		attr.InsertString(conventions.AttributeCloudAccount, projectID)
	}

	zone, err := d.metadata.Zone()
	if err != nil {
		errors = append(errors, err)
	} else {
		attr.InsertString(conventions.AttributeCloudZone, zone)
	}

	return errors
}

func (d *Detector) initializeHostAttributes(attr pdata.AttributeMap) []error {
	var errors []error

	hostname, err := d.metadata.Hostname()
	if err != nil {
		errors = append(errors, err)
	} else {
		attr.InsertString(conventions.AttributeHostHostname, hostname)
	}

	instanceID, err := d.metadata.InstanceID()
	if err != nil {
		errors = append(errors, err)
	} else {
		attr.InsertString(conventions.AttributeHostID, instanceID)
	}

	name, err := d.metadata.InstanceName()
	if err != nil {
		errors = append(errors, err)
	} else {
		attr.InsertString(conventions.AttributeHostName, name)
	}

	hostType, err := d.metadata.Get("instance/machine-type")
	if err != nil {
		errors = append(errors, err)
	} else {
		attr.InsertString(conventions.AttributeHostType, hostType)
	}

	return errors
}
