// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/go-github/v83/github"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/metadata"
)

type responses struct {
	repoResponse       repoResponse
	prResponse         prResponse
	mergedPRResponse   mergedPRResponse
	branchResponse     branchResponse
	checkLoginResponse loginResponse
	contribResponse    contribResponse
	commitResponse     commitResponse
	scrape             bool
}

type repoResponse struct {
	repos        []getRepoDataBySearchSearchSearchResultItemConnection
	responseCode int
	page         int
}

type prResponse struct {
	prs          []getPullRequestDataRepositoryPullRequestsPullRequestConnection
	responseCode int
	page         int
}

type mergedPRResponse struct {
	prs          []getMergedPullRequestDataRepositoryPullRequestsPullRequestConnection
	responseCode int
	page         int
}

type branchResponse struct {
	branches     []getBranchDataRepositoryRefsRefConnection
	responseCode int
	page         int
}

type commitResponse struct {
	commits      []BranchHistoryTargetCommit
	responseCode int
	page         int
}

type loginResponse struct {
	checkLogin   checkLoginResponse
	responseCode int
}

type contribResponse struct {
	contribs     [][]*github.Contributor
	responseCode int
	page         int
}

func MockServer(responses *responses) *http.ServeMux {
	var mux http.ServeMux
	restEndpoint := "/api-v3/repos/o/r/contributors"
	graphEndpoint := "/"
	if responses.scrape {
		graphEndpoint = "/api/graphql"
		restEndpoint = "/api/v3/repos/open-telemetry/repo1/contributors"
	}
	mux.HandleFunc(graphEndpoint, func(w http.ResponseWriter, r *http.Request) {
		var reqBody graphql.Request
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			return
		}
		switch reqBody.OpName {
		// These OpNames need to be name of the GraphQL query as defined in genqlient.graphql
		case "checkLogin":
			loginResp := &responses.checkLoginResponse
			w.WriteHeader(loginResp.responseCode)
			if loginResp.responseCode == http.StatusOK {
				login := loginResp.checkLogin
				graphqlResponse := graphql.Response{Data: &login}
				if err := json.NewEncoder(w).Encode(graphqlResponse); err != nil {
					return
				}
			}
		case "getRepoDataBySearch":
			repoResp := &responses.repoResponse
			w.WriteHeader(repoResp.responseCode)
			if repoResp.responseCode == http.StatusOK {
				repos := getRepoDataBySearchResponse{
					Search: repoResp.repos[repoResp.page],
				}
				graphqlResponse := graphql.Response{Data: &repos}
				if err := json.NewEncoder(w).Encode(graphqlResponse); err != nil {
					return
				}
				repoResp.page++
			}
		case "getBranchData":
			branchResp := &responses.branchResponse
			w.WriteHeader(branchResp.responseCode)
			if branchResp.responseCode == http.StatusOK {
				branches := getBranchDataResponse{
					Repository: getBranchDataRepository{
						Refs: branchResp.branches[branchResp.page],
					},
				}
				graphqlResponse := graphql.Response{Data: &branches}
				if err := json.NewEncoder(w).Encode(graphqlResponse); err != nil {
					return
				}
				branchResp.page++
			}
		case "getPullRequestData":
			prResp := &responses.prResponse
			w.WriteHeader(prResp.responseCode)
			if prResp.responseCode == http.StatusOK {
				repos := getPullRequestDataResponse{
					Repository: getPullRequestDataRepository{
						PullRequests: prResp.prs[prResp.page],
					},
				}
				graphqlResponse := graphql.Response{Data: &repos}
				if err := json.NewEncoder(w).Encode(graphqlResponse); err != nil {
					return
				}
				prResp.page++
			}
		case "getMergedPullRequestData":
			mergedPRResp := &responses.mergedPRResponse
			// Default to 200 if not set
			responseCode := mergedPRResp.responseCode
			if responseCode == 0 {
				responseCode = http.StatusOK
			}
			w.WriteHeader(responseCode)
			if responseCode == http.StatusOK {
				// Handle case where no merged PR data is provided
				var pullRequests getMergedPullRequestDataRepositoryPullRequestsPullRequestConnection
				if len(mergedPRResp.prs) > 0 && mergedPRResp.page < len(mergedPRResp.prs) {
					pullRequests = mergedPRResp.prs[mergedPRResp.page]
					mergedPRResp.page++
				} else {
					// Return empty result
					pullRequests = getMergedPullRequestDataRepositoryPullRequestsPullRequestConnection{
						Nodes: []MergedPullRequestNode{},
						PageInfo: getMergedPullRequestDataRepositoryPullRequestsPullRequestConnectionPageInfo{
							HasPreviousPage: false,
						},
					}
				}
				resp := getMergedPullRequestDataResponse{
					Repository: getMergedPullRequestDataRepository{
						PullRequests: pullRequests,
					},
				}
				graphqlResponse := graphql.Response{Data: &resp}
				if err := json.NewEncoder(w).Encode(graphqlResponse); err != nil {
					return
				}
			}
		case "getCommitData":
			commitResp := &responses.commitResponse
			w.WriteHeader(commitResp.responseCode)
			if commitResp.responseCode == http.StatusOK {
				branchHistory := []BranchHistory{
					{Target: &commitResp.commits[commitResp.page]},
				}
				commits := getCommitDataResponse{
					Repository: getCommitDataRepository{
						Refs: getCommitDataRepositoryRefsRefConnection{
							Nodes: branchHistory,
						},
					},
				}
				graphqlResponse := graphql.Response{Data: &commits}
				if err := json.NewEncoder(w).Encode(graphqlResponse); err != nil {
					return
				}
				commitResp.page++
			}
		}
	})
	mux.HandleFunc(restEndpoint, func(w http.ResponseWriter, _ *http.Request) {
		contribResp := &responses.contribResponse
		if contribResp.responseCode == http.StatusOK {
			contribs, err := json.Marshal(contribResp.contribs[contribResp.page])
			if err != nil {
				fmt.Printf("error marshaling response: %v", err)
			}
			link := fmt.Sprintf(
				"<https://api.github.com/repositories/placeholder/contributors?per_page=100&page=%d>; rel=\"next\"",
				len(contribResp.contribs)-contribResp.page-1,
			)
			w.Header().Set("Link", link)
			// Attempt to write data to the response writer.
			_, err = w.Write(contribs)
			if err != nil {
				fmt.Printf("error writing response: %v", err)
			}
			contribResp.page++
		}
	})
	return &mux
}

