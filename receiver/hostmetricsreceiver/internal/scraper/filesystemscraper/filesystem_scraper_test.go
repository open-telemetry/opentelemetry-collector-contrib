// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filesystemscraper

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/shirou/gopsutil/v3/common"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper/internal/metadata"
)

func TestScrape(t *testing.T) {
	type testCase struct {
		name                     string
		config                   Config
		rootPath                 string
		bootTimeFunc             func(context.Context) (uint64, error)
		partitionsFunc           func(context.Context, bool) ([]disk.PartitionStat, error)
		usageFunc                func(context.Context, string) (*disk.UsageStat, error)
		expectMetrics            bool
		expectedDeviceDataPoints int
		expectedDeviceAttributes []map[string]pcommon.Value
		newErrRegex              string
		initializationErr        string
		expectedErr              string
		failedMetricsLen         *int
		continueOnErr            bool
	}

	testCases := []testCase{
		{
			name:          "Standard",
			config:        Config{MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()},
			expectMetrics: true,
		},
		{
			name: "Include single device filter",
			config: Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				IncludeDevices:       DeviceMatchConfig{filterset.Config{MatchType: "strict"}, []string{"a"}},
			},
			partitionsFunc: func(context.Context, bool) ([]disk.PartitionStat, error) {
				return []disk.PartitionStat{{Device: "a"}, {Device: "b"}}, nil
			},
			usageFunc: func(context.Context, string) (*disk.UsageStat, error) {
				return &disk.UsageStat{}, nil
			},
			expectMetrics:            true,
			expectedDeviceDataPoints: 1,
		},
		{
			name: "Include Device Filter that matches nothing",
			config: Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				IncludeDevices:       DeviceMatchConfig{filterset.Config{MatchType: "strict"}, []string{"@*^#&*$^#)"}},
			},
			expectMetrics: false,
		},
		{
			name: "Include device filtering that includes virtual partitions",
			config: Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				IncludeVirtualFS:     true,
				IncludeFSTypes:       FSTypeMatchConfig{Config: filterset.Config{MatchType: filterset.Strict}, FSTypes: []string{"tmpfs"}},
			},
			partitionsFunc: func(_ context.Context, includeVirtual bool) (paritions []disk.PartitionStat, err error) {
				paritions = append(paritions, disk.PartitionStat{Device: "root-device", Fstype: "ext4"})
				if includeVirtual {
					paritions = append(paritions, disk.PartitionStat{Device: "shm", Fstype: "tmpfs"})
				}
				return paritions, err
			},
			usageFunc: func(_ context.Context, s string) (*disk.UsageStat, error) {
				return &disk.UsageStat{}, nil
			},
			expectMetrics:            true,
			expectedDeviceDataPoints: 1,
		},
		{
			name: "Include filter with devices, filesystem type and mount points",
			config: Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				IncludeDevices: DeviceMatchConfig{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Devices: []string{"device_a", "device_b"},
				},
				ExcludeFSTypes: FSTypeMatchConfig{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					FSTypes: []string{"fs_type_b"},
				},
				ExcludeMountPoints: MountPointMatchConfig{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					MountPoints: []string{"mount_point_b", "mount_point_c"},
				},
			},
			usageFunc: func(_ context.Context, s string) (*disk.UsageStat, error) {
				return &disk.UsageStat{
					Fstype: "fs_type_a",
				}, nil
			},
			partitionsFunc: func(_ context.Context, b bool) ([]disk.PartitionStat, error) {
				return []disk.PartitionStat{
					{
						Device:     "device_a",
						Mountpoint: "mount_point_a",
						Fstype:     "fs_type_a",
					},
					{
						Device:     "device_a",
						Mountpoint: "mount_point_b",
						Fstype:     "fs_type_b",
					},
					{
						Device:     "device_b",
						Mountpoint: "mount_point_c",
						Fstype:     "fs_type_b",
					},
					{
						Device:     "device_b",
						Mountpoint: "mount_point_d",
						Fstype:     "fs_type_c",
					},
				}, nil
			},
			expectMetrics:            true,
			expectedDeviceDataPoints: 2,
			expectedDeviceAttributes: []map[string]pcommon.Value{
				{
					"device":     pcommon.NewValueStr("device_a"),
					"mountpoint": pcommon.NewValueStr("mount_point_a"),
					"type":       pcommon.NewValueStr("fs_type_a"),
					"mode":       pcommon.NewValueStr("unknown"),
				},
				{
					"device":     pcommon.NewValueStr("device_b"),
					"mountpoint": pcommon.NewValueStr("mount_point_d"),
					"type":       pcommon.NewValueStr("fs_type_c"),
					"mode":       pcommon.NewValueStr("unknown"),
				},
			},
		},
		{
			name: "RootPath at /hostfs",
			config: Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
			rootPath: filepath.Join("/", "hostfs"),
			usageFunc: func(_ context.Context, s string) (*disk.UsageStat, error) {
				if s != filepath.Join("/hostfs", "mount_point_a") {
					return nil, errors.New("mountpoint not translated according to RootPath")
				}
				return &disk.UsageStat{
					Fstype: "fs_type_a",
				}, nil
			},
			partitionsFunc: func(_ context.Context, b bool) ([]disk.PartitionStat, error) {
				return []disk.PartitionStat{
					{
						Device:     "device_a",
						Mountpoint: "mount_point_a",
						Fstype:     "fs_type_a",
					},
				}, nil
			},
			expectMetrics:            true,
			expectedDeviceDataPoints: 1,
			expectedDeviceAttributes: []map[string]pcommon.Value{
				{
					"device":     pcommon.NewValueStr("device_a"),
					"mountpoint": pcommon.NewValueStr("mount_point_a"),
					"type":       pcommon.NewValueStr("fs_type_a"),
					"mode":       pcommon.NewValueStr("unknown"),
				},
			},
		},
		{
			name: "Invalid Include Device Filter",
			config: Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				IncludeDevices:       DeviceMatchConfig{Devices: []string{"test"}},
			},
			newErrRegex: "^error creating include_devices filter:",
		},
		{
			name: "Invalid Exclude Device Filter",
			config: Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				ExcludeDevices:       DeviceMatchConfig{Devices: []string{"test"}},
			},
			newErrRegex: "^error creating exclude_devices filter:",
		},
		{
			name: "Invalid Include Filesystems Filter",
			config: Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				IncludeFSTypes:       FSTypeMatchConfig{FSTypes: []string{"test"}},
			},
			newErrRegex: "^error creating include_fs_types filter:",
		},
		{
			name: "Invalid Exclude Filesystems Filter",
			config: Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				ExcludeFSTypes:       FSTypeMatchConfig{FSTypes: []string{"test"}},
			},
			newErrRegex: "^error creating exclude_fs_types filter:",
		},
		{
			name: "Invalid Include Moountpoints Filter",
			config: Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				IncludeMountPoints:   MountPointMatchConfig{MountPoints: []string{"test"}},
			},
			newErrRegex: "^error creating include_mount_points filter:",
		},
		{
			name: "Invalid Exclude Moountpoints Filter",
			config: Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				ExcludeMountPoints:   MountPointMatchConfig{MountPoints: []string{"test"}},
			},
			newErrRegex: "^error creating exclude_mount_points filter:",
		},
		{
			name:           "Partitions Error",
			partitionsFunc: func(context.Context, bool) ([]disk.PartitionStat, error) { return nil, errors.New("err1") },
			expectedErr:    "err1",
		},
		{
			name: "Partitions and error provided",
			config: Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				IncludeDevices: DeviceMatchConfig{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					Devices: []string{"device_a", "device_b"},
				},
				ExcludeFSTypes: FSTypeMatchConfig{
					Config: filterset.Config{
						MatchType: filterset.Strict,
					},
					FSTypes: []string{"fs_type_b"},
				},
			},
			usageFunc: func(_ context.Context, s string) (*disk.UsageStat, error) {
				return &disk.UsageStat{
					Fstype: "fs_type_a",
				}, nil
			},
			partitionsFunc: func(_ context.Context, b bool) ([]disk.PartitionStat, error) {
				return []disk.PartitionStat{
					{
						Device:     "device_a",
						Mountpoint: "mount_point_a",
						Fstype:     "fs_type_a",
					},
					{
						Device:     "device_b",
						Mountpoint: "mount_point_d",
						Fstype:     "fs_type_c",
					},
				}, errors.New("invalid partitions collection")
			},
			expectMetrics:            true,
			expectedDeviceDataPoints: 2,
			expectedDeviceAttributes: []map[string]pcommon.Value{
				{
					"device":     pcommon.NewValueStr("device_a"),
					"mountpoint": pcommon.NewValueStr("mount_point_a"),
					"type":       pcommon.NewValueStr("fs_type_a"),
					"mode":       pcommon.NewValueStr("unknown"),
				},
				{
					"device":     pcommon.NewValueStr("device_b"),
					"mountpoint": pcommon.NewValueStr("mount_point_d"),
					"type":       pcommon.NewValueStr("fs_type_c"),
					"mode":       pcommon.NewValueStr("unknown"),
				},
			},
			expectedErr:      "failed collecting partitions information: invalid partitions collection",
			failedMetricsLen: new(int),
			continueOnErr:    true,
		},
		{
			name:        "Usage Error",
			usageFunc:   func(context.Context, string) (*disk.UsageStat, error) { return nil, errors.New("err2") },
			expectedErr: "err2",
		},
	}

	for _, test := range testCases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			test.config.SetRootPath(test.rootPath)
			scraper, err := newFileSystemScraper(context.Background(), receivertest.NewNopCreateSettings(), &test.config, common.EnvMap{})
			if test.newErrRegex != "" {
				require.Error(t, err)
				require.Regexp(t, test.newErrRegex, err)
				return
			}
			require.NoError(t, err, "Failed to create file system scraper: %v", err)

			if test.partitionsFunc != nil {
				scraper.partitions = test.partitionsFunc
			}
			if test.usageFunc != nil {
				scraper.usage = test.usageFunc
			}
			if test.bootTimeFunc != nil {
				scraper.bootTime = test.bootTimeFunc
			}

			err = scraper.start(context.Background(), componenttest.NewNopHost())
			if test.initializationErr != "" {
				assert.EqualError(t, err, test.initializationErr)
				return
			}
			require.NoError(t, err, "Failed to initialize file system scraper: %v", err)

			md, err := scraper.scrape(context.Background())
			if test.expectedErr != "" {
				assert.Contains(t, err.Error(), test.expectedErr)

				isPartial := scrapererror.IsPartialScrapeError(err)
				assert.True(t, isPartial)
				if isPartial {
					var scraperErr scrapererror.PartialScrapeError
					require.ErrorAs(t, err, &scraperErr)
					expectedFailedMetricsLen := metricsLen
					if test.failedMetricsLen != nil {
						expectedFailedMetricsLen = *test.failedMetricsLen
					}
					assert.Equal(t, expectedFailedMetricsLen, scraperErr.Failed)
				}
				if !test.continueOnErr {
					return
				}
			} else {
				require.NoError(t, err, "Failed to scrape metrics: %v", err)
			}

			if !test.expectMetrics {
				assert.Equal(t, 0, md.MetricCount())
				return
			}

			assert.GreaterOrEqual(t, md.MetricCount(), 1)

			metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			m, err := findMetricByName(metrics, "system.filesystem.usage")
			assert.NoError(t, err)
			assertFileSystemUsageMetricValid(
				t,
				m,
				test.expectedDeviceDataPoints*fileSystemStatesLen,
				test.expectedDeviceAttributes,
			)

			if isUnix() {
				assertFileSystemUsageMetricHasUnixSpecificStateLabels(t, m)
				m, err = findMetricByName(metrics, "system.filesystem.inodes.usage")
				assert.NoError(t, err)
				assertFileSystemUsageMetricValid(
					t,
					m,
					test.expectedDeviceDataPoints*2,
					test.expectedDeviceAttributes,
				)
			}

			internal.AssertSameTimeStampForAllMetrics(t, metrics)
		})
	}
}

