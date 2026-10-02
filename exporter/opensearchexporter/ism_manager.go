// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/opensearch-project/opensearch-go/v4/plugins/ism"
	"go.uber.org/zap"
)

const (
	defaultRolloverMinSize     = "50gb"
	defaultRolloverMinIndexAge = "24h"
	defaultRolloverPriority    = 100
)

// ismManager creates the ISM rollover policy and initial write-aliased index for an otel-v1
// index. It is best-effort: like templateManager, it logs and returns on cluster errors rather
// than failing the exporter Start(), so a transient control-plane hiccup does not block the
// data path.
type ismManager struct {
	client    *opensearchapi.Client
	ismClient *ism.Client
	logger    *zap.Logger
	cfg       ISMConfig

	// customPolicy is the pre-loaded policy body when PolicyFile is set; nil otherwise.
	customPolicy *ism.PoliciesPutBody
}

// newISMManager pre-loads the optional custom policy file eagerly. A configured-but-unreadable
// or invalid policy file is a configuration error and fails startup. Client construction and the
// cluster-side policy/index calls remain best-effort (logged, non-fatal).
func newISMManager(endpoint string, transport http.RoundTripper, client *opensearchapi.Client, ismCfg ISMConfig, logger *zap.Logger) (*ismManager, error) {
	m := &ismManager{
		client: client,
		logger: logger,
		cfg:    ismCfg,
	}

	if ismCfg.PolicyFile != "" {
		body, err := loadPolicyFile(ismCfg.PolicyFile)
		if err != nil {
			return nil, err
		}
		m.customPolicy = &body
	}

	ismClient, err := ism.NewClient(ism.Config{
		Client: opensearch.Config{
			Addresses:    []string{endpoint},
			Transport:    transport,
			DisableRetry: true,
		},
	})
	if err != nil {
		// Best-effort: leave ismClient nil and skip ISM setup rather than failing Start().
		logger.Warn("Failed to create ISM client; skipping ISM setup", zap.Error(err))
	}
	m.ismClient = ismClient

	return m, nil
}

// setupISM creates the ISM policy and initial index with write alias for the given index alias.
func (m *ismManager) setupISM(ctx context.Context, indexAlias string) {
	if m.ismClient == nil {
		return
	}
	policyName := indexAlias + "-policy"
	m.ensurePolicy(ctx, policyName, indexAlias)
	m.ensureInitialIndex(ctx, indexAlias)
}

func (m *ismManager) ensurePolicy(ctx context.Context, policyName, indexAlias string) {
	req := ism.PoliciesPutReq{
		Policy: policyName,
		Body:   m.buildPolicyBody(indexAlias),
	}
	_, err := m.ismClient.Policies.Put(ctx, req)
	if err != nil {
		if strings.Contains(err.Error(), "version_conflict_engine_exception") ||
			strings.Contains(err.Error(), "resource_already_exists_exception") {
			m.logger.Debug("ISM policy already exists", zap.String("policy", policyName))
			return
		}
		m.logger.Warn("Failed to create ISM policy", zap.String("policy", policyName), zap.Error(err))
		return
	}
	m.logger.Info("Created ISM policy", zap.String("policy", policyName))
}

func (m *ismManager) buildPolicyBody(indexAlias string) ism.PoliciesPutBody {
	if m.customPolicy != nil {
		return *m.customPolicy
	}

	minSize := m.cfg.RolloverMinSize
	if minSize == "" {
		minSize = defaultRolloverMinSize
	}
	minAge := m.cfg.RolloverMinIndexAge
	if minAge == "" {
		minAge = defaultRolloverMinIndexAge
	}
	priority := m.cfg.RolloverPriority
	if priority == 0 {
		priority = defaultRolloverPriority
	}

	return ism.PoliciesPutBody{
		Policy: ism.PolicyBody{
			Description:  fmt.Sprintf("Rollover policy for %s", indexAlias),
			DefaultState: "current_write_index",
			States: []ism.PolicyState{
				{
					Name: "current_write_index",
					Actions: []ism.PolicyStateAction{
						{
							Rollover: &ism.PolicyStateRollover{
								MinSize:     minSize,
								MinIndexAge: minAge,
							},
						},
					},
				},
			},
			Template: []ism.Template{
				{
					IndexPatterns: []string{indexAlias + "-*"},
					Priority:      priority,
				},
			},
		},
	}
}

func loadPolicyFile(path string) (ism.PoliciesPutBody, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ism.PoliciesPutBody{}, fmt.Errorf("reading ISM policy file: %w", err)
	}
	var body ism.PoliciesPutBody
	if err := json.Unmarshal(data, &body); err != nil {
		return ism.PoliciesPutBody{}, fmt.Errorf("parsing ISM policy file: %w", err)
	}
	return body, nil
}

func (m *ismManager) ensureInitialIndex(ctx context.Context, indexAlias string) {
	existsReq := opensearchapi.IndicesExistsReq{Indices: []string{indexAlias}}
	_, err := m.client.Indices.Exists(ctx, existsReq)
	if err == nil {
		m.logger.Debug("Index/alias already exists", zap.String("alias", indexAlias))
		return
	}

	initialIndex := indexAlias + "-000001"
	body := fmt.Sprintf(`{"aliases":{%q:{"is_write_index":true}}}`, indexAlias)
	createReq := opensearchapi.IndicesCreateReq{
		Index: initialIndex,
		Body:  strings.NewReader(body),
	}
	_, createErr := m.client.Indices.Create(ctx, createReq)
	if createErr != nil {
		if strings.Contains(createErr.Error(), "resource_already_exists_exception") {
			m.logger.Debug("Initial index already exists", zap.String("index", initialIndex))
			return
		}
		m.logger.Warn("Failed to create initial index", zap.String("index", initialIndex), zap.Error(createErr))
		return
	}
	m.logger.Info("Created initial index with write alias",
		zap.String("index", initialIndex),
		zap.String("alias", indexAlias))
}
