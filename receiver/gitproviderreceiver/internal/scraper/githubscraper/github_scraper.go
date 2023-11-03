// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitproviderreceiver/internal/scraper/githubscraper"

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/Khan/genqlient/graphql"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitproviderreceiver/internal/metadata"
)

var (
	errClientNotInitErr = errors.New("http client not initialized")
)

type githubScraper struct {
	client   *http.Client
	cfg      *Config
	settings component.TelemetrySettings
	logger   *zap.Logger
	mb       *metadata.MetricsBuilder
}

func (ghs *githubScraper) start(_ context.Context, host component.Host) (err error) {
	ghs.logger.Sugar().Info("starting the GitHub scraper")
	ghs.client, err = ghs.cfg.ToClient(host, ghs.settings)
	return
}

func newGitHubScraper(
	_ context.Context,
	settings receiver.CreateSettings,
	cfg *Config,
) *githubScraper {
	return &githubScraper{
		cfg:      cfg,
		settings: settings.TelemetrySettings,
		logger:   settings.Logger,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
	}
}

// scrape and return github metrics
func (ghs *githubScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if ghs.client == nil {
		return pmetric.NewMetrics(), errClientNotInitErr
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	ghs.logger.Sugar().Debug("current time", zap.Time("now", now.AsTime()))

	currentDate := time.Now().Day()
	ghs.logger.Sugar().Debugf("current date: %v", currentDate)

	// Enable the ability to override the endpoint for self-hosted github instances
	// GitHub Free URL : https://api.github.com/graphql
	// https://docs.github.com/en/graphql/guides/forming-calls-with-graphql#the-graphql-endpoint
	graphCURL := "https://api.github.com/graphql"

	if ghs.cfg.HTTPClientSettings.Endpoint != "" {
		var err error

		// GitHub Enterprise (ghe) URL : http(s)://HOSTNAME/api/graphql
		// https://docs.github.com/en/enterprise-server@3.8/graphql/guides/forming-calls-with-graphql#the-graphql-endpoint
		graphCURL, err = url.JoinPath(ghs.cfg.HTTPClientSettings.Endpoint, "api/graphql")
		if err != nil {
			ghs.logger.Sugar().Errorf("error: %v", err)
		}
	}
	ghs.logger.Sugar().Debugf("GitHub GraphQL endpoint URL set to: %v", graphCURL)

	genClient := graphql.NewClient(graphCURL, ghs.client)

	// Do some basic validation to ensure the values provided actually exist in github
	// prior to making queries against that org or user value
	exists, ownertype, err := ghs.checkOwnerExists(ctx, genClient, ghs.cfg.GitHubOrg)
	if err != nil {
		ghs.logger.Sugar().Errorf("error checking if owner exists", zap.Error(err))
	}

	typeValid, err := checkOwnerTypeValid(ownertype)
	if err != nil {
		ghs.logger.Sugar().Errorf("error checking if owner type is valid", zap.Error(err))
	}

	if !exists || !typeValid {
		ghs.logger.Sugar().Error("error logging in and getting data from github")
		return ghs.mb.Emit(), err
	}

	// Generate the search query based on the type, org/user name, and the search_query
	// value if provided
	sq := genDefaultSearchQuery(ownertype, ghs.cfg.GitHubOrg)

	if ghs.cfg.SearchQuery != "" {
		sq = ghs.cfg.SearchQuery
		ghs.logger.Sugar().Debugf("using search query where query is: %v", ghs.cfg.SearchQuery)
	}

	// Get the repository data based on the search query retrieving a slice of branches
	// and the recording the total count of repositories
	repos, count, err := ghs.getRepos(ctx, genClient, sq)
	if err != nil {
		ghs.logger.Sugar().Errorf("error getting repo data", zap.Error(err))
		return ghs.mb.Emit(), err
	}

	ghs.mb.RecordGitRepositoryCountDataPoint(now, int64(count))

	// Get the branch count (future branch data) for each repo and record the given metrics
	for _, repo := range repos {
		name := repo.(*SearchNodeRepository).Name
		trunk := repo.(*SearchNodeRepository).DefaultBranchRef.Name

		count, err := ghs.getBranches(ctx, genClient, name, ghs.cfg.GitHubOrg, trunk)
		if err != nil {
			ghs.logger.Sugar().Errorf("error getting branch count", zap.Error(err))
			return ghs.mb.Emit(), err
		}
		ghs.mb.RecordGitRepositoryBranchCountDataPoint(now, int64(count), name)

	}

	return ghs.mb.Emit(), nil
}
