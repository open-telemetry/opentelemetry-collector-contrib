// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package host // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"go.uber.org/zap"
)

var ebsMountPointRegex = regexp.MustCompile(`kubernetes\.io/aws-ebs/mounts/aws/(.+)/(vol-\w+)$`)

type ebsVolumeClient interface {
	DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
}

type ebsVolumeProvider interface {
	getEBSVolumeID(devName string) string
	extractEbsIDsUsedByKubernetes() map[string]string
}

type ebsVolume struct {
	refreshInterval time.Duration
	maxJitterTime   time.Duration
	instanceID      string
	client          ebsVolumeClient
	logger          *zap.Logger
	shutdownC       chan bool

	mu sync.RWMutex
	// device name to volumeID mapping
	dev2Vol map[string]string

	// for testing only
	hostMounts   string
	osLstat      func(name string) (os.FileInfo, error)
	evalSymLinks func(path string) (string, error)
}

type ebsVolumeOption func(*ebsVolume)

func newEBSVolume(ctx context.Context, cfg aws.Config, instanceID string, region string,
	refreshInterval time.Duration, logger *zap.Logger, options ...ebsVolumeOption,
) ebsVolumeProvider {
	cfg.Region = region

	e := &ebsVolume{
		dev2Vol:         make(map[string]string),
		instanceID:      instanceID,
		client:          ec2.NewFromConfig(cfg),
		refreshInterval: refreshInterval,
		maxJitterTime:   3 * time.Second,
		shutdownC:       make(chan bool),
		logger:          logger,
		hostMounts:      hostMounts,
		osLstat:         os.Lstat,
		evalSymLinks:    filepath.EvalSymlinks,
	}

	for _, opt := range options {
		opt(e)
	}

	shouldRefresh := func() bool {
		// keep refreshing to get updated ebs volumes
		return true
	}
	go RefreshUntil(ctx, e.refresh, e.refreshInterval, shouldRefresh, e.maxJitterTime)

	return e
}

func (e *ebsVolume) refresh(ctx context.Context) {
	e.logger.Info("Fetch ebs volumes from ec2 api")

	input := &ec2.DescribeVolumesInput{
		Filters: []ec2types.Filter{
			{
				Name:   aws.String("attachment.instance-id"),
				Values: []string{e.instanceID},
			},
		},
	}

	devPathSet := make(map[string]bool)
	allSuccess := false
	for {
		result, err := e.client.DescribeVolumes(ctx, input)
		if err != nil {
			e.logger.Warn("Fail to call ec2 DescribeVolumes", zap.Error(err))
			break
		}
		for _, volume := range result.Volumes {
			for _, attachment := range volume.Attachments {
				devPath := e.addEBSVolumeMapping(volume.AvailabilityZone, attachment)
				devPathSet[devPath] = true
			}
		}
		allSuccess = true
		if result.NextToken == nil {
			break
		}
		input.NextToken = result.NextToken
	}

	if allSuccess {
		e.mu.Lock()
		defer e.mu.Unlock()
		for k := range e.dev2Vol {
			if !devPathSet[k] {
				delete(e.dev2Vol, k)
			}
		}
	}
}

func (e *ebsVolume) addEBSVolumeMapping(zone *string, attachment ec2types.VolumeAttachment) string {
	// *attachment.Device is sth like: /dev/xvda
	devPath := e.findNvmeBlockNameIfPresent(*attachment.Device)
	if devPath == "" {
		devPath = *attachment.Device
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.dev2Vol[devPath] = fmt.Sprintf("aws://%s/%s", *zone, *attachment.VolumeId)
	return devPath
}

// find nvme block name by symlink, if symlink doesn't exist, return ""
func (e *ebsVolume) findNvmeBlockNameIfPresent(devName string) string {
	// for nvme(ssd), there is a symlink from devName to nvme block name, i.e. /dev/xvda -> /dev/nvme0n1
	// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/nvme-ebs-volumes.html
	hasRootFs := true
	if _, err := e.osLstat(hostProc); os.IsNotExist(err) {
		hasRootFs = false
	}
	nvmeName := ""

	if hasRootFs {
		devName = "/rootfs" + devName
	}

	if info, err := e.osLstat(devName); err == nil {
		if info.Mode()&os.ModeSymlink == os.ModeSymlink {
			if path, err := e.evalSymLinks(devName); err == nil {
				nvmeName = path
			}
		}
	}

	if nvmeName != "" && hasRootFs {
		nvmeName = strings.TrimPrefix(nvmeName, "/rootfs")
	}
	return nvmeName
}

func (e *ebsVolume) getEBSVolumeID(devName string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for k, v := range e.dev2Vol {
		// The key of dev2Vol is device name like nvme0n1, while the input devName could be a partition name like nvme0n1p1
		if strings.HasPrefix(devName, k) {
			return v
		}
	}

	return ""
}

// extract the ebs volume id used by kubernetes cluster
func (e *ebsVolume) extractEbsIDsUsedByKubernetes() map[string]string {
	ebsVolumeIDs := make(map[string]string)

	file, err := os.Open(e.hostMounts)
	if err != nil {
		e.logger.Debug("cannot open /rootfs/proc/mounts", zap.Error(err))
		return ebsVolumeIDs
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		lineStr := scanner.Text()
		if strings.TrimSpace(lineStr) == "" {
			continue
		}

		// example line: /dev/nvme1n1 /var/lib/kubelet/plugins/kubernetes.io/aws-ebs/mounts/aws/us-west-2b/vol-0d9f0816149eb2050 ext4 rw,relatime,data=ordered 0 0
		keys := strings.Split(lineStr, " ")
		if len(keys) < 2 {
			continue
		}
		matches := ebsMountPointRegex.FindStringSubmatch(keys[1])
		if len(matches) > 0 {
			// Set {"/dev/nvme1n1": "aws://us-west-2b/vol-0d9f0816149eb2050"}
			ebsVolumeIDs[keys[0]] = fmt.Sprintf("aws://%s/%s", matches[1], matches[2])
		}
	}

	return ebsVolumeIDs
}
