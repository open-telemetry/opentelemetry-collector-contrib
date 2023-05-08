// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux
// +build linux

// Taken from https://github.com/signalfx/golib/blob/master/metadata/hostmetadata/host-linux_test.go as is.

package hostmetadata

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
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
					in.Version = [65]byte{35, 57, 45, 85, 98, 117, 110, 116,
						117, 32, 83, 77, 80, 32, 87, 101, 100,
						32, 77, 97, 121, 32, 49, 54, 32, 49,
						53, 58, 50, 50, 58, 53, 52, 32, 85,
						84, 67, 32, 50, 48, 49, 56}
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
			t.Setenv("HOST_ETC", tt.args.etc)
			in := &hostOS{}
			err := fillPlatformSpecificOSData(in)
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
