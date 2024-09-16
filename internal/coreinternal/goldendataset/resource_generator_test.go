// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package goldendataset

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateResource(t *testing.T) {
	resourceIDs := []PICTInputResource{ResourceEmpty, ResourceVMOnPrem, ResourceVMCloud, ResourceK8sOnPrem, ResourceK8sCloud, ResourceFaas, ResourceExec}
	for _, rscID := range resourceIDs {
		rsc := GenerateResource(rscID)
		if rscID == ResourceEmpty {
			assert.Equal(t, 0, rsc.Attributes().Len())
		} else {
			assert.Positive(t, rsc.Attributes().Len())
		}
	}
}
