package akamaisecurityeventsreceiver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/akamai/AkamaiOPEN-edgegrid-golang/edgegrid"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/akamaisecurityeventsreceiver/internal/metadata"
)

type akamaiSecurityEventsReceiver struct {
	settings receiver.Settings
	logger   *zap.Logger
	cfg      *Config

	edgegridCfg   edgegrid.Config
	httpClient    *http.Client
	storageClient storage.Client
	offset        string

	lb *metadata.LogsBuilder
}

func newAkamaiSecurityEventsScraper(
	settings receiver.Settings,
	cfg *Config,
) *akamaiSecurityEventsReceiver {
	a := &akamaiSecurityEventsReceiver{
		logger:   settings.Logger,
		settings: settings,
		cfg:      cfg,
		edgegridCfg: edgegrid.Config{
			ClientToken:  cfg.ClientToken,
			ClientSecret: cfg.ClientSecret,
			AccessToken:  cfg.AccessToken,
		},
		lb: metadata.NewLogsBuilder(settings),
	}

	return a
}

func (r *akamaiSecurityEventsReceiver) start(ctx context.Context, host component.Host) error {
	httpClient, err := r.cfg.ToClient(ctx, host, r.settings.TelemetrySettings)
	if err != nil {
		return err
	}
	r.httpClient = httpClient

	if r.cfg.StorageID != nil {
		extension, ok := host.GetExtensions()[*r.cfg.StorageID]
		if !ok {
			return fmt.Errorf("storage extension '%s' not found", r.cfg.StorageID)
		}

		storageExtension, ok := extension.(storage.Extension)
		if !ok {
			return fmt.Errorf("non-storage extension '%s' found", r.cfg.StorageID)
		}

		storageClient, err := storageExtension.GetClient(ctx, component.KindReceiver, r.settings.ID, "")
		if err != nil {
			return fmt.Errorf("failed to get storage client: %w", err)
		}

		r.storageClient = storageClient
	} else {
		r.storageClient = storage.NewNopClient()
	}

	bytes, err := r.storageClient.Get(ctx, "offset")
	if err != nil {
		return fmt.Errorf("failed to get offset: %w", err)
	}
	r.offset = string(bytes)

	if r.offset == "" {
		r.offset = "NULL"
	}

	r.logger.Info("Akamai Security Events Receiver started", zap.String("offset", r.offset))
	return nil
}

func (r *akamaiSecurityEventsReceiver) shutdown(ctx context.Context) error {
	return r.storageClient.Close(ctx)
}

func (r *akamaiSecurityEventsReceiver) scrape(ctx context.Context) (plog.Logs, error) {
	url := fmt.Sprintf("%s/siem/v1/configs/%s?offset=%s&limit=%d", r.cfg.Endpoint, r.cfg.ConfigIds, r.offset, r.cfg.Limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to create request: %w", err)
	}

	req = edgegrid.AddRequestHeader(r.edgegridCfg, req)

	r.logger.Debug("Scraping Akamai Security Events", zap.String("url", url), zap.String("offset", r.offset))
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to send request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return plog.Logs{}, fmt.Errorf("failed to send request, status code: %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	for decoder.More() {
		var evt map[string]any
		err := decoder.Decode(&evt)
		if err != nil {
			return plog.Logs{}, fmt.Errorf("failed to unmarshal akamai-security event: %w", err)
		}

		if _, ok := evt["offset"]; ok {
			r.offset = evt["offset"].(string)
			break
		}

		log := plog.NewLogRecord()
		err = log.Attributes().FromRaw(evt)
		if err != nil {
			return plog.Logs{}, fmt.Errorf("failed to set attributes from event: %w", err)
		}

		var ts pcommon.Timestamp
		if httpMessage, ok := log.Attributes().Get("httpMessage"); ok {
			if start, ok := httpMessage.Map().Get("start"); ok {
				if startStr := start.Str(); startStr != "" {
					if startInt, err := strconv.ParseInt(startStr, 10, 64); err == nil {
						ts = pcommon.NewTimestampFromTime(time.Unix(startInt, 0))
					}
				}
			}
		}

		ruleSlice := parseRuleData(log)
		if r.cfg.FlattenRuleData {
			for i := 0; i < ruleSlice.Len(); i++ {
				ruleData := ruleSlice.At(i)
				if ruleData.Type() != pcommon.ValueTypeMap {
					continue
				}

				newlog := plog.NewLogRecord()
				log.Attributes().CopyTo(newlog.Attributes())
				ruleData.Map().CopyTo(newlog.Attributes().PutEmptyMap("parsedRuleData"))
				newlog.SetTimestamp(ts)
				newlog.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))
				r.lb.AppendLogRecord(newlog)
			}
		} else {
			ruleSlice.MoveAndAppendTo(log.Attributes().PutEmptySlice("parsedRuleData"))
			log.SetTimestamp(ts)
			log.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))
			r.lb.AppendLogRecord(log)
		}
	}

	err = r.storageClient.Set(ctx, "offset", []byte(r.offset))
	if err != nil {
		return r.lb.Emit(), fmt.Errorf("failed to set offset: %w", err)
	}
	r.logger.Debug("Updated offset", zap.String("offset", r.offset))

	return r.lb.Emit(), nil
}

func parseRuleData(log plog.LogRecord) pcommon.Slice {
	attackDataAttr, exists := log.Attributes().Get("attackData")
	if !exists {
		return pcommon.Slice{}
	}

	attackDataMap := attackDataAttr.Map().AsRaw()
	ruleFields := make(map[string][]string)
	var ruleCount int

	for key, value := range attackDataMap {
		if !strings.HasPrefix(key, "rule") {
			continue
		}

		valueStr, ok := value.(string)
		if !ok || valueStr == "" {
			continue
		}

		urlDecoded, err := url.QueryUnescape(valueStr)
		if err != nil {
			continue
		}

		parts := strings.Split(urlDecoded, ";")
		if ruleCount == 0 {
			ruleCount = len(parts)
		}

		var decodedParts []string
		for _, part := range parts {
			if part == "" {
				decodedParts = append(decodedParts, "")
				continue
			}

			decoded, err := base64.StdEncoding.DecodeString(part)
			if err != nil {
				decodedParts = append(decodedParts, "")
				continue
			}
			decodedParts = append(decodedParts, string(decoded))
		}

		singularKey := key
		if strings.HasSuffix(key, "s") {
			singularKey = key[:len(key)-1]
		}

		ruleFields[singularKey] = decodedParts
	}

	if ruleCount == 0 {
		return pcommon.Slice{}
	}

	rules := make([]map[string]interface{}, ruleCount)
	for i := 0; i < ruleCount; i++ {
		rules[i] = make(map[string]interface{})
	}

	for fieldName, values := range ruleFields {
		for i, value := range values {
			if i < ruleCount {
				rules[i][fieldName] = value
			}
		}
	}

	rulesSlice := pcommon.NewSlice()
	for _, rule := range rules {
		ruleAttr := rulesSlice.AppendEmpty()
		if err := ruleAttr.FromRaw(rule); err != nil {
			continue
		}
	}
	return rulesSlice
}
