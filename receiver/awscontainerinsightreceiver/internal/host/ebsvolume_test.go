// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package host

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type mockEBSVolumeClient struct {
	count   int
	success chan bool
	mu      sync.Mutex
}

func (m *mockEBSVolumeClient) DescribeVolumesWithContext(context.Context, *ec2.DescribeVolumesInput,
	...request.Option) (*ec2.DescribeVolumesOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.count++

	if m.count == 1 {
		return &ec2.DescribeVolumesOutput{}, errors.New("error")
	}

	if m.count == 2 {
		return &ec2.DescribeVolumesOutput{
			NextToken: aws.String("nextToken"),
			Volumes: []*ec2.Volume{
				{
					AvailabilityZone: aws.String("us-west-2"),
					Attachments: []*ec2.VolumeAttachment{
						{
							Device:   aws.String("/dev/xvdb"),
							VolumeId: aws.String("vol-0c241693efb58734a"),
						},
					},
				},
			},
		}, nil
	}

	if m.count == 3 {
		return &ec2.DescribeVolumesOutput{
			// NextToken: nil,
			Volumes: []*ec2.Volume{
				{
					AvailabilityZone: aws.String("us-west-2"),
					Attachments: []*ec2.VolumeAttachment{
						{
							Device:   aws.String("/dev/xvdc"),
							VolumeId: aws.String("vol-0303a1cc896c42d28"),
						},
					},
				},
			},
		}, nil
	}

	if m.count == 4 {
		close(m.success)
	}

	return &ec2.DescribeVolumesOutput{
		// NextToken: nil,
		Volumes: []*ec2.Volume{
			{
				AvailabilityZone: aws.String("us-west-2"),
				Attachments: []*ec2.VolumeAttachment{
					{
						Device:   aws.String("/dev/xvdc"),
						VolumeId: aws.String("vol-0303a1cc896c42d28"),
					},
				},
			},
			{
				AvailabilityZone: aws.String("us-west-2"),
				Attachments: []*ec2.VolumeAttachment{
					{
						Device:   aws.String("/dev/xvdb"),
						VolumeId: aws.String("vol-0c241693efb58734a"),
					},
				},
			},
		},
	}, nil
}

type mockFileInfo struct {
}

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

func TestEBSVolume(t *testing.T) {
	ctx := context.Background()
	sess := mock.Session
	mockVolumeClient := &mockEBSVolumeClient{
		success: make(chan bool),
	}
	clientOption := func(e *ebsVolume) {
		e.client = mockVolumeClient
	}
	maxJitterOption := func(e *ebsVolume) {
		e.maxJitterTime = time.Millisecond
	}
	hostMountsOption := func(e *ebsVolume) {
		e.hostMounts = "./testdata/mounts"
	}

	LstatOption := func(e *ebsVolume) {
		e.osLstat = func(name string) (os.FileInfo, error) {
			if name == hostProc {
				return &mockFileInfo{}, nil
			}

			return &mockFileInfo{}, nil
		}
	}

	evalSymLinksOption := func(e *ebsVolume) {
		e.evalSymLinks = func(path string) (string, error) {
			if strings.HasSuffix(path, "/dev/xvdb") {
				return "/dev/nvme0n2", nil
			}
			return "", errors.New("error")
		}
	}

	e := newEBSVolume(ctx, sess, "instanceId", "us-west-2", time.Millisecond, zap.NewNop(),
		clientOption, maxJitterOption, hostMountsOption, LstatOption, evalSymLinksOption)

	<-mockVolumeClient.success
	assert.Equal(t, "aws://us-west-2/vol-0303a1cc896c42d28", e.getEBSVolumeID("/dev/xvdc"))
	assert.Equal(t, "aws://us-west-2/vol-0c241693efb58734a", e.getEBSVolumeID("/dev/nvme0n2"))
	assert.Equal(t, "", e.getEBSVolumeID("/dev/invalid"))

	ebsIds := e.extractEbsIDsUsedByKubernetes()
	assert.Equal(t, 1, len(ebsIds))
	assert.Equal(t, "aws://us-west-2b/vol-0d9f0816149eb2050", ebsIds["/dev/nvme1n1"])

	// set e.hostMounts to an invalid path
	hostMountsOption = func(e *ebsVolume) {
		e.hostMounts = "/an-invalid-path"
	}
	e = newEBSVolume(ctx, sess, "instanceId", "us-west-2", time.Millisecond, zap.NewNop(),
		clientOption, maxJitterOption, hostMountsOption, LstatOption, evalSymLinksOption)
	ebsIds = e.extractEbsIDsUsedByKubernetes()
	assert.Equal(t, 0, len(ebsIds))
}
