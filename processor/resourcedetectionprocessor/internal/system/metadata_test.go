// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