func TestGetNumPages100(t *testing.T) {
	p := float64(100)
	n := float64(375)

	expected := 4

	num := getNumPages(p, n)

	assert.Equal(t, expected, num)
}

func TestGetNumPages10(t *testing.T) {
	p := float64(10)
	n := float64(375)

	expected := 38

	num := getNumPages(p, n)

	assert.Equal(t, expected, num)
}

func TestGetNumPages1(t *testing.T) {
	p := float64(10)
	n := float64(1)

	expected := 1

	num := getNumPages(p, n)

	assert.Equal(t, expected, num)
}

func TestGenDefaultSearchQueryOrg(t *testing.T) {
	st := "org"
	org := "empire"

	expected := "org:empire archived:false"

	actual := genDefaultSearchQuery(st, org)

	assert.Equal(t, expected, actual)
}

func TestGenDefaultSearchQueryUser(t *testing.T) {
	st := "user"
	org := "vader"

	expected := "user:vader archived:false"

	actual := genDefaultSearchQuery(st, org)

	assert.Equal(t, expected, actual)
}

func TestGetAge(t *testing.T) {
	testCases := []struct {
		desc     string
		hrsAdd   time.Duration
		minsAdd  time.Duration
		expected float64
	}{
		{
			desc:     "TestHalfHourDiff",
			hrsAdd:   time.Duration(0) * time.Hour,
			minsAdd:  time.Duration(30) * time.Minute,
			expected: 60 * 30,
		},
		{
			desc:     "TestHourDiff",
			hrsAdd:   time.Duration(1) * time.Hour,
			minsAdd:  time.Duration(0) * time.Minute,
			expected: 60 * 60,
		},
		{
			desc:     "TestDayDiff",
			hrsAdd:   time.Duration(24) * time.Hour,
			minsAdd:  time.Duration(0) * time.Minute,
			expected: 60 * 60 * 24,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			start := time.Now()
			end := start.Add(tc.hrsAdd).Add(tc.minsAdd)

			actual := getAge(start, end)

			assert.Equal(t, int64(tc.expected), actual)
		})
	}
}

