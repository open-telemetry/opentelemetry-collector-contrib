// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsinfo // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/ecsInfo"

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	ecsInstanceMountConfigPath = "/proc/self/mountinfo"
	// infinity magic number for cgroup: https://unix.stackexchange.com/questions/420906/what-is-the-value-for-the-cgroups-limit-in-bytes-if-the-memory-is-not-restricte
	kernelMagicCodeNotSet = int64(9223372036854771712)
)

type cgroupScanner struct {
	logger                        *zap.Logger
	mountPoint                    string
	ecsTaskInfoProvider           ecsTaskInfoProvider
	containerInstanceInfoProvider containerInstanceInfoProvider
	refreshInterval               time.Duration
	ctx                           context.Context
	sync.RWMutex

	cpuReserved int64
	memReserved int64
}

type cgroupScannerProvider interface {
	getCPUReserved() int64
	getMemReserved() int64

	// use for test
	getCPUReservedInTask(taskID string, clusterName string) int64
	getMEMReservedInTask(taskID string, clusterName string, containers []ECSContainer) int64
}

func newCGroupScanner(ctx context.Context, mountConfigPath string, logger *zap.Logger, ecsTaskInfoProvider ecsTaskInfoProvider, containerInstanceInfoProvider containerInstanceInfoProvider, refreshInterval time.Duration) cgroupScannerProvider {
	mp, err := getCGroupMountPoint(mountConfigPath)
	if err != nil {
		logger.Warn("Fallback to /cgroup, because it failed to get the cgroup mount point, error: ", zap.Error(err))
		mp = "/cgroup"
	}

	c := &cgroupScanner{
		ctx:                           ctx,
		logger:                        logger,
		mountPoint:                    mp,
		ecsTaskInfoProvider:           ecsTaskInfoProvider,
		containerInstanceInfoProvider: containerInstanceInfoProvider,
		refreshInterval:               refreshInterval,
	}

	if c.getMemReserved() == 0 && c.getCPUReserved() == 0 {
		c.refresh()
	}

	go func() {
		refreshTicker := time.NewTicker(c.refreshInterval)
		defer refreshTicker.Stop()
		for {
			select {
			case <-refreshTicker.C:
				c.refresh()
			case <-ctx.Done():
				refreshTicker.Stop()
				return
			}
		}
	}()

	return c
}

func (c *cgroupScanner) refresh() {

	if c.ecsTaskInfoProvider == nil {
		return
	}

	cpuReserved := int64(0)
	memReserved := int64(0)

	for _, task := range c.ecsTaskInfoProvider.getRunningTasksInfo() {
		taskID, err := getTaskCgroupPathFromARN(task.ARN)
		if err != nil {
			c.logger.Warn("Failed to get ecs taskid from arn: ", zap.Error(err))
			continue
		}
		// ignore the one only consume 2 shares which is the default value in cgroup
		if cr := c.getCPUReservedInTask(taskID, c.containerInstanceInfoProvider.GetClusterName()); cr > 2 {
			cpuReserved += cr
		}
		memReserved += c.getMEMReservedInTask(taskID, c.containerInstanceInfoProvider.GetClusterName(), task.Containers)
	}
	c.Lock()
	defer c.Unlock()
	c.memReserved = memReserved
	c.cpuReserved = cpuReserved
}

func newCGroupScannerForContainer(ctx context.Context, logger *zap.Logger, ecsTaskInfoProvider ecsTaskInfoProvider, containerInstanceInfoProvider containerInstanceInfoProvider, refreshInterval time.Duration) cgroupScannerProvider {
	return newCGroupScanner(ctx, ecsInstanceMountConfigPath, logger, ecsTaskInfoProvider, containerInstanceInfoProvider, refreshInterval)
}

func (c *cgroupScanner) getCPUReservedInTask(taskID string, clusterName string) int64 {
	cpuPath, err := getCGroupPathForTask(c.mountPoint, "cpu", taskID, clusterName)
	if err != nil {
		c.logger.Warn("Failed to get cpu cgroup path for task: ", zap.Error(err))
		return int64(0)
	}

	// check if hard limit is configured
	if cfsQuota, err := readInt64(cpuPath, "cpu.cfs_quota_us"); err == nil && cfsQuota != -1 {
		if cfsPeriod, err := readInt64(cpuPath, "cpu.cfs_period_us"); err == nil && cfsPeriod > 0 {
			return int64(math.Ceil(float64(1024*cfsQuota) / float64(cfsPeriod)))
		}
	}

	if shares, err := readInt64(cpuPath, "cpu.shares"); err == nil {
		return shares
	}

	return int64(0)
}

