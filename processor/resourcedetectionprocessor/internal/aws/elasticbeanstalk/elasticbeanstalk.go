// Copyright -c OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package elasticbeanstalk

import (
	"context"
	"encoding/json"
	"io"
	"strconv"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

const (
	// TypeStr is type of detector.
	TypeStr = "elastic_beanstalk"

	linuxPath   = "/var/elasticbeanstalk/xray/environment.conf"
	windowsPath = "C:\\Program Files\\Amazon\\XRay\\environment.conf"
)

var _ internal.Detector = (*Detector)(nil)

type Detector struct {
	fs fileSystem
}

type EbMetaData struct {
	DeploymentID    int    `json:"deployment_id"`
	EnvironmentName string `json:"environment_name"`
	VersionLabel    string `json:"version_label"`
}

func NewDetector(component.ProcessorCreateSettings, internal.DetectorConfig) (internal.Detector, error) {
	return &Detector{fs: &ebFileSystem{}}, nil
}

func (d Detector) Detect(context.Context) (resource pdata.Resource, schemaURL string, err error) {
	res := pdata.NewResource()
	var conf io.ReadCloser

	if d.fs.IsWindows() {
		conf, err = d.fs.Open(windowsPath)
	} else {
		conf, err = d.fs.Open(linuxPath)
	}

	// Do not want to return error so it fails silently on non-EB instances
	if err != nil {
		// TODO: Log a more specific message with zap
		return res, "", nil
	}

	ebmd := &EbMetaData{}
	err = json.NewDecoder(conf).Decode(ebmd)
	conf.Close()

	if err != nil {
		// TODO: Log a more specific error with zap
		return res, "", err
	}

	attr := res.Attributes()
	attr.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attr.InsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSElasticBeanstalk)
	attr.InsertString(conventions.AttributeServiceInstanceID, strconv.Itoa(ebmd.DeploymentID))
	attr.InsertString(conventions.AttributeDeploymentEnvironment, ebmd.EnvironmentName)
	attr.InsertString(conventions.AttributeServiceVersion, ebmd.VersionLabel)
	return res, conventions.SchemaURL, nil
}
