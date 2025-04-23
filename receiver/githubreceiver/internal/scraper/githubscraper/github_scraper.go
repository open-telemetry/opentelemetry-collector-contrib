// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate ../../../../../.tools/genqlient

package githubscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/scraper/githubscraper"

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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/metadata"
)

var errClientNotInitErr = errors.New("http client not initialized")

type githubScraper struct {
	client   *http.Client
	cfg      *Config
	settings component.TelemetrySettings
	logger   *zap.Logger
	mb       *metadata.MetricsBuilder
	rb       *metadata.ResourceBuilder
}

func (ghs *githubScraper) start(ctx context.Context, host component.Host) (err error) {
	ghs.logger.Sugar().Info("starting the GitHub scraper")
	ghs.client, err = ghs.cfg.ToClient(ctx, host, ghs.settings)
	return
}

func newGitHubScraper(
	settings receiver.Settings,
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
		ghs.logger.Sugar().Error("unable to create clients", zap.Error(err))
	}

	// Do some basic validation to ensure the values provided actually exist in github
	// prior to making queries against that org or user value
	loginType, err := ghs.login(ctx, genClient, ghs.cfg.GitHubOrg)
	if err != nil {
		ghs.logger.Sugar().Error("error logging into GitHub via GraphQL", zap.Error(err))
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
		ghs.logger.Sugar().Error("error getting repo data", zap.Error(err))
		return ghs.mb.Emit(), err
	}

	ghs.mb.RecordVcsRepositoryCountDataPoint(now, int64(count))

	// Get the ref (branch) count (future branch data) for each repo and record
	// the given metrics
	var wg sync.WaitGroup
	wg.Add(len(repos))
	var mux sync.Mutex

	for _, repo := range repos {
		name := repo.Name
		url := repo.Url
		trunk := repo.DefaultBranchRef.Name
		now := now

		go func() {
			defer wg.Done()

			branches, count, err := ghs.getBranches(ctx, genClient, name, trunk)
			if err != nil {
				ghs.logger.Sugar().Errorf("error getting branch count: %v", zap.Error(err))
			}

			// Create a mutual exclusion lock to prevent the recordDataPoint
			// SetStartTimestamp call from having a nil pointer panic
			mux.Lock()

			refType := metadata.AttributeVcsRefHeadTypeBranch
			ghs.mb.RecordVcsRefCountDataPoint(now, int64(count), url, name, refType)

			// Iterate through the refs (branches) populating the Branch focused
			// metrics
			for _, branch := range branches {
				// See https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/githubreceiver/internal/scraper/githubscraper/README.md#github-limitations
				// for more information as to why we do not emit metrics for
				// the default branch (trunk) nor any branch with no changes to
				// it.
				if branch.Name == branch.Repository.DefaultBranchRef.Name || branch.Compare.BehindBy == 0 {
					continue
				}

				// See https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/githubreceiver/internal/scraper/githubscraper/README.md#github-limitations
				// for more information as to why `BehindBy` and `AheadBy` are
				// swapped.
				ghs.mb.RecordVcsRefRevisionsDeltaDataPoint(now, int64(branch.Compare.BehindBy), url, branch.Repository.Name, branch.Name, refType, metadata.AttributeVcsRevisionDeltaDirectionAhead)
				ghs.mb.RecordVcsRefRevisionsDeltaDataPoint(now, int64(branch.Compare.AheadBy), url, branch.Repository.Name, branch.Name, refType, metadata.AttributeVcsRevisionDeltaDirectionBehind)

				var additions int
				var deletions int
				var age int64

				additions, deletions, age, err = ghs.evalCommits(ctx, genClient, branch.Repository.Name, branch)
				if err != nil {
					ghs.logger.Sugar().Errorf("error getting commit info: %v", zap.Error(err))
					continue
				}

				ghs.mb.RecordVcsRefTimeDataPoint(now, age, url, branch.Repository.Name, branch.Name, refType)
				ghs.mb.RecordVcsRefLinesDeltaDataPoint(now, int64(additions), url, branch.Repository.Name, branch.Name, refType, metadata.AttributeVcsLineChangeTypeAdded)
				ghs.mb.RecordVcsRefLinesDeltaDataPoint(now, int64(deletions), url, branch.Repository.Name, branch.Name, refType, metadata.AttributeVcsLineChangeTypeRemoved)
			}

			// Get the contributor count for each of the repositories
			contribs, err := ghs.getContributorCount(ctx, restClient, name)
			if err != nil {
				ghs.logger.Sugar().Errorf("error getting contributor count: %v", zap.Error(err))
			}
			ghs.mb.RecordVcsContributorCountDataPoint(now, int64(contribs), url, name)

			// Get change (pull request) data
			prs, err := ghs.getPullRequests(ctx, genClient, name)
			if err != nil {
				ghs.logger.Sugar().Errorf("error getting pull requests: %v", zap.Error(err))
			}

			var merged int
			var open int

			for _, pr := range prs {
				if pr.Merged {
					merged++

					age := getAge(pr.CreatedAt, pr.MergedAt)

					ghs.mb.RecordVcsChangeTimeToMergeDataPoint(now, age, url, name, pr.HeadRefName)
				} else {
					open++

					age := getAge(pr.CreatedAt, now.AsTime())

					ghs.mb.RecordVcsChangeDurationDataPoint(now, age, url, name, pr.HeadRefName, metadata.AttributeVcsChangeStateOpen)

					if pr.Reviews.TotalCount > 0 {
						age := getAge(pr.CreatedAt, pr.Reviews.Nodes[0].CreatedAt)

						ghs.mb.RecordVcsChangeTimeToApprovalDataPoint(now, age, url, name, pr.HeadRefName)
					}
				}
			}

			ghs.mb.RecordVcsChangeCountDataPoint(now, int64(open), url, metadata.AttributeVcsChangeStateOpen, name)
			ghs.mb.RecordVcsChangeCountDataPoint(now, int64(merged), url, metadata.AttributeVcsChangeStateMerged, name)
			mux.Unlock()
		}()
	}

	wg.Wait()

	// Set the resource attributes and emit metrics with those resources
	ghs.rb.SetVcsVendorName("github")
	ghs.rb.SetOrganizationName(ghs.cfg.GitHubOrg)

	res := ghs.rb.Emit()
	return ghs.mb.Emit(metadata.WithResource(res)), nil
}
