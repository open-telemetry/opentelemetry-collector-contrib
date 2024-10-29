// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudlogsreceiver/internal"

import (
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/lts/v2/model"
	"go.opentelemetry.io/collector/pdata/plog"
)

func ConvertLTSLogsToOTLP(projectID, regionID, groupID, streamD string, ltsLogs []model.LogContents) plog.Logs {
	logs := plog.NewLogs()
	resourceLog := logs.ResourceLogs().AppendEmpty()

	resource := resourceLog.Resource()
	resourceAttr := resource.Attributes()
	resourceAttr.PutStr("cloud.provider", "huawei_cloud")
	resourceAttr.PutStr("project.id", projectID)
	resourceAttr.PutStr("region.id", regionID)
	resourceAttr.PutStr("group.id", groupID)
	resourceAttr.PutStr("stream.id", streamD)

	if len(ltsLogs) == 0 {
		return logs
	}
	scopedLog := resourceLog.ScopeLogs().AppendEmpty()
	scopedLog.Scope().SetName("huawei_cloud_lts")
	scopedLog.Scope().SetVersion("v2")
	for _, ltsLog := range ltsLogs {

		logRecord := scopedLog.LogRecords().AppendEmpty()
		if ltsLog.Content != nil {
			logRecord.Body().SetStr(*ltsLog.Content)
		}
		for key, value := range ltsLog.Labels {
			logRecord.Attributes().PutStr(key, value)
		}
		if ltsLog.LineNum != nil {
			logRecord.Attributes().PutStr("lineNum", *ltsLog.LineNum)
		}
	}

	return logs
}