func TestCheckOwnerExists(t *testing.T) {
	testCases := []struct {
		desc              string
		login             string
		expectedError     bool
		expectedOwnerType string
		server            *http.ServeMux
	}{
		{
			desc:  "TestOrgOwnerExists",
			login: "open-telemetry",
			server: MockServer(&responses{
				checkLoginResponse: loginResponse{
					checkLogin: checkLoginResponse{
						Organization: checkLoginOrganization{
							Login: "open-telemetry",
						},
					},
					responseCode: http.StatusOK,
				},
			}),
			expectedOwnerType: "org",
		},
		{
			desc:  "TestUserOwnerExists",
			login: "open-telemetry",
			server: MockServer(&responses{
				checkLoginResponse: loginResponse{
					checkLogin: checkLoginResponse{
						User: checkLoginUser{
							Login: "open-telemetry",
						},
					},
					responseCode: http.StatusOK,
				},
			}),
			expectedOwnerType: "user",
		},
		{
			desc:  "TestLoginError",
			login: "open-telemetry",
			server: MockServer(&responses{
				checkLoginResponse: loginResponse{
					checkLogin: checkLoginResponse{
						Organization: checkLoginOrganization{
							Login: "open-telemetry",
						},
					},
					responseCode: http.StatusNotFound,
				},
			}),
			expectedOwnerType: "",
			expectedError:     true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			factory := Factory{}
			defaultConfig := factory.CreateDefaultConfig()
			settings := receivertest.NewNopSettings(metadata.Type)
			ghs := newGitHubScraper(settings, defaultConfig.(*Config))
			server := httptest.NewServer(tc.server)
			defer server.Close()

			client := graphql.NewClient(server.URL, ghs.client)
			loginType, err := ghs.login(t.Context(), client, tc.login)

			assert.Equal(t, tc.expectedOwnerType, loginType)
			if !tc.expectedError {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetPullRequests(t *testing.T) {
	testCases := []struct {
		desc            string
		server          *http.ServeMux
		expectedErr     error
		expectedPrCount int
	}{
		{
			desc: "TestSinglePageResponse",
			server: MockServer(&responses{
				scrape: false,
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
								{
									Merged: false,
								},
								{
									Merged: false,
								},
							},
						},
					},
					responseCode: http.StatusOK,
				},
			}),
			expectedErr:     nil,
			expectedPrCount: 3, // 3 PRs per page, 1 pages
		},
		{
			desc: "TestMultiPageResponse",
			server: MockServer(&responses{
				scrape: false,
				prResponse: prResponse{
					prs: []getPullRequestDataRepositoryPullRequestsPullRequestConnection{
						{
							PageInfo: getPullRequestDataRepositoryPullRequestsPullRequestConnectionPageInfo{
								HasNextPage: true,
							},
							Nodes: []PullRequestNode{
								{
									Merged: false,
								},
								{
									Merged: false,
								},
								{
									Merged: false,
								},
							},
						},
						{
							PageInfo: getPullRequestDataRepositoryPullRequestsPullRequestConnectionPageInfo{
								HasNextPage: false,
							},
							Nodes: []PullRequestNode{
								{
									Merged: false,
								},
								{
									Merged: false,
								},
								{
									Merged: false,
								},
							},
						},
					},
					responseCode: http.StatusOK,
				},
			}),
			expectedErr:     nil,
			expectedPrCount: 6, // 3 PRs per page, 2 pages
		},
		{
			desc: "Test404Response",
			server: MockServer(&responses{
				scrape: false,
				prResponse: prResponse{
					responseCode: http.StatusNotFound,
				},
			}),
			expectedErr:     errors.New("returned error 404"),
			expectedPrCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			factory := Factory{}
			defaultConfig := factory.CreateDefaultConfig()
			settings := receivertest.NewNopSettings(metadata.Type)
			ghs := newGitHubScraper(settings, defaultConfig.(*Config))
			server := httptest.NewServer(tc.server)
			defer server.Close()
			client := graphql.NewClient(server.URL, ghs.client)

			openPRs, _, err := ghs.getPullRequests(t.Context(), client, "repo name")

			assert.Len(t, openPRs, tc.expectedPrCount)
			if tc.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.expectedErr.Error())
			}
		})
	}
}

