// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
)

func TestLookupCNAME_Linux(t *testing.T) {
	p := fakeLinuxSystemMetadataProvider()
	cname, err := p.LookupCNAME()
	require.NoError(t, err)
	assert.Equal(t, "my-linux-vm.abcdefghijklmnopqrstuvwxyz.xx.internal.foo.net", cname)
}

func TestLookupCNAME_Windows(t *testing.T) {
	p := fakeWindowsSystemMetadataProvider()
	cname, err := p.LookupCNAME()
	require.NoError(t, err)
	assert.Equal(t, "my-windows-vm.abcdefghijklmnopqrstuvwxyz.xx.internal.foo.net", cname)
}

func TestReverseLookupHost_Linux(t *testing.T) {
	p := fakeLinuxSystemMetadataProvider()
	fqdn, err := p.ReverseLookupHost()
	require.NoError(t, err)
	assert.Equal(t, "my-linux-vm.internal.foo.net", fqdn)
}

func TestReverseLookupHost_Windows(t *testing.T) {
	p := fakeWindowsSystemMetadataProvider()
	fqdn, err := p.ReverseLookupHost()
	require.NoError(t, err)
	assert.Equal(t, "my-windows-vm.abcdefghijklmnopqrstuvwxyz.xx.internal.foo.net", fqdn)
}

func TestHostID(t *testing.T) {
	tests := []struct {
		name         string
		resValue     string
		resError     error
		fakeResource func(context.Context, ...resource.Option) (*resource.Resource, error)
		err          string
		expected     string
	}{
		{
			name:     "valid host.id",
			resValue: "my-linux-host-id",
			resError: nil,
			expected: "my-linux-host-id",
		},
		{
			name:     "empty host.id",
			resValue: "",
			resError: nil,
			err:      "failed to obtain host id",
			expected: "",
		},
		{
			name:     "error",
			resValue: "",
			resError: fmt.Errorf("some error"),
			err:      "failed to obtain host id: some error",
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := fakeLinuxSystemMetadataProvider()
			p.newResource = func(ctx context.Context, o ...resource.Option) (*resource.Resource, error) {
				if tt.resValue == "" {
					return resource.NewSchemaless(), tt.resError
				}

				v := attribute.KeyValue{
					Key:   "host.id",
					Value: attribute.StringValue(tt.resValue),
				}
				ret := resource.NewSchemaless(v)

				return ret, tt.resError
			}
			id, err := p.HostID(context.Background())

			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, id)
		})
	}
}

func fakeLinuxSystemMetadataProvider() *systemMetadataProvider {
	return &systemMetadataProvider{
		nameInfoProvider: fakeLinuxNameInfoProvider(),
	}
}

func fakeWindowsSystemMetadataProvider() *systemMetadataProvider {
	return &systemMetadataProvider{
		nameInfoProvider: fakeWindowsNameInfoProvider(),
	}
}

func fakeLinuxNameInfoProvider() nameInfoProvider {
	return nameInfoProvider{
		osHostname: func() (string, error) {
			return "my-linux-vm", nil
		},
		lookupCNAME: func(s string) (string, error) {
			return "my-linux-vm.abcdefghijklmnopqrstuvwxyz.xx.internal.foo.net.", nil
		},
		lookupHost: func(s string) ([]string, error) {
			return []string{"172.24.0.4"}, nil
		},
		lookupAddr: func(s string) ([]string, error) {
			return []string{"my-linux-vm.internal.foo.net."}, nil
		},
	}
}

func fakeWindowsNameInfoProvider() nameInfoProvider {
	fqdn := "my-windows-vm.abcdefghijklmnopqrstuvwxyz.xx.internal.foo.net."
	return nameInfoProvider{
		osHostname: func() (string, error) {
			return "my-windows-vm", nil
		},
		lookupCNAME: func(s string) (string, error) {
			return fqdn, nil
		},
		lookupHost: func(s string) ([]string, error) {
			return []string{"ffff::0000:1111:2222:3333%Ethernet", "1.2.3.4"}, nil
		},
		lookupAddr: func(s string) ([]string, error) {
			if strings.HasSuffix(s, "%Ethernet") {
				return nil, fmt.Errorf("lookup %s: unrecognized address", s)
			}
			return []string{fqdn}, nil
		},
	}
}
