// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

// Taken from https://github.com/signalfx/golib/blob/master/metadata/hostmetadata/host-linux_test.go as is.

package hostmetadata

import (
	"context"
	"errors"
	"testing"

	"github.com/shirou/gopsutil/v4/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/gopsutilenv"
)

func TestFillOSSpecificData(t *testing.T) {
	type args struct {
		syscallUname func(*unix.Utsname) error
		etc          string
	}
	tests := []struct {
		name    string
		args    args
		want    *hostOS
		wantErr bool
	}{
		{
			name: "get uname os information",
			args: args{
				etc: "./testdata/lsb-release",
				syscallUname: func(in *unix.Utsname) error {
					in.Version = [65]byte{
						35, 57, 45, 85, 98, 117, 110, 116,
						117, 32, 83, 77, 80, 32, 87, 101, 100,
						32, 77, 97, 121, 32, 49, 54, 32, 49,
						53, 58, 50, 50, 58, 53, 52, 32, 85,
						84, 67, 32, 50, 48, 49, 56,
					}
					return nil
				},
			},
			want: &hostOS{
				HostKernelVersion: "#9-Ubuntu SMP Wed May 16 15:22:54 UTC 2018",
				HostLinuxVersion:  "Ubuntu 18.04 LTS",
			},
		},
		{
			name: "get uname os information uname call fails",
			args: args{
				etc: "./testdata/lsb-release",
				syscallUname: func(in *unix.Utsname) error {
					in.Version = [65]byte{}
					return errors.New("shouldn't work")
				},
			},
			want:    &hostOS{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			syscallUname = tt.args.syscallUname
			t.Cleanup(func() { gopsutilenv.SetGlobalRootPath("") })
			envMap := gopsutilenv.SetGoPsutilEnvVars(tt.args.etc)
			in := &hostOS{}
			err := fillPlatformSpecificOSData(context.WithValue(context.Background(), common.EnvKey, envMap), in)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, in)
		})
		syscallUname = unix.Uname
	}
}

func TestFillPlatformSpecificCPUData(t *testing.T) {
	type args struct {
		syscallUname func(*unix.Utsname) error
	}
	tests := []struct {
		name    string
		args    args
		want    *hostCPU
		wantErr bool
	}{
		{
			name: "get uname cpu information",
			args: args{
				syscallUname: func(in *unix.Utsname) error {
					in.Machine = [65]byte{120, 56, 54, 95, 54, 52}
					return nil
				},
			},
			want: &hostCPU{
				HostMachine:   "x86_64",
				HostProcessor: "x86_64",
			},
		},
		{
			name: "get uname cpu information and the call to uname fails",
			args: args{
				syscallUname: func(in *unix.Utsname) error {
					in.Machine = [65]byte{}
					return errors.New("shouldn't work")
				},
			},
			want:    &hostCPU{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			syscallUname = tt.args.syscallUname
			in := &hostCPU{}
			err := fillPlatformSpecificCPUData(in)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, in)
		})
		syscallUname = unix.Uname
	}
}

func Test_GetLinuxVersion_EnvVar(t *testing.T) {
	tests := []struct {
		name    string
		etc     string
		want    string
		wantErr bool
	}{
		{
			name: "lsb-release",
			etc:  "./testdata/lsb-release/etc",
			want: "Ubuntu 18.04 LTS",
		},
		{
			name: "os-release",
			etc:  "./testdata/os-release/etc",
			want: "Debian GNU/Linux 9 (stretch)",
		},
		{
			name: "centos-release",
			etc:  "./testdata/centos-release/etc",
			want: "CentOS Linux release 7.5.1804 (Core)",
		},
		{
			name: "redhat-release",
			etc:  "./testdata/redhat-release/etc",
			want: "Red Hat Enterprise Linux Server release 7.5 (Maipo)",
		},
		{
			name: "system-release",
			etc:  "./testdata/system-release/etc",
			want: "CentOS Linux release 7.5.1804 (Core)",
		},
		{
			name:    "no release returns error",
			etc:     "./testdata",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOST_ETC", tt.etc)
			got, err := getLinuxVersion(context.Background())
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_GetLinuxVersion_EnvMap(t *testing.T) {
	tests := []struct {
		name    string
		etc     string
		want    string
		wantErr bool
	}{
		{
			name: "lsb-release",
			etc:  "./testdata/lsb-release",
			want: "Ubuntu 18.04 LTS",
		},
		{
			name: "os-release",
			etc:  "./testdata/os-release",
			want: "Debian GNU/Linux 9 (stretch)",
		},
		{
			name: "centos-release",
			etc:  "./testdata/centos-release",
			want: "CentOS Linux release 7.5.1804 (Core)",
		},
		{
			name: "redhat-release",
			etc:  "./testdata/redhat-release",
			want: "Red Hat Enterprise Linux Server release 7.5 (Maipo)",
		},
		{
			name: "system-release",
			etc:  "./testdata/system-release",
			want: "CentOS Linux release 7.5.1804 (Core)",
		},
		{
			name:    "no release returns error",
			etc:     "./testdata",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(func() { gopsutilenv.SetGlobalRootPath("") })
			err := gopsutilenv.ValidateRootPath(tt.etc)
			require.NoError(t, err)
			envMap := gopsutilenv.SetGoPsutilEnvVars(tt.etc)
			ctx := context.WithValue(context.Background(), common.EnvKey, envMap)
			got, err := getLinuxVersion(ctx)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