func TestGetRepos(t *testing.T) {
	testCases := []struct {
		desc        string
		server      *http.ServeMux
		expectedErr error
		expected    int
	}{
		{
			desc: "TestSinglePageResponse",
			server: MockServer(&responses{
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
			}),
			expectedErr: nil,
			expected:    1,
		},
		{
			desc: "TestMultiPageResponse",
			server: MockServer(&responses{
				repoResponse: repoResponse{
					repos: []getRepoDataBySearchSearchSearchResultItemConnection{
						{
							RepositoryCount: 4,
							Nodes: []SearchNode{
								&SearchNodeRepository{
									Name: "repo1",
								},
								&SearchNodeRepository{
									Name: "repo2",
								},
							},
							PageInfo: getRepoDataBySearchSearchSearchResultItemConnectionPageInfo{
								HasNextPage: true,
							},
						},
						{
							RepositoryCount: 4,
							Nodes: []SearchNode{
								&SearchNodeRepository{
									Name: "repo3",
								},
								&SearchNodeRepository{
									Name: "repo4",
								},
							},
							PageInfo: getRepoDataBySearchSearchSearchResultItemConnectionPageInfo{
								HasNextPage: false,
							},
						},
					},
					responseCode: http.StatusOK,
				},
			}),
			expectedErr: nil,
			expected:    4,
		},
		{
			desc: "Test404Response",
			server: MockServer(&responses{
				repoResponse: repoResponse{
					responseCode: http.StatusNotFound,
				},
			}),
			expectedErr: errors.New("returned error 404"),
			expected:    0,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			factory := Factory{}
			defaultConfig := factory.CreateDefaultConfig()
			settings := receivertest.NewNopSettings(metadata.Type)
			ghs := newGitHubScraper(settings, defaultConfig.(*Config))
			server := httptest.NewServer(tc.server)
			defer server.Close()
			client := graphql.NewClient(server.URL, ghs.client)

			_, count, err := ghs.getRepos(t.Context(), client, "fake query")

			assert.Equal(t, tc.expected, count)
			if tc.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.expectedErr.Error())
			}
		})
	}
}

