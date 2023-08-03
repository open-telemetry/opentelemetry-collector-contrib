// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticbeanstalk // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/elasticbeanstalk"

import (
	"context"
	"encoding/json"
	"io"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/elasticbeanstalk/internal/metadata"
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
	rb *metadata.ResourceBuilder
}

type EbMetaData struct {
	DeploymentID    int    `json:"deployment_id"`
	EnvironmentName string `json:"environment_name"`
	VersionLabel    string `json:"version_label"`
}

func NewDetector(_ processor.CreateSettings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	return &Detector{fs: &ebFileSystem{}, rb: metadata.NewResourceBuilder(cfg.ResourceAttributes)}, nil
}

func (d Detector) Detect(context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	var conf io.ReadCloser

	if d.fs.IsWindows() {
		conf, err = d.fs.Open(windowsPath)
	} else {
		conf, err = d.fs.Open(linuxPath)
	}

	// Do not want to return error so it fails silently on non-EB instances
	if err != nil {
		// TODO: Log a more specific message with zap
		return pcommon.NewResource(), "", nil
	}

	ebmd := &EbMetaData{}
	err = json.NewDecoder(conf).Decode(ebmd)
	conf.Close()

	if err != nil {
		// TODO: Log a more specific error with zap
		return pcommon.NewResource(), "", err
	}

	d.rb.SetCloudProvider(conventions.AttributeCloudProviderAWS)
	d.rb.SetCloudPlatform(conventions.AttributeCloudPlatformAWSElasticBeanstalk)
	d.rb.SetServiceInstanceID(strconv.Itoa(ebmd.DeploymentID))
	d.rb.SetDeploymentEnvironment(ebmd.EnvironmentName)
	d.rb.SetServiceVersion(ebmd.VersionLabel)

	return d.rb.Emit(), conventions.SchemaURL, nil
}
