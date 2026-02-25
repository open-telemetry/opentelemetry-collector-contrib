// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signalfxexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter"

import (
	"context"
	"io"
	"net/http"
	"path"
	"strings"

	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

// sfxEventClient sends the data to the SignalFx backend.
type sfxEventClient struct {
	sfxClientBase
	logger                 *zap.Logger
	accessTokenPassthrough bool
}

func (s *sfxEventClient) pushEvents(ctx context.Context, rl plog.ResourceLogs, sfxEvents []*sfxpb.Event) error {
	accessToken := s.retrieveAccessToken(ctx, rl)

	body, compressed, err := s.encodeBody(sfxEvents)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	eventURL := *s.ingestURL
	if !strings.HasSuffix(eventURL.Path, "v2/event") {
		eventURL.Path = path.Join(eventURL.Path, "v2/event")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, eventURL.String(), body)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	for k, v := range s.headers {
		req.Header.Set(k, v)
	}

	if s.accessTokenPassthrough && accessToken != "" {
		req.Header.Set(splunk.SFxAccessTokenHeader, accessToken)
	}

	if compressed {
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	err = splunk.HandleHTTPCode(resp)
	if err != nil {
		return err
	}

	return nil
}

func (s *sfxEventClient) encodeBody(events []*sfxpb.Event) (bodyReader io.Reader, compressed bool, err error) {
	msg := sfxpb.EventUploadMessage{
		Events: events,
	}
	body, err := msg.Marshal()
	if err != nil {
		return nil, false, err
	}
	return s.getReader(body)
}

func (s *sfxEventClient) retrieveAccessToken(ctx context.Context, rl plog.ResourceLogs) string {
	if !s.accessTokenPassthrough {
		// Nothing to do if token is pass through not configured or resource is nil.
		return ""
	}

	cl := client.FromContext(ctx)
	ss := cl.Metadata.Get(splunk.SFxAccessTokenHeader)
	if len(ss) > 0 {
		return ss[0]
	}

	attrs := rl.Resource().Attributes()
	if accessToken, ok := attrs.Get(splunk.SFxAccessTokenLabel); ok && accessToken.Type() == pcommon.ValueTypeStr {
		return accessToken.Str()
	}
	return ""
}
