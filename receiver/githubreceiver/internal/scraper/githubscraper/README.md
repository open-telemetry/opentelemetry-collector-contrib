# GitHub Limitations

## API Limitations

The GitHub scraper is reliant on limitations found within GitHub's REST and
GraphQL APIs. The following limitations are known:

* The original creation date of a branch is not available via either of the
  APIs. GitSCM (the tool) does provide Ref creation time however this is not
  exposed. As such, we're forced to calculate the age by looking to see if any
  changes have been made to the branch, using that commit as the time from
  which we can grab the date. This means that age will reflect the time between
  now and the first commit on a new branch. It also means that we don't have
  ages for branches that have been created from trunk but have not had any
  changes made to them.
* It's possible that some queries may run against a branch that has been
  deleted. This is unlikely due to the speed of the requests, however,
  possible.
* Both APIs have primary and secondary rate limits applied to them. The default
  rate limit for GraphQL API is 5,000 points per hour when authenticated with a
  GitHub Personal Access Token (PAT). If using the [GitHub App Auth
  extension][ghext] then your rate limit increases to 10,000. The receiver on
  average costs 4 points per repository (which can heavily fluctuate), allowing
  it to scrape up to 1250 repositories per hour under normal conditions. You
  may use the following equation to roughly calculate your ideal collection
  interval.

```math
\text{collection\_interval (seconds)} = \frac{4n}{r/3600}
```

```math
\begin{aligned}
    \text{where:} \\
    n &= \text{number of repositories} \\
    r &= \text{hourly rate limit} \\
\end{aligned}
```

In addition to these primary rate limits, GitHub enforces secondary rate limits
to prevent abuse and maintain API availability. The following secondary limit is
particularly relevant:

- **Concurrent Requests Limit**: The API allows no more than 100 concurrent
requests. This limit is shared across the REST and GraphQL APIs. Since the
scraper creates a goroutine per repository, having more than 100 repositories
returned by the `search_query` will result in exceeding this limit.
It is recommended to use the `search_query` config option to limit the number of
repositories that are scraped. We recommend one instance of the receiver per
team (note: `team` is not a valid quantifier when searching repositories `topic`
is). Reminder that each instance of the receiver should have its own
corresponding token for authentication as this is what rate limits are tied to.

In summary, we recommend the following:

- One instance of the receiver per team
- Each instance of the receiver should have its own token
- Leverage `search_query` config option to limit repositories returned to 100 or
less per instance
- `collection_interval` should be long enough to avoid rate limiting (see above
formula). A sensible default is `300s`.

**Additional Resources:**

- [GitHub GraphQL Primary Rate Limit](https://docs.github.com/en/graphql/overview/rate-limits-and-node-limits-for-the-graphql-api#primary-rate-limit)
- [GitHub GraphQL Secondary Rate Limit](https://docs.github.com/en/graphql/overview/rate-limits-and-node-limits-for-the-graphql-api#secondary-rate-limit)

[ghext]: https://github.com/liatrio/liatrio-otel-collector/tree/main/extension/githubappauthextension

## Branch Data Limitations


Due to the limitations of the GitHub GraphQL and REST APIs, some data retrieved
may not be as expected. Notably there are spots in the code which link to this
section that make decisions based on these limitations.

Queries are constructed to maximize performance without being overly complex.
Note that there are sections in the code where `BehindBy` is being used in
place of `AheadBy` and vice versa. This is a byproduct of the `getBranchData`
query which returns all the refs (branches) from a given repository and the
comparison to the default branch (trunk). Comparing it here reduces the need
to make a query that gets all the names of the refs (branches), and then queries
against each branch. 

Another such byproduct of this method is the skipping of metric creation if the
branch is the default branch (trunk) or if no changes have been made to the
ref (branch). This is done for three main reasons.

1. The default branch will always be a long-lived branch and
   may end up with more commits than can be possibly queried
   at a given time.
2. The default is the trunk of which all changes should go
   into. The intent of these metrics is to provide useful
   signals helping identify cognitive overhead and
   bottlenecks.
3. GitHub does not provide any means to determine when a
   branch was actually created. Git the tool however does
   provide a created time for each ref off the trunk. GitHub
   does not expose this data via their APIs and thus we
   have to calculate age based on commits added to the
   branch.

We also have to calculate the number of pages before getting the commit data.
This is because you have to know the exact number of commits added to the
branch, otherwise you'll get all commits from both trunk and the branch from
all time. From there we can evaluate the commits on each branch. To calculate
the time (age) of a branch, we have to know the commits that have been added to
the branch because GitHub does not provide the actual created date of a branch
through either of its APIs.
