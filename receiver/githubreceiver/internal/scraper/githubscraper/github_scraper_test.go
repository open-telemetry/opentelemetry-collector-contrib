// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-github/v83/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/metadata"
)

func TestNewGitHubScraper(t *testing.T) {
	factory := Factory{}
	defaultConfig := factory.CreateDefaultConfig()

	s := newGitHubScraper(receiver.Settings{}, defaultConfig.(*Config))

	assert.NotNil(t, s)
}

func TestScrape(t *testing.T) {
	testCases := []struct {
		desc     string
		server   *http.ServeMux
		testFile string
	}{
		{
			desc: "TestNoRepos",
			server: MockServer(&responses{
				scrape: true,
				checkLoginResponse: loginResponse{
					checkLogin: checkLoginResponse{
						Organization: checkLoginOrganization{
							Login: "open-telemetry",
						},
					},
					responseCode: http.StatusOK,
				},
				repoResponse: repoResponse{
					repos: []getRepoDataBySearchSearchSearchResultItemConnection{
						{
							RepositoryCount: 0,
							Nodes:           []SearchNode{},
						},
					},
					responseCode: http.StatusOK,
				},
			}),
			testFile: "expected_no_repos.yaml",
		},
		{
			desc: "TestHappyPath",
			server: MockServer(&responses{
				scrape: true,
				checkLoginResponse: loginResponse{
					checkLogin: checkLoginResponse{
						Organization: checkLoginOrganization{
							Login: "open-telemetry",
						},
					},
					responseCode: http.StatusOK,
				},
				repoResponse: repoResponse{
					repos: []getRepoDataBySearchSearchSearchResultItemConnection{
						{
							RepositoryCount: 1,
							Nodes: []SearchNode{
								&SearchNodeRepository{
									Name: "repo1",
								},
							},
							PageInfo: getRepoDataBySearchSearchSearchResultItemConnectionPageInfo{
								HasNextPage: false,
							},
						},
					},
					responseCode: http.StatusOK,
				},
				prResponse: prResponse{
					prs: []getPullRequestDataRepositoryPullRequestsPullRequestConnection{
						{
							PageInfo: getPullRequestDataRepositoryPullRequestsPullRequestConnectionPageInfo{
								HasNextPage: false,
							},
							Nodes: []PullRequestNode{
								{
									Merged: false,
								},
							},
						},
					},
					responseCode: http.StatusOK,
				},
				mergedPRResponse: mergedPRResponse{
					prs: []getMergedPullRequestDataRepositoryPullRequestsPullRequestConnection{
						{
							PageInfo: getMergedPullRequestDataRepositoryPullRequestsPullRequestConnectionPageInfo{
								HasPreviousPage: false,
							},
							Nodes: []MergedPullRequestNode{
								{
									Merged: true,
								},
							},
						},
					},
					responseCode: http.StatusOK,
				},
				branchResponse: branchResponse{
					branches: []getBranchDataRepositoryRefsRefConnection{
						{
							TotalCount: 1,
							Nodes: []BranchNode{
								{
									Name: "main",
									Compare: BranchNodeCompareComparison{
										AheadBy:  0,
										BehindBy: 1,
									},
								},
							},
							PageInfo: getBranchDataRepositoryRefsRefConnectionPageInfo{
								HasNextPage: false,
							},
						},
					},
					responseCode: http.StatusOK,
				},
				commitResponse: commitResponse{
					commits: []BranchHistoryTargetCommit{
						{
							History: BranchHistoryTargetCommitHistoryCommitHistoryConnection{
								Nodes: []CommitNode{
									{
										CommittedDate: time.Now().AddDate(0, 0, -1),
										Additions:     10,
										Deletions:     9,
									},
								},
							},
						},
					},
					responseCode: http.StatusOK,
				},
				contribResponse: contribResponse{
					contribs: [][]*github.Contributor{
						{
							{
								ID: github.Ptr(int64(1)),
							},
						},
					},
					responseCode: http.StatusOK,
				},
			}),
			testFile: "expected_happy_path.yaml",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			server := httptest.NewServer(tc.server)
			defer server.Close()

			cfg := &Config{MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()}

			ghs := newGitHubScraper(receivertest.NewNopSettings(metadata.Type), cfg)
			ghs.cfg.GitHubOrg = "open-telemetry"
			ghs.cfg.Endpoint = server.URL

			err := ghs.start(t.Context(), componenttest.NewNopHost())
			require.NoError(t, err)

			actualMetrics, err := ghs.scrape(t.Context())
			require.NoError(t, err)

			expectedFile := filepath.Join("testdata", "scraper", tc.testFile)

			// Due to the generative nature of the code we're using through
			// genqlient. The tests happy path changes, and needs to be rebuilt
			// to satisfy the unit tests. When the metadata.yaml changes, and
			// code is introduced, or removed. We'll need to update the metrics
			// by uncommenting the below and running `make test` to generate
			// it. Then we're safe to comment this out again and see happy
			// tests.
			// golden.WriteMetrics(t, expectedFile, actualMetrics)

			expectedMetrics, err := golden.ReadMetrics(expectedFile)
			require.NoError(t, err)
			require.NoError(t, pmetrictest.CompareMetrics(
				expectedMetrics,
				actualMetrics,
				pmetrictest.IgnoreMetricDataPointsOrder(),
				pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreStartTimestamp(),
				pmetrictest.IgnoreMetricValues(
					"vcs.ref.time",
					"vcs.change.duration",
					"vcs.change.time_to_merge",
				),
			))
		})
	}
}