func TestGetBranches(t *testing.T) {
	testCases := []struct {
		desc        string
		server      *http.ServeMux
		expectedErr error
		expected    int
	}{
		{
			desc: "TestSinglePageResponse",
			server: MockServer(&responses{
				branchResponse: branchResponse{
					branches: []getBranchDataRepositoryRefsRefConnection{
						{
							TotalCount: 1,
							Nodes: []BranchNode{
								{
									Name: "main",
								},
							},
							PageInfo: getBranchDataRepositoryRefsRefConnectionPageInfo{
								HasNextPage: false,
							},
						},
					},
					responseCode: http.StatusOK,
				},
			}),
			expectedErr: nil,
			expected:    1,
		},
		{
			desc: "TestMultiPageResponse",
			server: MockServer(&responses{
				branchResponse: branchResponse{
					branches: []getBranchDataRepositoryRefsRefConnection{
						{
							TotalCount: 4,
							Nodes: []BranchNode{
								{
									Name: "main",
								},
								{
									Name: "vader",
								},
							},
							PageInfo: getBranchDataRepositoryRefsRefConnectionPageInfo{
								HasNextPage: true,
							},
						},
						{
							TotalCount: 4,
							Nodes: []BranchNode{
								{
									Name: "skywalker",
								},
								{
									Name: "rebelalliance",
								},
							},
							PageInfo: getBranchDataRepositoryRefsRefConnectionPageInfo{
								HasNextPage: false,
							},
						},
					},
					responseCode: http.StatusOK,
				},
			}),
			expectedErr: nil,
			expected:    4,
		},
		{
			desc: "Test404Response",
			server: MockServer(&responses{
				branchResponse: branchResponse{
					responseCode: http.StatusNotFound,
				},
			}),
			expectedErr: errors.New("returned error 404"),
			expected:    0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			factory := Factory{}
			defaultConfig := factory.CreateDefaultConfig()
			settings := receivertest.NewNopSettings(metadata.Type)
			ghs := newGitHubScraper(settings, defaultConfig.(*Config))
			server := httptest.NewServer(tc.server)
			defer server.Close()
			client := graphql.NewClient(server.URL, ghs.client)

			_, count, err := ghs.getBranches(t.Context(), client, "deathstarrepo", "main")

			assert.Equal(t, tc.expected, count)
			if tc.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.expectedErr.Error())
			}
		})
	}
}

func TestGetContributors(t *testing.T) {
	testCases := []struct {
		desc          string
		server        *http.ServeMux
		repo          string
		org           string
		expectedErr   error
		expectedCount int
	}{
		{
			desc: "TestListContributorsResponse",
			server: MockServer(&responses{
				contribResponse: contribResponse{
					contribs: [][]*github.Contributor{{
						{
							ID: github.Ptr(int64(1)),
						},
						{
							ID: github.Ptr(int64(2)),
						},
					}},
					responseCode: http.StatusOK,
				},
			}),
			repo:          "r",
			org:           "o",
			expectedErr:   nil,
			expectedCount: 2,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			factory := Factory{}
			defaultConfig := factory.CreateDefaultConfig()
			settings := receivertest.NewNopSettings(metadata.Type)
			ghs := newGitHubScraper(settings, defaultConfig.(*Config))
			ghs.cfg.GitHubOrg = tc.org

			server := httptest.NewServer(tc.server)
			defer func() { server.Close() }()

			client := github.NewClient(nil)
			url, _ := url.Parse(server.URL + "/api-v3" + "/")
			client.BaseURL = url
			client.UploadURL = url

			contribs, err := ghs.getContributorCount(t.Context(), client, tc.repo)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedCount, contribs)
		})
	}
}

