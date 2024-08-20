// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver/internal"

import (
	ces "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ces/v1"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ces/v1/model"
)

// This interface should have all the function defined inside https://github.com/huaweicloud/huaweicloud-sdk-go-v3/blob/v0.1.110/services/ces/v1/ces_client.go
// Check https://github.com/vektra/mockery on how to install it on your machine.
//
//go:generate mockery --name CesClient --case=underscore --output=./mocks
type CesClient interface {
	BatchListMetricData(request *model.BatchListMetricDataRequest) (*model.BatchListMetricDataResponse, error)
	BatchListMetricDataInvoker(request *model.BatchListMetricDataRequest) *ces.BatchListMetricDataInvoker
	CreateAlarm(request *model.CreateAlarmRequest) (*model.CreateAlarmResponse, error)
	CreateAlarmInvoker(request *model.CreateAlarmRequest) *ces.CreateAlarmInvoker
	CreateAlarmTemplate(request *model.CreateAlarmTemplateRequest) (*model.CreateAlarmTemplateResponse, error)
	CreateAlarmTemplateInvoker(request *model.CreateAlarmTemplateRequest) *ces.CreateAlarmTemplateInvoker
	CreateEvents(request *model.CreateEventsRequest) (*model.CreateEventsResponse, error)
	CreateEventsInvoker(request *model.CreateEventsRequest) *ces.CreateEventsInvoker
	CreateMetricData(request *model.CreateMetricDataRequest) (*model.CreateMetricDataResponse, error)
	CreateMetricDataInvoker(request *model.CreateMetricDataRequest) *ces.CreateMetricDataInvoker
	CreateResourceGroup(request *model.CreateResourceGroupRequest) (*model.CreateResourceGroupResponse, error)
	CreateResourceGroupInvoker(request *model.CreateResourceGroupRequest) *ces.CreateResourceGroupInvoker
	DeleteAlarm(request *model.DeleteAlarmRequest) (*model.DeleteAlarmResponse, error)
	DeleteAlarmInvoker(request *model.DeleteAlarmRequest) *ces.DeleteAlarmInvoker
	DeleteAlarmTemplate(request *model.DeleteAlarmTemplateRequest) (*model.DeleteAlarmTemplateResponse, error)
	DeleteAlarmTemplateInvoker(request *model.DeleteAlarmTemplateRequest) *ces.DeleteAlarmTemplateInvoker
	DeleteResourceGroup(request *model.DeleteResourceGroupRequest) (*model.DeleteResourceGroupResponse, error)
	DeleteResourceGroupInvoker(request *model.DeleteResourceGroupRequest) *ces.DeleteResourceGroupInvoker
	ListAlarmHistories(request *model.ListAlarmHistoriesRequest) (*model.ListAlarmHistoriesResponse, error)
	ListAlarmHistoriesInvoker(request *model.ListAlarmHistoriesRequest) *ces.ListAlarmHistoriesInvoker
	ListAlarmTemplates(request *model.ListAlarmTemplatesRequest) (*model.ListAlarmTemplatesResponse, error)
	ListAlarmTemplatesInvoker(request *model.ListAlarmTemplatesRequest) *ces.ListAlarmTemplatesInvoker
	ListAlarms(request *model.ListAlarmsRequest) (*model.ListAlarmsResponse, error)
	ListAlarmsInvoker(request *model.ListAlarmsRequest) *ces.ListAlarmsInvoker
	ListEventDetail(request *model.ListEventDetailRequest) (*model.ListEventDetailResponse, error)
	ListEventDetailInvoker(request *model.ListEventDetailRequest) *ces.ListEventDetailInvoker
	ListEvents(request *model.ListEventsRequest) (*model.ListEventsResponse, error)
	ListEventsInvoker(request *model.ListEventsRequest) *ces.ListEventsInvoker
	ListMetrics(request *model.ListMetricsRequest) (*model.ListMetricsResponse, error)
	ListMetricsInvoker(request *model.ListMetricsRequest) *ces.ListMetricsInvoker
	ListResourceGroup(request *model.ListResourceGroupRequest) (*model.ListResourceGroupResponse, error)
	ListResourceGroupInvoker(request *model.ListResourceGroupRequest) *ces.ListResourceGroupInvoker
	ShowAlarm(request *model.ShowAlarmRequest) (*model.ShowAlarmResponse, error)
	ShowAlarmInvoker(request *model.ShowAlarmRequest) *ces.ShowAlarmInvoker
	ShowEventData(request *model.ShowEventDataRequest) (*model.ShowEventDataResponse, error)
	ShowEventDataInvoker(request *model.ShowEventDataRequest) *ces.ShowEventDataInvoker
	ShowMetricData(request *model.ShowMetricDataRequest) (*model.ShowMetricDataResponse, error)
	ShowMetricDataInvoker(request *model.ShowMetricDataRequest) *ces.ShowMetricDataInvoker
	ShowQuotas(request *model.ShowQuotasRequest) (*model.ShowQuotasResponse, error)
	ShowQuotasInvoker(request *model.ShowQuotasRequest) *ces.ShowQuotasInvoker
	ShowResourceGroup(request *model.ShowResourceGroupRequest) (*model.ShowResourceGroupResponse, error)
	ShowResourceGroupInvoker(request *model.ShowResourceGroupRequest) *ces.ShowResourceGroupInvoker
	UpdateAlarm(request *model.UpdateAlarmRequest) (*model.UpdateAlarmResponse, error)
	UpdateAlarmAction(request *model.UpdateAlarmActionRequest) (*model.UpdateAlarmActionResponse, error)
	UpdateAlarmActionInvoker(request *model.UpdateAlarmActionRequest) *ces.UpdateAlarmActionInvoker
	UpdateAlarmInvoker(request *model.UpdateAlarmRequest) *ces.UpdateAlarmInvoker
	UpdateAlarmTemplate(request *model.UpdateAlarmTemplateRequest) (*model.UpdateAlarmTemplateResponse, error)
	UpdateAlarmTemplateInvoker(request *model.UpdateAlarmTemplateRequest) *ces.UpdateAlarmTemplateInvoker
	UpdateResourceGroup(request *model.UpdateResourceGroupRequest) (*model.UpdateResourceGroupResponse, error)
	UpdateResourceGroupInvoker(request *model.UpdateResourceGroupRequest) *ces.UpdateResourceGroupInvoker
}
