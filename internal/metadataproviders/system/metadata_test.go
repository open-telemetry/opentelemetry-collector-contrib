// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