func TestEvalCommits(t *testing.T) {
	testCases := []struct {
		desc              string
		server            *http.ServeMux
		expectedErr       error
		branch            BranchNode
		expectedAge       int64
		expectedAdditions int
		expectedDeletions int
	}{
		{
			desc: "TestNoBranchChanges",
			server: MockServer(&responses{
				scrape: false,
				commitResponse: commitResponse{
					commits: []BranchHistoryTargetCommit{
						{
							History: BranchHistoryTargetCommitHistoryCommitHistoryConnection{
								Nodes: []CommitNode{},
							},
						},
					},
					responseCode: http.StatusOK,
				},
			}),
			branch: BranchNode{
				Name: "branch1",
				Compare: BranchNodeCompareComparison{
					AheadBy:  0,
					BehindBy: 0,
				},
			},
			expectedAge:       0,
			expectedAdditions: 0,
			expectedDeletions: 0,
			expectedErr:       nil,
		},
		{
			desc: "TestNoCommitsResponse",
			server: MockServer(&responses{
				scrape: false,
				commitResponse: commitResponse{
					commits: []BranchHistoryTargetCommit{
						{
							History: BranchHistoryTargetCommitHistoryCommitHistoryConnection{
								Nodes: []CommitNode{},
							},
						},
					},
					responseCode: http.StatusOK,
				},
			}),
			branch: BranchNode{
				Name: "branch1",
				Compare: BranchNodeCompareComparison{
					AheadBy:  0,
					BehindBy: 1,
				},
			},
			expectedAge:       0,
			expectedAdditions: 0,
			expectedDeletions: 0,
			expectedErr:       nil,
		},
		{
			desc: "TestSinglePageResponse",
			server: MockServer(&responses{
				scrape: false,
				commitResponse: commitResponse{
					commits: []BranchHistoryTargetCommit{
						{
							History: BranchHistoryTargetCommitHistoryCommitHistoryConnection{
								Nodes: []CommitNode{
									{
										CommittedDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
										Additions:     10,
										Deletions:     9,
									},
								},
							},
						},
					},
					responseCode: http.StatusOK,
				},
			}),
			branch: BranchNode{
				Name: "branch1",
				Compare: BranchNodeCompareComparison{
					AheadBy:  0,
					BehindBy: 1,
				},
			},
			expectedAge:       int64(time.Since(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)).Seconds()),
			expectedAdditions: 10,
			expectedDeletions: 9,
			expectedErr:       nil,
		},
		{
			desc: "TestMultiplePageResponse",
			server: MockServer(&responses{
				scrape: false,
				commitResponse: commitResponse{
					commits: []BranchHistoryTargetCommit{
						{
							History: BranchHistoryTargetCommitHistoryCommitHistoryConnection{
								Nodes: []CommitNode{
									{
										CommittedDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
										Additions:     10,
										Deletions:     9,
									},
								},
							},
						},
						{
							History: BranchHistoryTargetCommitHistoryCommitHistoryConnection{
								Nodes: []CommitNode{
									{
										CommittedDate: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
										Additions:     1,
										Deletions:     1,
									},
								},
							},
						},
					},
					responseCode: http.StatusOK,
				},
			}),
			branch: BranchNode{
				Name: "branch1",
				Compare: BranchNodeCompareComparison{
					AheadBy:  0,
					BehindBy: 101, // 100 per page, so this is 2 pages
				},
			},
			expectedAge:       int64(time.Since(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)).Seconds()),
			expectedAdditions: 11,
			expectedDeletions: 10,
			expectedErr:       nil,
		},
		{
			desc: "Test404ErrorResponse",
			server: MockServer(&responses{
				scrape: false,
				commitResponse: commitResponse{
					responseCode: http.StatusNotFound,
				},
			}),
			branch: BranchNode{
				Name: "branch1",
				Compare: BranchNodeCompareComparison{
					AheadBy:  0,
					BehindBy: 1,
				},
			},
			expectedAge:       0,
			expectedAdditions: 0,
			expectedDeletions: 0,
			expectedErr:       errors.New("returned error 404"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			factory := Factory{}
			defaultConfig := factory.CreateDefaultConfig()
			settings := receivertest.NewNopSettings(metadata.Type)
			ghs := newGitHubScraper(settings, defaultConfig.(*Config))
			server := httptest.NewServer(tc.server)
			defer server.Close()
			client := graphql.NewClient(server.URL, ghs.client)
			adds, dels, age, err := ghs.evalCommits(t.Context(), client, "repo1", tc.branch)

			assert.Equal(t, tc.expectedDeletions, dels)
			assert.Equal(t, tc.expectedAdditions, adds)
			if tc.expectedAge != 0 {
				assert.WithinDuration(t, time.UnixMilli(tc.expectedAge), time.UnixMilli(age), 10*time.Second)
			} else {
				assert.Equal(t, tc.expectedAge, age)
			}

			if tc.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.expectedErr.Error())
			}
		})
	}
}
