// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_aws_ecs) && !omit_detector_aws_ecs

package ecs

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
)

type retryMetadataProvider struct {
	*enrichmentMetadataProvider
	containerErr error
}

func (p *retryMetadataProvider) FetchContainerMetadata() (*ecsutil.ContainerMetadata, error) {
	return &ecsutil.ContainerMetadata{}, p.containerErr
}

func TestEC2MetadataStrictRetryDiscardsPartialAttributes(t *testing.T) {
	for _, failure := range []string{"enrichment", "container"} {
		t.Run(failure, func(t *testing.T) {
			d, a, p, _ := newEnrichmentDetector(t)
			d.failOnMissingMetadata = true
			provider := &retryMetadataProvider{enrichmentMetadataProvider: p}
			d.provider = provider
			if failure == "enrichment" {
				a.taskErr = errors.New("temporary lookup failure")
			} else {
				provider.containerErr = errors.New("temporary container failure")
			}
			_, _, err := d.Detect(t.Context())
			require.Error(t, err)

			// The retry sees a different task with only an instance ID. Neither
			// TMDE nor EC2 supplies the optional fields from the failed attempt.
			setTask(p, a, enrichmentOtherTask, enrichmentOtherInstance)
			p.task.AvailabilityZone = ""
			a.instance().PrivateDnsName = nil
			a.instance().InstanceType = ""
			a.instance().ImageId = nil
			a.instance().Placement = nil
			a.taskErr, provider.containerErr = nil, nil
			resource, _, err := d.Detect(t.Context())
			require.NoError(t, err)
			attrs := resource.Attributes().AsRaw()
			assert.Equal(t, enrichmentOtherTask, attrs["aws.ecs.task.arn"])
			assert.Equal(t, enrichmentOtherInstance, attrs["host.id"])
			for _, key := range []string{"host.name", "host.type", "host.image.id", "cloud.availability_zone"} {
				assert.NotContains(t, attrs, key)
			}
		})
	}
}
