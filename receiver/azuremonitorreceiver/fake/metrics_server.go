// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// TODO remove this package in favor of the new arriving feature in Azure SDK for Go.
//  Ref: https://github.com/Azure/azure-sdk-for-go/pull/24309

package fake // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver/fake"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	azfake "github.com/Azure/azure-sdk-for-go/sdk/azcore/fake"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/fake/server"
	"github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azmetrics"
)

// MetricsServer is a fake server for instances of the azmetrics.MetricsClient type.
type MetricsServer struct {
	// QueryResources is the fake for method MetricsClient.List
	// HTTP status codes to indicate success: http.StatusOK
	QueryResources func(ctx context.Context, subscriptionID, metricNamespace string, metricNames []string, resourceIDs azmetrics.ResourceIDList, options *azmetrics.QueryResourcesOptions) (resp azfake.Responder[azmetrics.QueryResourcesResponse], errResp azfake.ErrorResponder)
}

// NewMetricsServerTransport creates a new instance of MetricsServerTransport with the provided implementation.
// The returned MetricsServerTransport instance is connected to an instance of azmetrics.MetricsClient via the
// azcore.ClientOptions.Transporter field in the client's constructor parameters.
func NewMetricsServerTransport(srv *MetricsServer) *MetricsServerTransport {
	return &MetricsServerTransport{srv: srv}
}

// MetricsServerTransport connects instances of armmonitor.MetricsClient to instances of MetricsServer.
// Don't use this type directly, use NewMetricsServerTransport instead.
type MetricsServerTransport struct {
	srv *MetricsServer
}

// Do implements the policy.Transporter interface for MetricsServerTransport.
func (m *MetricsServerTransport) Do(req *http.Request) (*http.Response, error) {
	// rawMethod := req.Context().Value(runtime.CtxAPINameKey{})
	// method, ok := rawMethod.(string)
	// if !ok {
	// 	return nil, nonRetriableError{errors.New("unable to dispatch request, missing value for CtxAPINameKey")}
	// }
	//
	// var resp *http.Response
	// var err error
	//
	// switch method {
	// case "Client.QueryResources":
	// 	resp, err = m.dispatchQueryResources(req)
	// default:
	// 	err = fmt.Errorf("unhandled API %s", method)
	// }
	//
	// if err != nil {
	// 	return nil, err
	// }
	//
	// return resp, nil

	// We can't directly reuse the logic as the runtime.CtxAPINameKey is not send by the client.
	return m.dispatchQueryResources(req)
}

func (m *MetricsServerTransport) dispatchQueryResources(req *http.Request) (*http.Response, error) {
	if m.srv.QueryResources == nil {
		return nil, &nonRetriableError{errors.New("fake for method QueryResources not implemented")}
	}

	const regexStr = `/subscriptions/(?P<subscriptionId>[!#&$-;=?-\[\]_a-zA-Z0-9~%@]+)/metrics:getBatch`
	regex := regexp.MustCompile(regexStr)
	matches := regex.FindStringSubmatch(req.URL.EscapedPath())
	if len(matches) < 1 {
		return nil, fmt.Errorf("failed to parse path %s", req.URL.Path)
	}
	qp := req.URL.Query()
	body, err := server.UnmarshalRequestAsJSON[azmetrics.ResourceIDList](req)
	if err != nil {
		return nil, err
	}
	subscriptionIDParam, err := url.PathUnescape(matches[regex.SubexpIndex("subscriptionId")])
	if err != nil {
		return nil, err
	}
	startTimeUnescaped, err := url.QueryUnescape(qp.Get("startTime"))
	if err != nil {
		return nil, err
	}
	startTimeParam := getOptional(startTimeUnescaped)
	endTimeUnescaped, err := url.QueryUnescape(qp.Get("endTime"))
	if err != nil {
		return nil, err
	}
	endTimeParam := getOptional(endTimeUnescaped)
	intervalUnescaped, err := url.QueryUnescape(qp.Get("interval"))
	if err != nil {
		return nil, err
	}
	intervalParam := getOptional(intervalUnescaped)
	metricnamesUnescaped, err := url.QueryUnescape(qp.Get("metricnames"))
	if err != nil {
		return nil, err
	}
	aggregationUnescaped, err := url.QueryUnescape(qp.Get("aggregation"))
	if err != nil {
		return nil, err
	}
	aggregationParam := getOptional(aggregationUnescaped)
	topUnescaped, err := url.QueryUnescape(qp.Get("top"))
	if err != nil {
		return nil, err
	}
	topParam, err := parseOptional(topUnescaped, func(v string) (int32, error) {
		p, parseErr := strconv.ParseInt(v, 10, 32)
		if parseErr != nil {
			return 0, parseErr
		}
		return int32(p), nil
	})
	if err != nil {
		return nil, err
	}
	orderbyUnescaped, err := url.QueryUnescape(qp.Get("orderby"))
	if err != nil {
		return nil, err
	}
	orderbyParam := getOptional(orderbyUnescaped)
	rollupbyUnescaped, err := url.QueryUnescape(qp.Get("rollupby"))
	if err != nil {
		return nil, err
	}
	rollupbybyParam := getOptional(rollupbyUnescaped)
	filterUnescaped, err := url.QueryUnescape(qp.Get("filter"))
	if err != nil {
		return nil, err
	}
	filterParam := getOptional(filterUnescaped)
	metricnamespaceUnescaped, err := url.QueryUnescape(qp.Get("metricnamespace"))
	if err != nil {
		return nil, err
	}
	var options *azmetrics.QueryResourcesOptions
	if startTimeParam != nil || endTimeParam != nil || intervalParam != nil || aggregationParam != nil || topParam != nil || orderbyParam != nil || rollupbybyParam != nil || filterParam != nil {
		options = &azmetrics.QueryResourcesOptions{
			StartTime:   startTimeParam,
			EndTime:     endTimeParam,
			Interval:    intervalParam,
			Aggregation: aggregationParam,
			Top:         topParam,
			OrderBy:     orderbyParam,
			RollUpBy:    rollupbybyParam,
			Filter:      filterParam,
		}
	}
	respr, errRespr := m.srv.QueryResources(req.Context(), subscriptionIDParam, metricnamespaceUnescaped, strings.Split(metricnamesUnescaped, ","), body, options)
	if respErr := server.GetError(errRespr, req); respErr != nil {
		return nil, respErr
	}
	respContent := server.GetResponseContent(respr)
	if !contains([]int{http.StatusOK}, respContent.HTTPStatus) {
		return nil, &nonRetriableError{fmt.Errorf("unexpected status code %d. acceptable values are http.StatusOK", respContent.HTTPStatus)}
	}
	resp, err := server.MarshalResponseAsJSON(respContent, server.GetResponse(respr), req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
