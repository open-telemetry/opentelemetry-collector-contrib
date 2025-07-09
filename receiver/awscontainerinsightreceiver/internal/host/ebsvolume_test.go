// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package host

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type mockEBSVolumeClient func(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)

func (m mockEBSVolumeClient) DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return m(ctx, params, optFns...)
}

type mockFileInfo struct{}

func (m *mockFileInfo) Name() string {
	return "mockFileInfo"
}

func (m *mockFileInfo) Size() int64 {
	return 256
}

func (m *mockFileInfo) Mode() os.FileMode {
	return os.ModeSymlink
}

func (m *mockFileInfo) ModTime() time.Time {
	return time.Now()
}

func (m *mockFileInfo) IsDir() bool {
	return false
}

func (m *mockFileInfo) Sys() any {
	return nil
}

func TestGetEBSVolumeID(t *testing.T) {
	tests := []struct {
		name     string
		client   func(t *testing.T) ebsVolumeClient
		devPath  string
		expected string
	}{
		{
			name: "Able to get EBS volume with no symlink to nvme device",
			client: func(t *testing.T) ebsVolumeClient {
				return mockEBSVolumeClient(func(_ context.Context, _ *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
					t.Helper()
					return &ec2.DescribeVolumesOutput{
						Volumes: []ec2types.Volume{
							{
								AvailabilityZone: aws.String("us-west-2"),
								Attachments: []ec2types.VolumeAttachment{
									{
										Device:   aws.String("/dev/xvdc"),
										VolumeId: aws.String("vol-0c241693efb58734a"),
									},
								},
							},
						},
					}, nil
				})
			},
			devPath:  "/dev/xvdc",
			expected: "aws://us-west-2/vol-0c241693efb58734a",
		},
		{
			name: "Able to get EBS volume through nvme symlink",
			client: func(t *testing.T) ebsVolumeClient {
				return mockEBSVolumeClient(func(_ context.Context, _ *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
					t.Helper()
					return &ec2.DescribeVolumesOutput{
						Volumes: []ec2types.Volume{
							{
								AvailabilityZone: aws.String("us-west-2"),
								Attachments: []ec2types.VolumeAttachment{
									{
										Device:   aws.String("/dev/xvdb"),
										VolumeId: aws.String("vol-0c241693efb58734a"),
									},
								},
							},
						},
					}, nil
				})
			},
			devPath:  "/dev/nvme0n2",
			expected: "aws://us-west-2/vol-0c241693efb58734a",
		},
		{
			name: "Empty string when no matching EBS volume returned by client",
			client: func(t *testing.T) ebsVolumeClient {
				return mockEBSVolumeClient(func(_ context.Context, _ *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
					t.Helper()
					return &ec2.DescribeVolumesOutput{
						Volumes: []ec2types.Volume{
							{
								AvailabilityZone: aws.String("us-west-2"),
								Attachments: []ec2types.VolumeAttachment{
									{
										Device:   aws.String("/dev/xvdb"),
										VolumeId: aws.String("vol-0c241693efb58734a"),
									},
								},
							},
						},
					}, nil
				})
			},
			devPath:  "/dev/invalid",
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := &ebsVolume{
				maxJitterTime: time.Millisecond,
				instanceID:    "instanceID",
				client:        test.client(t),
				dev2Vol:       make(map[string]string),
				logger:        zap.NewNop(),
				hostMounts:    "./testdata/mounts",
				osLstat: func(name string) (os.FileInfo, error) {
					if name == hostProc {
						return &mockFileInfo{}, nil
					}

					return &mockFileInfo{}, nil
				},
				evalSymLinks: func(path string) (string, error) {
					if strings.HasSuffix(path, "/dev/xvdb") {
						return "/dev/nvme0n2", nil
					}
					return "", errors.New("error")
				},
			}
			e.refresh(context.Background())

			assert.Equal(t, test.expected, e.getEBSVolumeID(test.devPath))
		})
	}
}

func TestExtractEbsIDsUsedByKubernetes(t *testing.T) {
	tests := []struct {
		name                string
		client              func(t *testing.T) ebsVolumeClient
		hostMounts          string
		devPath             string
		expected            string
		expectedVolumeCount int
	}{
		{
			name:                "One Volume",
			hostMounts:          "./testdata/mounts",
			devPath:             "/dev/nvme1n1",
			expected:            "aws://us-west-2b/vol-0d9f0816149eb2050",
			expectedVolumeCount: 1,
		},
		{
			name:                "Invalid Path",
			hostMounts:          "/an-invalid-path",
			expectedVolumeCount: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := &ebsVolume{
				maxJitterTime: time.Millisecond,
				instanceID:    "instanceID",
				logger:        zap.NewNop(),
				hostMounts:    test.hostMounts,
				osLstat: func(name string) (os.FileInfo, error) {
					if name == hostProc {
						return &mockFileInfo{}, nil
					}

					return &mockFileInfo{}, nil
				},
				evalSymLinks: func(path string) (string, error) {
					if strings.HasSuffix(path, "/dev/xvdb") {
						return "/dev/nvme0n2", nil
					}
					return "", errors.New("error")
				},
			}

			ebsIDs := e.extractEbsIDsUsedByKubernetes()

			assert.Len(t, ebsIDs, test.expectedVolumeCount)
			if test.expectedVolumeCount > 0 {
				assert.Equal(t, test.expected, ebsIDs[test.devPath])
			}
		})
	}
}
