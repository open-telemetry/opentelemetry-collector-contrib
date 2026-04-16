// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package constants

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProviderFromResourceID(t *testing.T) {
	tests := []struct {
		name       string
		resourceID string
		want       string
	}{
		{
			name:       "Microsoft.Web uppercase",
			resourceID: "/SUBSCRIPTIONS/AAAA0A0A-BB1B-CC2C-DD3D-EEEEEE4E4E4E/RESOURCEGROUPS/RG-001/PROVIDERS/MICROSOFT.WEB/SITES/SCALEABLEWEBAPP1",
			want:       "azure.web",
		},
		{
			name:       "Microsoft.Compute uppercase",
			resourceID: "/SUBSCRIPTIONS/AAAA0A0A-BB1B-CC2C-DD3D-EEEEEE4E4E4E/RESOURCEGROUPS/RG-001/PROVIDERS/MICROSOFT.COMPUTE/VIRTUALMACHINES/VM1",
			want:       "azure.compute",
		},
		{
			name:       "microsoft.keyvault lowercase",
			resourceID: "/subscriptions/aaaa0a0a-bb1b-cc2c-dd3d-eeeeee4e4e4e/resourcegroups/rg-dcrs/providers/microsoft.keyvault/vaults/dcr-vault",
			want:       "azure.keyvault",
		},
		{
			name:       "Microsoft.Storage mixed case",
			resourceID: "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Storage/storageAccounts/sa1",
			want:       "azure.storage",
		},
		{
			name:       "Microsoft.Network",
			resourceID: "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Network/applicationGateways/gw1",
			want:       "azure.network",
		},
		{
			name:       "non-Microsoft provider",
			resourceID: "/subscriptions/sub-1/resourceGroups/rg-1/providers/Sendgrid.Email/accounts/sg1",
			want:       "azure.sendgrid.email",
		},
		{
			name:       "empty string",
			resourceID: "",
			want:       "azure.generic",
		},
		{
			name:       "no providers segment",
			resourceID: "/subscriptions/sub-1/resourceGroups/rg-1",
			want:       "azure.generic",
		},
		{
			name:       "providers at end with no value after",
			resourceID: "/subscriptions/sub-1/resourceGroups/rg-1/providers",
			want:       "azure.generic",
		},
		{
			name:       "providers followed by trailing slash only",
			resourceID: "/subscriptions/sub-1/resourceGroups/rg-1/providers/",
			want:       "azure.generic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProviderFromResourceID(tt.resourceID)
			assert.Equal(t, tt.want, got)
		})
	}
}
