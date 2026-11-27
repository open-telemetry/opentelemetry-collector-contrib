// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuredataexplorerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/Azure/azure-kusto-go/azkustodata"
	trustedEndpoints "github.com/Azure/azure-kusto-go/azkustodata/trusted_endpoints"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"go.uber.org/zap"
)

const (
	publicKustoAudience = "https://kusto.kusto.windows.net"
	publicLoginEndpoint = "https://login.microsoftonline.com"
)

type ingestV2Authentication struct {
	mu         sync.Mutex
	config     Config
	dmURL      string
	httpClient *http.Client
	logger     *zap.Logger
	credential azcore.TokenCredential
	scope      string
}

func (a *ingestV2Authentication) initialize(ctx context.Context) (azcore.TokenCredential, string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.credential != nil {
		return a.credential, a.scope, nil
	}
	info, err := ingestV2CloudInfo(ctx, a.dmURL, a.httpClient, a.logger)
	if err != nil {
		return nil, "", err
	}
	if err := trustedEndpoints.Instance.ValidateTrustedEndpoint(a.dmURL, info.LoginEndpoint); err != nil {
		return nil, "", err
	}
	credential, err := newIngestV2Credential(&a.config, info.LoginEndpoint, a.httpClient)
	if err != nil {
		return nil, "", err
	}
	a.credential = credential
	// V2 uses the advertised audience without the legacy MFA audience rewrite.
	a.scope = info.KustoServiceResourceID + "/.default"
	return a.credential, a.scope, nil
}

func (a *ingestV2Authentication) token(ctx context.Context) (string, error) {
	credential, scope, err := a.initialize(ctx)
	if err != nil {
		return "", err
	}
	token, err := credential.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{scope}})
	if err != nil {
		return "", err
	}
	return token.Token, nil
}

// GetToken preserves the storage scope requested by the SDK's OneLake uploader.
func (a *ingestV2Authentication) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	credential, _, err := a.initialize(ctx)
	if err != nil {
		return azcore.AccessToken{}, err
	}
	return credential.GetToken(ctx, opts)
}

func newIngestV2Credential(cfg *Config, loginEndpoint string, httpClient *http.Client) (azcore.TokenCredential, error) {
	opts := azcore.ClientOptions{
		Cloud:     cloud.Configuration{ActiveDirectoryAuthorityHost: loginEndpoint},
		Transport: httpClient,
	}
	switch {
	case cfg.UseAzureAuth:
		return azidentity.NewDefaultAzureCredential(&azidentity.DefaultAzureCredentialOptions{ClientOptions: opts})
	case !isEmpty(cfg.ManagedIdentityID):
		miOpts := &azidentity.ManagedIdentityCredentialOptions{ClientOptions: opts}
		if !strings.EqualFold(strings.TrimSpace(cfg.ManagedIdentityID), "SYSTEM") {
			miOpts.ID = azidentity.ClientID(strings.TrimSpace(cfg.ManagedIdentityID))
		}
		return azidentity.NewManagedIdentityCredential(miOpts)
	default:
		return azidentity.NewClientSecretCredential(cfg.TenantID, cfg.ApplicationID, string(cfg.ApplicationKey),
			&azidentity.ClientSecretCredentialOptions{ClientOptions: opts})
	}
}

func ingestV2CloudInfo(ctx context.Context, dmURL string, client *http.Client, logger *zap.Logger) (azkustodata.CloudInfo, error) {
	defaults := azkustodata.CloudInfo{LoginEndpoint: publicLoginEndpoint, KustoServiceResourceID: publicKustoAudience}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dmURL+"/v1/rest/auth/metadata", nil)
	if err != nil {
		return azkustodata.CloudInfo{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return azkustodata.CloudInfo{}, fmt.Errorf("read DM authentication metadata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Warn("DM authentication metadata returned an error; using public cloud defaults", zap.Int("status", resp.StatusCode))
		return defaults, nil
	}
	var response struct {
		AzureAD *azkustodata.CloudInfo
	}
	const maxMetadataSize = 1024 * 1024
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMetadataSize+1))
	if err != nil {
		return azkustodata.CloudInfo{}, fmt.Errorf("read DM authentication metadata body: %w", err)
	}
	if len(body) > maxMetadataSize {
		return azkustodata.CloudInfo{}, fmt.Errorf("DM authentication metadata exceeds size limit")
	}
	if len(body) == 0 {
		logger.Warn("DM authentication metadata is empty; using public cloud defaults")
		return defaults, nil
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return azkustodata.CloudInfo{}, fmt.Errorf("decode DM authentication metadata: %w", err)
	}
	if response.AzureAD == nil {
		logger.Warn("DM authentication metadata has no AzureAD settings; using public cloud defaults")
		return defaults, nil
	}
	info := *response.AzureAD
	if info.LoginEndpoint == "" {
		logger.Warn("DM authentication metadata has no login endpoint; using the public login endpoint")
		info.LoginEndpoint = publicLoginEndpoint
	}
	if info.KustoServiceResourceID == "" {
		logger.Warn("DM authentication metadata has no Kusto audience; using the public Kusto audience")
		info.KustoServiceResourceID = publicKustoAudience
	}
	return info, nil
}
