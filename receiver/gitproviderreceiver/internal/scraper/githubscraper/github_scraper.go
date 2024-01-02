// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate ../../../../../.tools/genqlient

package githubscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitproviderreceiver/internal/scraper/githubscraper"

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

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
	rb       *metadata.ResourceBuilder
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
		rb:       metadata.NewResourceBuilder(cfg.ResourceAttributes),
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

	genClient, restClient, err := ghs.createClients()
	if err != nil {
		ghs.logger.Sugar().Errorf("unable to create clients", zap.Error(err))
	}

	// Do some basic validation to ensure the values provided actually exist in github
	// prior to making queries against that org or user value
	loginType, err := ghs.login(ctx, genClient, ghs.cfg.GitHubOrg)
	if err != nil {
		ghs.logger.Sugar().Errorf("error logging into GitHub via GraphQL", zap.Error(err))
		return ghs.mb.Emit(), err
	}

	// Generate the search query based on the type, org/user name, and the search_query
	// value if provided
	sq := genDefaultSearchQuery(loginType, ghs.cfg.GitHubOrg)

	if ghs.cfg.SearchQuery != "" {
		sq = ghs.cfg.SearchQuery
		ghs.logger.Sugar().Debugf("using search query where query is: %q", ghs.cfg.SearchQuery)
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
	var wg sync.WaitGroup

	for _, repo := range repos {
		repo := repo
		name := repo.Name
		trunk := repo.DefaultBranchRef.Name

		wg.Add(1)
		go func() {
			defer wg.Done()

			count, err := ghs.getBranches(ctx, genClient, name, trunk)
			if err != nil {
				ghs.logger.Sugar().Errorf("error getting branch count for repo %s", zap.Error(err), repo.Name)
			}
			ghs.mb.RecordGitRepositoryBranchCountDataPoint(now, int64(count), name)

			// Get the contributor count for each of the repositories
			contribs, err := ghs.getContributorCount(ctx, restClient, name)
			if err != nil {
				ghs.logger.Sugar().Errorf("error getting contributor count for repo %s", zap.Error(err), repo.Name)
			}
			ghs.mb.RecordGitRepositoryContributorCountDataPoint(now, int64(contribs), name)
		}()
	}
	wg.Wait()

	// Set the resource attributes and emit metrics with those resources
	ghs.rb.SetGitVendorName("github")
	ghs.rb.SetOrganizationName(ghs.cfg.GitHubOrg)
	res := ghs.rb.Emit()
	return ghs.mb.Emit(metadata.WithResource(res)), nil
}
