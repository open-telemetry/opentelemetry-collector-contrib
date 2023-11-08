// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitproviderreceiver/internal/scraper/githubscraper"

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/go-github/v53/github"
	"go.uber.org/zap"
)

func (ghs *githubScraper) getRepos(
	ctx context.Context,
	client graphql.Client,
	searchQuery string,
) ([]SearchNodeRepository, int, error) {
	// here we use a pointer to a string so that graphql will receive null if the
	// value is not set since the after: $repoCursor is optional to graphql
	var cursor *string
	var repos []SearchNodeRepository
	var count int

	for next := true; next; {
		r, err := getRepoDataBySearch(ctx, client, searchQuery, cursor)
		if err != nil {
			ghs.logger.Sugar().Errorf("error getting repo data", zap.Error(err))
			return nil, 0, err
		}

		for _, repo := range r.Search.Nodes {
			if r, ok := repo.(*SearchNodeRepository); ok {
				repos = append(repos, *r)
			}
		}

		count = r.Search.RepositoryCount
		cursor = &r.Search.PageInfo.EndCursor
		next = r.Search.PageInfo.HasNextPage
	}

	return repos, count, nil
}

func (ghs *githubScraper) getBranches(
	ctx context.Context,
	client graphql.Client,
	repoName string,
	defaultBranch string,
) (int, error) {
	var cursor *string
	var count int

	for next := true; next; {
		r, err := getBranchData(ctx, client, repoName, ghs.cfg.GitHubOrg, 50, defaultBranch, cursor)
		if err != nil {
			ghs.logger.Sugar().Errorf("error getting branch data", zap.Error(err))
			return 0, err
		}
		count = r.Repository.Refs.TotalCount
		cursor = &r.Repository.Refs.PageInfo.EndCursor
		next = r.Repository.Refs.PageInfo.HasNextPage
	}
	return count, nil
}

// Ensure that the type of owner is user or organization
func checkOwnerTypeValid(ownertype string) (bool, error) {
	if ownertype == "org" || ownertype == "user" {
		return true, nil
	}
	return false, errors.New("ownertype must be either org or user")
}

// Check to ensure that the login user (org name or user id) exists or
// can be logged into.
func (ghs *githubScraper) checkOwnerExists(ctx context.Context, client graphql.Client, owner string) (exists bool, ownerType string, err error) {

	loginResp, err := checkLogin(ctx, client, ghs.cfg.GitHubOrg)

	exists = false
	ownerType = ""

	// These types are used later to generate the default string for the search query
	// and thus must match the convention for user: and org: searches in GitHub
	if loginResp.User.Login == owner {
		exists = true
		ownerType = "user"
	} else if loginResp.Organization.Login == owner {
		exists = true
		ownerType = "org"
	}

	if exists {
		err = nil
	}

	return
}

// Returns the default search query string based on input of owner type
// and GitHubOrg name with a default of archived:false to ignore archived repos
func genDefaultSearchQuery(ownertype string, ghorg string) string {
	return fmt.Sprintf("%s:%s archived:false", ownertype, ghorg)
}

// Returns the graphql and rest clients for GitHub.
// By default, the graphql client will use the public GitHub API URL as will
// the rest client. If the user has specified an endpoint in the config via the
// inherited HTTPClientSettings, then the both clients will use that endpoint.
// The endpoint defined needs to be the root server.
// See the GitHub documentation for more information.
// https://docs.github.com/en/graphql/guides/forming-calls-with-graphql#the-graphql-endpoint
// https://docs.github.com/en/enterprise-server@3.8/graphql/guides/forming-calls-with-graphql#the-graphql-endpoint
// https://docs.github.com/en/enterprise-server@3.8/rest/guides/getting-started-with-the-rest-api#making-a-request
func (ghs *githubScraper) createClients() (gClient graphql.Client, rClient *github.Client, err error) {

	gURL := "https://api.github.com/graphql"
	rClient = github.NewClient(ghs.client)

	if ghs.cfg.HTTPClientSettings.Endpoint != "" {

		// Given endpoint set as `https://myGHEserver.com` we need to join the path
		// with `api/graphql`
		gURL, err = url.JoinPath(ghs.cfg.HTTPClientSettings.Endpoint, "api/graphql")
		if err != nil {
			ghs.logger.Sugar().Errorf("error joining graphql endpoint: %v", err)
			return nil, nil, err
		}

		// The rest client needs the endpoint to be the root of the server
		rURL := ghs.cfg.HTTPClientSettings.Endpoint
		rClient, err = github.NewEnterpriseClient(rURL, rURL, ghs.client)
		if err != nil {
			ghs.logger.Sugar().Errorf("error creating enterprise client: %v", err)
			return nil, nil, err
		}
	}

	gClient = graphql.NewClient(gURL, ghs.client)

	return gClient, rClient, nil
}

// Get the contributor count for a repository via the REST API
func (ghs *githubScraper) getContributorCount(
	ctx context.Context,
	client *github.Client,
	repoName string,
) (int, error) {
	var all []*github.Contributor

	// Options for Pagination support, default from GitHub was 30
	// https://docs.github.com/en/rest/repos/repos#list-repository-contributors
	opt := &github.ListContributorsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		contribs, resp, err := client.Repositories.ListContributors(ctx, ghs.cfg.GitHubOrg, repoName, opt)
		if err != nil {
			ghs.logger.Sugar().Errorf("error getting contributor count", zap.Error(err))
			return 0, err
		}

		all = append(all, contribs...)
		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return len(all), nil
}
