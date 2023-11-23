package githubactionsreceiver

import "time"

type WorkflowJobEvent struct {
	Action       string       `json:"action"`
	WorkflowJob  WorkflowJob  `json:"workflow_job"`
	Repository   Repository   `json:"repository"`
	Organization Organization `json:"organization"`
	Sender       Sender       `json:"sender"`
	Steps        []Step       `json:"steps"`
}

type WorkflowRunEvent struct {
	Action       string       `json:"action"`
	WorkflowRun  WorkflowRun  `json:"workflow_run"`
	Workflow     Workflow     `json:"workflow"`
	Repository   Repository   `json:"repository"`
	Organization Organization `json:"organization"`
	Enterprise   Enterprise   `json:"enterprise,omitempty"`
	Sender       Sender       `json:"sender"`
	Installation Installation `json:"installation,omitempty"`
}

type Actor struct {
	Login             string `json:"login,omitempty"`
	ID                int    `json:"id,omitempty"`
	NodeID            string `json:"node_id,omitempty"`
	AvatarURL         string `json:"avatar_url,omitempty"`
	GravatarID        string `json:"gravatar_id,omitempty"`
	URL               string `json:"url,omitempty"`
	HTMLURL           string `json:"html_url,omitempty"`
	FollowersURL      string `json:"followers_url,omitempty"`
	FollowingURL      string `json:"following_url,omitempty"`
	GistsURL          string `json:"gists_url,omitempty"`
	StarredURL        string `json:"starred_url,omitempty"`
	SubscriptionsURL  string `json:"subscriptions_url,omitempty"`
	OrganizationsURL  string `json:"organizations_url,omitempty"`
	ReposURL          string `json:"repos_url,omitempty"`
	EventsURL         string `json:"events_url,omitempty"`
	ReceivedEventsURL string `json:"received_events_url,omitempty"`
	Type              string `json:"type,omitempty"`
	SiteAdmin         bool   `json:"site_admin,omitempty"`
}

type Author struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

type Base struct {
	Ref  string `json:"ref,omitempty"`
	Sha  string `json:"sha,omitempty"`
	Repo Repo   `json:"repo,omitempty"`
}

type Committer struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

type Enterprise struct {
	ID          int       `json:"id,omitempty"`
	Slug        string    `json:"slug,omitempty"`
	Name        string    `json:"name,omitempty"`
	NodeID      string    `json:"node_id,omitempty"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	Description string    `json:"description,omitempty"`
	WebsiteURL  string    `json:"website_url,omitempty"`
	HTMLURL     string    `json:"html_url,omitempty"`
	CreatedAt   time.Time `json:"created_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
}