func (c *cgroupScanner) getMEMReservedInTask(taskID string, clusterName string, containers []ECSContainer) int64 {
	memPath, err := getCGroupPathForTask(c.mountPoint, "memory", taskID, clusterName)
	if err != nil {
		c.logger.Warn("Failed to get memory cgroup path for task: %v", zap.Error(err))
		return int64(0)
	}

	if memReserved, err := readInt64(memPath, "memory.limit_in_bytes"); err == nil && memReserved != kernelMagicCodeNotSet {
		return memReserved
	}

	// sum the containers' memory if the task's memory limit is not configured
	sum := int64(0)
	for _, container := range containers {
		containerPath := filepath.Join(memPath, container.DockerID)

		// soft limit first
		if softLimit, err := readInt64(containerPath, "memory.soft_limit_in_bytes"); err == nil && softLimit != kernelMagicCodeNotSet {
			sum += softLimit
			continue
		}

		// try hard limit when soft limit is not configured
		if hardLimit, err := readInt64(containerPath, "memory.limit_in_bytes"); err == nil && hardLimit != kernelMagicCodeNotSet {
			sum += hardLimit
		}
	}
	return sum
}

func readString(dirpath string, file string) (string, error) {
	cgroupFile := filepath.Join(dirpath, file)

	// Read
	out, err := os.ReadFile(cgroupFile)
	if err != nil {
		// Ignore non-existent files
		log.Printf("W! readString: Failed to read %q: %s", cgroupFile, err)
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func readInt64(dirpath string, file string) (int64, error) {
	out, err := readString(dirpath, file)
	if err != nil {
		return 0, err
	}
	if out == "" || out == "max" {
		return 0, err
	}

	val, err := strconv.ParseInt(out, 10, 64)
	if err != nil {
		log.Printf("W! readInt64: Failed to parse int %q from file %q: %s", out, filepath.Join(dirpath, file), err)
		return 0, err
	}

	return val, nil
}
func getCGroupMountPoint(mountConfigPath string) (string, error) {
	f, err := os.Open(mountConfigPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		var (
			text   = scanner.Text()
			fields = strings.Split(text, " ")
			// safe as mountinfo encodes mountpoints with spaces as \040.
			// an example: 26 22 0:23 / /cgroup/cpu rw,relatime - cgroup cgroup rw,cpu
			index               = strings.Index(text, " - ")
			postSeparatorFields = strings.Fields(text[index+3:])
			numPostFields       = len(postSeparatorFields)
		)
		// this is an error as we can't detect if the mount is for "cgroup"
		if numPostFields == 0 {
			return "", fmt.Errorf("Found no fields post '-' in %q", text)
		}
		if postSeparatorFields[0] == "cgroup" {
			// check that the mount is properly formated.
			if numPostFields < 3 {
				return "", fmt.Errorf("Error found less than 3 fields post '-' in %q", text)
			}
			return filepath.Dir(fields[4]), nil
		}
	}
	return "", fmt.Errorf("mount point not existed")
}

func getCGroupPathForTask(cgroupMount, controller, taskID, clusterName string) (string, error) {
	taskPath := filepath.Join(cgroupMount, controller, "ecs", taskID)
	if _, err := os.Stat(taskPath); os.IsNotExist(err) {
		// Task cgroup path does not exist, fallback to try legacy Task cgroup path,
		// legacy cgroup path of task with new format ARN used to contain cluster name,
		// before ECS Agent PR https://github.com/aws/amazon-ecs-agent/pull/2497/
		taskPath = filepath.Join(cgroupMount, controller, "ecs", clusterName, taskID)
		if _, err := os.Stat(taskPath); os.IsNotExist(err) {
			return "", fmt.Errorf("CGroup Path %q does not exist", taskPath)
		}
	}
	return taskPath, nil
}

func (c *cgroupScanner) getCPUReserved() int64 {
	c.RLock()
	defer c.RUnlock()
	return c.cpuReserved
}

func (c *cgroupScanner) getMemReserved() int64 {
	c.RLock()
	defer c.RUnlock()
	return c.memReserved
}

// There are two formats of Task ARN (https://docs.aws.amazon.com/AmazonECS/latest/userguide/ecs-account-settings.html#ecs-resource-ids)
// arn:aws:ecs:region:aws_account_id:task/task-id
// arn:aws:ecs:region:aws_account_id:task/cluster-name/task-id
// we should get "task-id" as result no matter what format the ARN is.
func getTaskCgroupPathFromARN(arn string) (string, error) {
	result := strings.Split(arn, ":")
	if len(result) < 6 {
		return "", fmt.Errorf("invalid ecs task arn: %q", arn)
	}

	result = strings.Split(result[5], "/")
	switch len(result) {
	case 2:
		return result[1], nil
	case 3:
		return result[2], nil
	default:
		return "", fmt.Errorf("invalid ecs task arn: %q", arn)
	}
}