func findMetricByName(metrics pmetric.MetricSlice, name string) (pmetric.Metric, error) {
	for i := 0; i < metrics.Len(); i++ {
		if metrics.At(i).Name() == name {
			return metrics.At(i), nil
		}
	}
	return pmetric.Metric{}, fmt.Errorf("no metric found with name %s", name)
}

func assertFileSystemUsageMetricValid(
	t *testing.T,
	metric pmetric.Metric,
	expectedDeviceDataPoints int,
	expectedDeviceAttributes []map[string]pcommon.Value) {
	for i := 0; i < metric.Sum().DataPoints().Len(); i++ {
		for _, label := range []string{"device", "type", "mode", "mountpoint"} {
			internal.AssertSumMetricHasAttribute(t, metric, i, label)
		}
	}

	if expectedDeviceDataPoints > 0 {
		assert.Equal(t, expectedDeviceDataPoints, metric.Sum().DataPoints().Len())

		// Assert label values if specified.
		if expectedDeviceAttributes != nil {
			dpsPerDevice := expectedDeviceDataPoints / len(expectedDeviceAttributes)
			deviceIdx := 0
			for i := 0; i < metric.Sum().DataPoints().Len(); i += dpsPerDevice {
				for j := i; j < i+dpsPerDevice; j++ {
					for labelKey, labelValue := range expectedDeviceAttributes[deviceIdx] {
						internal.AssertSumMetricHasAttributeValue(t, metric, j, labelKey, labelValue)
					}
				}
				deviceIdx++
			}
		}
	} else {
		assert.GreaterOrEqual(t, metric.Sum().DataPoints().Len(), fileSystemStatesLen)
	}
	internal.AssertSumMetricHasAttributeValue(t, metric, 0, "state",
		pcommon.NewValueStr(metadata.AttributeStateUsed.String()))
	internal.AssertSumMetricHasAttributeValue(t, metric, 1, "state",
		pcommon.NewValueStr(metadata.AttributeStateFree.String()))
}

func assertFileSystemUsageMetricHasUnixSpecificStateLabels(t *testing.T, metric pmetric.Metric) {
	internal.AssertSumMetricHasAttributeValue(t, metric, 2, "state",
		pcommon.NewValueStr(metadata.AttributeStateReserved.String()))
}

func isUnix() bool {
	for _, unixOS := range []string{"linux", "darwin", "freebsd", "openbsd", "solaris"} {
		if runtime.GOOS == unixOS {
			return true
		}
	}

	return false
}