type HeadCommit struct {
	ID        string    `json:"id,omitempty"`
	TreeID    string    `json:"tree_id,omitempty"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
	Author    Author    `json:"author,omitempty"`
	Committer Committer `json:"committer,omitempty"`
}

type Head struct {
	Ref  string `json:"ref,omitempty"`
	Sha  string `json:"sha,omitempty"`
	Repo Repo   `json:"repo,omitempty"`
}

type HeadRepository struct {
	ID               int    `json:"id,omitempty"`
	NodeID           string `json:"node_id,omitempty"`
	Name             string `json:"name,omitempty"`
	FullName         string `json:"full_name,omitempty"`
	Private          bool   `json:"private,omitempty"`
	Owner            Owner  `json:"owner,omitempty"`
	HTMLURL          string `json:"html_url,omitempty"`
	Description      string `json:"description,omitempty"`
	Fork             bool   `json:"fork,omitempty"`
	URL              string `json:"url,omitempty"`
	ForksURL         string `json:"forks_url,omitempty"`
	KeysURL          string `json:"keys_url,omitempty"`
	CollaboratorsURL string `json:"collaborators_url,omitempty"`
	TeamsURL         string `json:"teams_url,omitempty"`
	HooksURL         string `json:"hooks_url,omitempty"`
	IssueEventsURL   string `json:"issue_events_url,omitempty"`
	EventsURL        string `json:"events_url,omitempty"`
	AssigneesURL     string `json:"assignees_url,omitempty"`
	BranchesURL      string `json:"branches_url,omitempty"`
	TagsURL          string `json:"tags_url,omitempty"`
	BlobsURL         string `json:"blobs_url,omitempty"`
	GitTagsURL       string `json:"git_tags_url,omitempty"`
	GitRefsURL       string `json:"git_refs_url,omitempty"`
	TreesURL         string `json:"trees_url,omitempty"`
	StatusesURL      string `json:"statuses_url,omitempty"`
	LanguagesURL     string `json:"languages_url,omitempty"`
	StargazersURL    string `json:"stargazers_url,omitempty"`
	ContributorsURL  string `json:"contributors_url,omitempty"`
	SubscribersURL   string `json:"subscribers_url,omitempty"`
	SubscriptionURL  string `json:"subscription_url,omitempty"`
	CommitsURL       string `json:"commits_url,omitempty"`
	GitCommitsURL    string `json:"git_commits_url,omitempty"`
	CommentsURL      string `json:"comments_url,omitempty"`
	IssueCommentURL  string `json:"issue_comment_url,omitempty"`
	ContentsURL      string `json:"contents_url,omitempty"`
	CompareURL       string `json:"compare_url,omitempty"`
	MergesURL        string `json:"merges_url,omitempty"`
	ArchiveURL       string `json:"archive_url,omitempty"`
	DownloadsURL     string `json:"downloads_url,omitempty"`
	IssuesURL        string `json:"issues_url,omitempty"`
	PullsURL         string `json:"pulls_url,omitempty"`
	MilestonesURL    string `json:"milestones_url,omitempty"`
	NotificationsURL string `json:"notifications_url,omitempty"`
	LabelsURL        string `json:"labels_url,omitempty"`
	ReleasesURL      string `json:"releases_url,omitempty"`
	DeploymentsURL   string `json:"deployments_url,omitempty"`
}

type Installation struct {
	ID     int    `json:"id,omitempty"`
	NodeID string `json:"node_id,omitempty"`
}

type Organization struct {
	Login            string `json:"login,omitempty"`
	ID               int    `json:"id,omitempty"`
	NodeID           string `json:"node_id,omitempty"`
	URL              string `json:"url,omitempty"`
	ReposURL         string `json:"repos_url,omitempty"`
	EventsURL        string `json:"events_url,omitempty"`
	HooksURL         string `json:"hooks_url,omitempty"`
	IssuesURL        string `json:"issues_url,omitempty"`
	MembersURL       string `json:"members_url,omitempty"`
	PublicMembersURL string `json:"public_members_url,omitempty"`
	AvatarURL        string `json:"avatar_url,omitempty"`
	Description      string `json:"description,omitempty"`
}

type Owner struct {
	Login             string `json:"login,omitempty"`
	ID                int    `json:"id,omitempty"`
	NodeID            string `json:"node_id,omitempty"`
	AvatarURL         string `json:"avatar_url,omitempty"`
	GravatarID        string `json:"gravatar_id,omitempty"`
	URL               string `json:"url,omitempty"`
	HTMLURL           string `json:"html_url,omitempty"`
	FollowersURL      string `json:"followers_url,omitempty"`
	FollowingURL      string `json:"following_url,omitempty"`
	GistsURL          string `json:"gists_url,omitempty"`
	StarredURL        string `json:"starred_url,omitempty"`
	SubscriptionsURL  string `json:"subscriptions_url,omitempty"`
	OrganizationsURL  string `json:"organizations_url,omitempty"`
	ReposURL          string `json:"repos_url,omitempty"`
	EventsURL         string `json:"events_url,omitempty"`
	ReceivedEventsURL string `json:"received_events_url,omitempty"`
	Type              string `json:"type,omitempty"`
	SiteAdmin         bool   `json:"site_admin,omitempty"`
}

type PullRequests struct {
	URL    string `json:"url,omitempty"`
	ID     int    `json:"id,omitempty"`
	Number int    `json:"number,omitempty"`
	Head   Head   `json:"head,omitempty"`
	Base   Base   `json:"base,omitempty"`
}

type ReferencedWorkflows struct {
	Path string `json:"path,omitempty"`
	Sha  string `json:"sha,omitempty"`
	Ref  string `json:"ref,omitempty"`
}

type Repo struct {
	ID   int    `json:"id,omitempty"`
	URL  string `json:"url,omitempty"`
	Name string `json:"name,omitempty"`
}

type Repository struct {
	ID                       int       `json:"id,omitempty"`
	NodeID                   string    `json:"node_id,omitempty"`
	Name                     string    `json:"name,omitempty"`
	FullName                 string    `json:"full_name,omitempty"`
	Private                  bool      `json:"private,omitempty"`
	Owner                    Owner     `json:"owner,omitempty"`
	HTMLURL                  string    `json:"html_url,omitempty"`
	Description              string    `json:"description,omitempty"`
	Fork                     bool      `json:"fork,omitempty"`
	URL                      string    `json:"url,omitempty"`
	ForksURL                 string    `json:"forks_url,omitempty"`
	KeysURL                  string    `json:"keys_url,omitempty"`
	CollaboratorsURL         string    `json:"collaborators_url,omitempty"`
	TeamsURL                 string    `json:"teams_url,omitempty"`
	HooksURL                 string    `json:"hooks_url,omitempty"`
	IssueEventsURL           string    `json:"issue_events_url,omitempty"`
	EventsURL                string    `json:"events_url,omitempty"`
	AssigneesURL             string    `json:"assignees_url,omitempty"`
	BranchesURL              string    `json:"branches_url,omitempty"`
	TagsURL                  string    `json:"tags_url,omitempty"`
	BlobsURL                 string    `json:"blobs_url,omitempty"`
	GitTagsURL               string    `json:"git_tags_url,omitempty"`
	GitRefsURL               string    `json:"git_refs_url,omitempty"`
	TreesURL                 string    `json:"trees_url,omitempty"`
	StatusesURL              string    `json:"statuses_url,omitempty"`
	LanguagesURL             string    `json:"languages_url,omitempty"`
	StargazersURL            string    `json:"stargazers_url,omitempty"`
	ContributorsURL          string    `json:"contributors_url,omitempty"`
	SubscribersURL           string    `json:"subscribers_url,omitempty"`
	SubscriptionURL          string    `json:"subscription_url,omitempty"`
	CommitsURL               string    `json:"commits_url,omitempty"`
	GitCommitsURL            string    `json:"git_commits_url,omitempty"`
	CommentsURL              string    `json:"comments_url,omitempty"`
	IssueCommentURL          string    `json:"issue_comment_url,omitempty"`
	ContentsURL              string    `json:"contents_url,omitempty"`
	CompareURL               string    `json:"compare_url,omitempty"`
	MergesURL                string    `json:"merges_url,omitempty"`
	ArchiveURL               string    `json:"archive_url,omitempty"`
	DownloadsURL             string    `json:"downloads_url,omitempty"`
	IssuesURL                string    `json:"issues_url,omitempty"`
	PullsURL                 string    `json:"pulls_url,omitempty"`
	MilestonesURL            string    `json:"milestones_url,omitempty"`
	NotificationsURL         string    `json:"notifications_url,omitempty"`
	LabelsURL                string    `json:"labels_url,omitempty"`
	ReleasesURL              string    `json:"releases_url,omitempty"`
	DeploymentsURL           string    `json:"deployments_url,omitempty"`
	CreatedAt                time.Time `json:"created_at,omitempty"`
	UpdatedAt                time.Time `json:"updated_at,omitempty"`
	PushedAt                 time.Time `json:"pushed_at,omitempty"`
	GitURL                   string    `json:"git_url,omitempty"`
	SSHURL                   string    `json:"ssh_url,omitempty"`
	CloneURL                 string    `json:"clone_url,omitempty"`
	SvnURL                   string    `json:"svn_url,omitempty"`
	Homepage                 string    `json:"homepage,omitempty"`
	Size                     int       `json:"size,omitempty"`
	StargazersCount          int       `json:"stargazers_count,omitempty"`
	WatchersCount            int       `json:"watchers_count,omitempty"`
	Language                 string    `json:"language,omitempty"`
	HasIssues                bool      `json:"has_issues,omitempty"`
	HasProjects              bool      `json:"has_projects,omitempty"`
	HasDownloads             bool      `json:"has_downloads,omitempty"`
	HasWiki                  bool      `json:"has_wiki,omitempty"`
	HasPages                 bool      `json:"has_pages,omitempty"`
	HasDiscussions           bool      `json:"has_discussions,omitempty"`
	ForksCount               int       `json:"forks_count,omitempty"`
	MirrorURL                string    `json:"mirror_url,omitempty"`
	Archived                 bool      `json:"archived,omitempty"`
	Disabled                 bool      `json:"disabled,omitempty"`
	OpenIssuesCount          int       `json:"open_issues_count,omitempty"`
	License                  string    `json:"license,omitempty"`
	AllowForking             bool      `json:"allow_forking,omitempty"`
	IsTemplate               bool      `json:"is_template,omitempty"`
	WebCommitSignoffRequired bool      `json:"web_commit_signoff_required,omitempty"`
	Topics                   []string  `json:"topics,omitempty"`
	Visibility               string    `json:"visibility,omitempty"`
	Forks                    int       `json:"forks,omitempty"`
	OpenIssues               int       `json:"open_issues,omitempty"`
	Watchers                 int       `json:"watchers,omitempty"`
	DefaultBranch            string    `json:"default_branch,omitempty"`
}

type Sender struct {
	Login             string `json:"login,omitempty"`
	ID                int    `json:"id,omitempty"`
	NodeID            string `json:"node_id,omitempty"`
	AvatarURL         string `json:"avatar_url,omitempty"`
	GravatarID        string `json:"gravatar_id,omitempty"`
	URL               string `json:"url,omitempty"`
	HTMLURL           string `json:"html_url,omitempty"`
	FollowersURL      string `json:"followers_url,omitempty"`
	FollowingURL      string `json:"following_url,omitempty"`
	GistsURL          string `json:"gists_url,omitempty"`
	StarredURL        string `json:"starred_url,omitempty"`
	SubscriptionsURL  string `json:"subscriptions_url,omitempty"`
	OrganizationsURL  string `json:"organizations_url,omitempty"`
	ReposURL          string `json:"repos_url,omitempty"`
	EventsURL         string `json:"events_url,omitempty"`
	ReceivedEventsURL string `json:"received_events_url,omitempty"`
	Type              string `json:"type,omitempty"`
	SiteAdmin         bool   `json:"site_admin,omitempty"`
}

type Step struct {
	Name        string    `json:"name,omitempty"`
	Status      string    `json:"status,omitempty"`
	Conclusion  string    `json:"conclusion,omitempty"`
	Number      int       `json:"number,omitempty"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

type TriggeringActor struct {
	Login             string `json:"login,omitempty"`
	ID                int    `json:"id,omitempty"`
	NodeID            string `json:"node_id,omitempty"`
	AvatarURL         string `json:"avatar_url,omitempty"`
	GravatarID        string `json:"gravatar_id,omitempty"`
	URL               string `json:"url,omitempty"`
	HTMLURL           string `json:"html_url,omitempty"`
	FollowersURL      string `json:"followers_url,omitempty"`
	FollowingURL      string `json:"following_url,omitempty"`
	GistsURL          string `json:"gists_url,omitempty"`
	StarredURL        string `json:"starred_url,omitempty"`
	SubscriptionsURL  string `json:"subscriptions_url,omitempty"`
	OrganizationsURL  string `json:"organizations_url,omitempty"`
	ReposURL          string `json:"repos_url,omitempty"`
	EventsURL         string `json:"events_url,omitempty"`
	ReceivedEventsURL string `json:"received_events_url,omitempty"`
	Type              string `json:"type,omitempty"`
	SiteAdmin         bool   `json:"site_admin,omitempty"`
}

type Workflow struct {
	ID        int       `json:"id,omitempty"`
	NodeID    string    `json:"node_id,omitempty"`
	Name      string    `json:"name,omitempty"`
	Path      string    `json:"path,omitempty"`
	State     string    `json:"state,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	URL       string    `json:"url,omitempty"`
	HTMLURL   string    `json:"html_url,omitempty"`
	BadgeURL  string    `json:"badge_url,omitempty"`
}

type WorkflowJob struct {
	ID              int64     `json:"id"`
	RunID           int64     `json:"run_id"`
	WorkflowName    string    `json:"workflow_name,omitempty"`
	HeadBranch      string    `json:"head_branch,omitempty"`
	RunURL          string    `json:"run_url,omitempty"`
	RunAttempt      int       `json:"run_attempt"`
	NodeID          string    `json:"node_id,omitempty"`
	HeadSha         string    `json:"head_sha,omitempty"`
	URL             string    `json:"url,omitempty"`
	HTMLURL         string    `json:"html_url,omitempty"`
	Status          string    `json:"status,omitempty"`
	Conclusion      string    `json:"conclusion,omitempty"`
	CreatedAt       time.Time `json:"created_at,omitempty"`
	StartedAt       time.Time `json:"started_at,omitempty"`
	CompletedAt     time.Time `json:"completed_at,omitempty"`
	Name            string    `json:"name,omitempty"`
	Steps           []Step    `json:"steps,omitempty"`
	CheckRunURL     string    `json:"check_run_url,omitempty"`
	Labels          []string  `json:"labels,omitempty"`
	RunnerID        int       `json:"runner_id,omitempty"`
	RunnerName      string    `json:"runner_name,omitempty"`
	RunnerGroupID   int       `json:"runner_group_id,omitempty"`
	RunnerGroupName string    `json:"runner_group_name,omitempty"`
}

type WorkflowRun struct {
	ID                  int64                 `json:"id"`
	Name                string                `json:"name,omitempty"`
	NodeID              string                `json:"node_id,omitempty"`
	HeadBranch          string                `json:"head_branch,omitempty"`
	HeadSha             string                `json:"head_sha,omitempty"`
	Path                string                `json:"path,omitempty"`
	DisplayTitle        string                `json:"display_title,omitempty"`
	RunNumber           int                   `json:"run_number,omitempty"`
	Event               string                `json:"event,omitempty"`
	Status              string                `json:"status,omitempty"`
	Conclusion          string                `json:"conclusion,omitempty"`
	WorkflowID          int                   `json:"workflow_id,omitempty"`
	CheckSuiteID        int64                 `json:"check_suite_id,omitempty"`
	CheckSuiteNodeID    string                `json:"check_suite_node_id,omitempty"`
	URL                 string                `json:"url,omitempty"`
	HTMLURL             string                `json:"html_url,omitempty"`
	PullRequests        []PullRequests        `json:"pull_requests,omitempty"`
	CreatedAt           time.Time             `json:"created_at,omitempty"`
	UpdatedAt           time.Time             `json:"updated_at,omitempty"`
	Actor               Actor                 `json:"actor,omitempty"`
	RunAttempt          int                   `json:"run_attempt,omitempty"`
	ReferencedWorkflows []ReferencedWorkflows `json:"referenced_workflows,omitempty"`
	RunStartedAt        time.Time             `json:"run_started_at,omitempty"`
	TriggeringActor     TriggeringActor       `json:"triggering_actor,omitempty"`
	JobsURL             string                `json:"jobs_url,omitempty"`
	LogsURL             string                `json:"logs_url,omitempty"`
	CheckSuiteURL       string                `json:"check_suite_url,omitempty"`
	ArtifactsURL        string                `json:"artifacts_url,omitempty"`
	CancelURL           string                `json:"cancel_url,omitempty"`
	RerunURL            string                `json:"rerun_url,omitempty"`
	PreviousAttemptURL  string                `json:"previous_attempt_url,omitempty"`
	WorkflowURL         string                `json:"workflow_url,omitempty"`
	HeadCommit          HeadCommit            `json:"head_commit,omitempty"`
	Repository          Repository            `json:"repository,omitempty"`
	HeadRepository      HeadRepository        `json:"head_repository,omitempty"`
}
