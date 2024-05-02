// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitproviderreceiver/internal/scraper/githubscraper"

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/go-github/v61/github"
	"go.uber.org/zap"
)

const (
	// The default public GitHub GraphQL Endpoint
	defaultGraphURL = "https://api.github.com/graphql"
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
) ([]BranchNode, int, error) {
	var cursor *string
	var count int
	var branches []BranchNode

	for next := true; next; {
		r, err := getBranchData(ctx, client, repoName, ghs.cfg.GitHubOrg, 50, defaultBranch, cursor)
		if err != nil {
			ghs.logger.Sugar().Errorf("error getting branch data", zap.Error(err))
			return nil, 0, err
		}
		count = r.Repository.Refs.TotalCount
		cursor = &r.Repository.Refs.PageInfo.EndCursor
		next = r.Repository.Refs.PageInfo.HasNextPage
		branches = append(branches, r.Repository.Refs.Nodes...)
	}
	return branches, count, nil
}

// Login via the GraphQL checkLogin query in order to ensure that the user
// and it's credentials are valid and return the type of user being authenticated.
func (ghs *githubScraper) login(
	ctx context.Context,
	client graphql.Client,
	owner string,
) (string, error) {
	var loginType string

	// The checkLogin GraphQL query will always return an error. We only return
	// the error if the login response for User and Organization are both nil.
	// This is represented by checking to see if each resp.*.Login resolves to equal the owner.
	resp, err := checkLogin(ctx, client, ghs.cfg.GitHubOrg)

	// These types are used later to generate the default string for the search query
	// and thus must match the convention for user: and org: searches in GitHub
	switch {
	case resp.User.Login == owner:
		loginType = "user"
	case resp.Organization.Login == owner:
		loginType = "org"
	default:
		return "", err
	}

	return loginType, nil
}

// Returns the default search query string based on input of owner type
// and GitHubOrg name with a default of archived:false to ignore archived repos
func genDefaultSearchQuery(ownertype string, ghorg string) string {
	return fmt.Sprintf("%s:%s archived:false", ownertype, ghorg)
}

// Returns the graphql and rest clients for GitHub.
// By default, the graphql client will use the public GitHub API URL as will
// the rest client. If the user has specified an endpoint in the config via the
// inherited ClientConfig, then the both clients will use that endpoint.
// The endpoint defined needs to be the root server.
// See the GitHub documentation for more information.
// https://docs.github.com/en/graphql/guides/forming-calls-with-graphql#the-graphql-endpoint
// https://docs.github.com/en/enterprise-server@3.8/graphql/guides/forming-calls-with-graphql#the-graphql-endpoint
// https://docs.github.com/en/enterprise-server@3.8/rest/guides/getting-started-with-the-rest-api#making-a-request
func (ghs *githubScraper) createClients() (gClient graphql.Client, rClient *github.Client, err error) {
	rClient = github.NewClient(ghs.client)
	gClient = graphql.NewClient(defaultGraphURL, ghs.client)

	if ghs.cfg.ClientConfig.Endpoint != "" {

		// Given endpoint set as `https://myGHEserver.com` we need to join the path
		// with `api/graphql`
		gu, err := url.JoinPath(ghs.cfg.ClientConfig.Endpoint, "api/graphql")
		if err != nil {
			ghs.logger.Sugar().Errorf("error joining graphql endpoint: %v", err)
			return nil, nil, err
		}
		gClient = graphql.NewClient(gu, ghs.client)

		// The rest client needs the endpoint to be the root of the server
		ru := ghs.cfg.ClientConfig.Endpoint
		rClient, err = github.NewClient(ghs.client).WithEnterpriseURLs(ru, ru)
		if err != nil {
			ghs.logger.Sugar().Errorf("error creating enterprise client: %v", err)
			return nil, nil, err
		}
	}

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

// Get the pull request data from the GraphQL API.
func (ghs *githubScraper) getPullRequests(
	ctx context.Context,
	client graphql.Client,
	repoName string,
) ([]PullRequestNode, error) {
	var prCursor *string
	var pullRequests []PullRequestNode

	for hasNextPage := true; hasNextPage; {
		prs, err := getPullRequestData(
			ctx,
			client,
			repoName,
			ghs.cfg.GitHubOrg,
			100,
			prCursor,
			[]PullRequestState{"OPEN", "MERGED"},
		)
		if err != nil {
			return nil, err
		}

		pullRequests = append(pullRequests, prs.Repository.PullRequests.Nodes...)
		prCursor = &prs.Repository.PullRequests.PageInfo.EndCursor
		hasNextPage = prs.Repository.PullRequests.PageInfo.HasNextPage
	}

	return pullRequests, nil
}

func (ghs *githubScraper) getCommitInfo(
	client graphql.Client,
	repoName string,
	branch BranchNode,
) (int, int, int64, error) {
	comCount := 100
	var cc *string
	var adds int
	var dels int
	var age int64

	// We're using BehindBy here because we're comparing against the target
	// branch, which is the default branch. In essence the response is saying
	// the default branch is behind the queried branch by X commits which is
	// the number of commits made to the queried branch but not merged into
	// the default branch. Doing it this way involves less queries because
	// we don't have to know the queried branch name ahead of time.
	comPages := getNumPages(float64(100), float64(branch.Compare.BehindBy))

	for nPage := 1; nPage <= comPages; nPage++ {
		if nPage == comPages {
			comCount = branch.Compare.BehindBy % 100
			// When the last page is full
			if comCount == 0 {
				comCount = 100
			}
		}
		c, err := ghs.getCommitData(client, repoName, comCount, cc, branch.Name)
		if err != nil {
			ghs.logger.Sugar().Errorf("error getting commit data", zap.Error(err))
			return 0, 0, 0, err
		}

		if len(c.Edges) == 0 {
			break
		}
		cc = &c.PageInfo.EndCursor
		if nPage == comPages {
			e := c.GetEdges()
			oldest := e[len(e)-1].Node.GetCommittedDate()
			age = int64(time.Since(oldest).Seconds())
		}
		for b := 0; b < len(c.Edges); b++ {
			adds = add(adds, c.Edges[b].Node.Additions)
			dels = add(dels, c.Edges[b].Node.Deletions)
		}

	}
	return adds, dels, age, nil
}

func (ghs *githubScraper) getCommitData(
	client graphql.Client,
	repoName string,
	comCount int,
	cc *string,
	branchName string,
) (*CommitNodeTargetCommitHistoryCommitHistoryConnection, error) {
	data, err := getCommitData(context.Background(), client, repoName, ghs.cfg.GitHubOrg, 1, comCount, cc, branchName)
	if err != nil {
		return nil, err
	}

	if len(data.Repository.Refs.Nodes) == 0 {
		return nil, errors.New("no commits returned")
	}

	tar := data.Repository.Refs.Nodes[0].GetTarget()

	if ct, ok := tar.(*CommitNodeTargetCommit); ok {
		return &ct.History, nil
	}

	return nil, errors.New("target is not a commit")
}

func getNumPages(p float64, n float64) int {
	numPages := math.Ceil(n / p)

	return int(numPages)
}

func add[T ~int | ~float64](a, b T) T {
	return a + b
}

// Get the age/duration between two times in seconds.
func getAge(start time.Time, end time.Time) int64 {
	return int64(end.Sub(start).Seconds())
}
