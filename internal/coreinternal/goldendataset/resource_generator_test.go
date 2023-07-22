// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package goldendataset

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateResource(t *testing.T) {
	resourceIds := []PICTInputResource{ResourceEmpty, ResourceVMOnPrem, ResourceVMCloud, ResourceK8sOnPrem, ResourceK8sCloud, ResourceFaas, ResourceExec}
	for _, rscID := range resourceIds {
		rsc := GenerateResource(rscID)
		if rscID == ResourceEmpty {
			assert.Equal(t, 0, rsc.Attributes().Len())
		} else {
			assert.True(t, rsc.Attributes().Len() > 0)
		}
	}
}
