// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/go-github/v57/github"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

type responses struct {
	checkLogin   checkLoginResponse
	repos        []getRepoDataBySearchSearchSearchResultItemConnection
	branches     []getBranchDataRepositoryRefsRefConnection
	responseCode int
	page         int
	contribs     []*github.Contributor
}

func graphqlMockServer(responses *responses) *http.ServeMux {
	var mux http.ServeMux
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var reqBody graphql.Request
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			return
		}
		switch {
		// These OpNames need to be name of the GraphQL query as defined in genqlient.graphql
		case reqBody.OpName == "checkLogin":
			w.WriteHeader(responses.responseCode)
			if responses.responseCode == http.StatusOK {
				login := responses.checkLogin
				graphqlResponse := graphql.Response{Data: &login}
				if err := json.NewEncoder(w).Encode(graphqlResponse); err != nil {
					return
				}
			}
		case reqBody.OpName == "getRepoDataBySearch":
			w.WriteHeader(responses.responseCode)
			if responses.responseCode == http.StatusOK {
				repos := getRepoDataBySearchResponse{
					Search: responses.repos[responses.page],
				}
				graphqlResponse := graphql.Response{Data: &repos}
				if err := json.NewEncoder(w).Encode(graphqlResponse); err != nil {
					return
				}
				responses.page++
			}
		case reqBody.OpName == "getBranchData":
			w.WriteHeader(responses.responseCode)
			if responses.responseCode == http.StatusOK {
				repos := getBranchDataResponse{
					Repository: getBranchDataRepository{
						Refs: responses.branches[responses.page],
					},
				}
				graphqlResponse := graphql.Response{Data: &repos}
				if err := json.NewEncoder(w).Encode(graphqlResponse); err != nil {
					return
				}
				responses.page++
			}
		}
	})
	return &mux
}

func restMockServer(resp responses) *http.ServeMux {
	var mux http.ServeMux
	mux.HandleFunc("/api-v3/repos/o/r/contributors", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(resp.responseCode)
		if resp.responseCode == http.StatusOK {
			contribs, _ := json.Marshal(resp.contribs)
			// Attempt to write data to the response writer.
			_, err := w.Write(contribs)
			if err != nil {
				fmt.Printf("error writing response: %v", err)
			}
		}
	})
	return &mux
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
			login: "liatrio",
			server: graphqlMockServer(&responses{
				checkLogin: checkLoginResponse{
					Organization: checkLoginOrganization{
						Login: "liatrio",
					},
				},
				responseCode: http.StatusOK,
			}),
			expectedOwnerType: "org",
		},
		{
			desc:  "TestUserOwnerExists",
			login: "liatrio",
			server: graphqlMockServer(&responses{
				checkLogin: checkLoginResponse{
					User: checkLoginUser{
						Login: "liatrio",
					},
				},
				responseCode: http.StatusOK,
			}),
			expectedOwnerType: "user",
		},
		{
			desc:  "TestLoginError",
			login: "liatrio",
			server: graphqlMockServer(&responses{
				checkLogin: checkLoginResponse{
					User: checkLoginUser{
						Login: "liatrio",
					},
				},
				responseCode: http.StatusNotFound,
			}),
			expectedOwnerType: "",
			expectedError:     true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {

			factory := Factory{}
			defaultConfig := factory.CreateDefaultConfig()
			settings := receivertest.NewNopCreateSettings()
			ghs := newGitHubScraper(context.Background(), settings, defaultConfig.(*Config))
			server := httptest.NewServer(tc.server)
			defer server.Close()

			client := graphql.NewClient(server.URL, ghs.client)
			loginType, err := ghs.login(context.Background(), client, tc.login)

			assert.Equal(t, tc.expectedOwnerType, loginType)
			if !tc.expectedError {
				assert.NoError(t, err)
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
			server: graphqlMockServer(&responses{
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
			}),
			expectedErr: nil,
			expected:    1,
		},
		{
			desc: "TestMultiPageResponse",
			server: graphqlMockServer(&responses{
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
			}),
			expectedErr: nil,
			expected:    4,
		},
		{
			desc: "Test404Response",
			server: graphqlMockServer(&responses{
				responseCode: http.StatusNotFound,
			}),
			expectedErr: errors.New("returned error 404 Not Found: "),
			expected:    0,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			factory := Factory{}
			defaultConfig := factory.CreateDefaultConfig()
			settings := receivertest.NewNopCreateSettings()
			ghs := newGitHubScraper(context.Background(), settings, defaultConfig.(*Config))
			server := httptest.NewServer(tc.server)
			defer server.Close()
			client := graphql.NewClient(server.URL, ghs.client)

			_, count, err := ghs.getRepos(context.Background(), client, "fake query")

			assert.Equal(t, tc.expected, count)
			if tc.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr.Error())
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
			server: graphqlMockServer(&responses{
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
			}),
			expectedErr: nil,
			expected:    1,
		},
		{
			desc: "TestMultiPageResponse",
			server: graphqlMockServer(&responses{
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
			}),
			expectedErr: nil,
			expected:    4,
		},
		{
			desc: "Test404Response",
			server: graphqlMockServer(&responses{
				responseCode: http.StatusNotFound,
			}),
			expectedErr: errors.New("returned error 404 Not Found: "),
			expected:    0,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			factory := Factory{}
			defaultConfig := factory.CreateDefaultConfig()
			settings := receivertest.NewNopCreateSettings()
			ghs := newGitHubScraper(context.Background(), settings, defaultConfig.(*Config))
			server := httptest.NewServer(tc.server)
			defer server.Close()
			client := graphql.NewClient(server.URL, ghs.client)

			count, err := ghs.getBranches(context.Background(), client, "deathstarrepo", "main")

			assert.Equal(t, tc.expected, count)
			if tc.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr.Error())
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
			server: restMockServer(responses{
				contribs: []*github.Contributor{

					{
						ID: github.Int64(1),
					},
					{
						ID: github.Int64(2),
					},
				},
				responseCode: http.StatusOK,
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
			settings := receivertest.NewNopCreateSettings()
			ghs := newGitHubScraper(context.Background(), settings, defaultConfig.(*Config))
			ghs.cfg.GitHubOrg = tc.org

			server := httptest.NewServer(tc.server)

			client := github.NewClient(nil)
			url, _ := url.Parse(server.URL + "/api-v3" + "/")
			client.BaseURL = url
			client.UploadURL = url

			contribs, err := ghs.getContributorCount(context.Background(), client, tc.repo)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedCount, contribs)
		})
	}
}
