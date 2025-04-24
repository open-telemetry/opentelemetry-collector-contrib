// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nvme // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/gpu"
import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
)

const (
	// Original Metric Names
	ebsReadOpsTotal        = "aws_ebs_csi_read_ops_total"
	ebsWriteOpsTotal       = "aws_ebs_csi_write_ops_total"
	ebsReadBytesTotal      = "aws_ebs_csi_read_bytes_total"
	ebsWriteBytesTotal     = "aws_ebs_csi_write_bytes_total"
	ebsReadTime            = "aws_ebs_csi_read_seconds_total"
	ebsWriteTime           = "aws_ebs_csi_write_seconds_total"
	ebsExceededIOPSTime    = "aws_ebs_csi_exceeded_iops_seconds_total"
	ebsExceededTPTime      = "aws_ebs_csi_exceeded_tp_seconds_total"
	ebsExceededEC2IOPSTime = "aws_ebs_csi_ec2_exceeded_iops_seconds_total"
	ebsExceededEC2TPTime   = "aws_ebs_csi_ec2_exceeded_tp_seconds_total"
	ebsVolumeQueueLength   = "aws_ebs_csi_volume_queue_length"
)

var MetricToUnit = map[string]string{
	ebsReadOpsTotal:        containerinsight.UnitCount,
	ebsWriteOpsTotal:       containerinsight.UnitCount,
	ebsReadBytesTotal:      containerinsight.UnitBytes,
	ebsWriteBytesTotal:     containerinsight.UnitBytes,
	ebsReadTime:            containerinsight.UnitSecond,
	ebsWriteTime:           containerinsight.UnitSecond,
	ebsExceededIOPSTime:    containerinsight.UnitSecond,
	ebsExceededTPTime:      containerinsight.UnitSecond,
	ebsExceededEC2IOPSTime: containerinsight.UnitSecond,
	ebsExceededEC2TPTime:   containerinsight.UnitSecond,
	ebsVolumeQueueLength:   containerinsight.UnitCount,
}
