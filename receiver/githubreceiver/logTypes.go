package githubreceiver

import "time"

// type gitHubLog struct {
// 	logType    string
// 	user       *gitHubUserLog
// 	org        *gitHubOrganizationLog
// 	enterprise *gitHubEnterpriseLog
// }

// GitHubLog represents a log entry for GitHub events.
type GitHubLog interface {
}

// ActorLocation represents the location of the actor.
type ActorLocation struct {
	CountryCode string `json:"country_code"`
}

// gitHubEnterpriseLog represents a log entry for GitHub Enterprise events.
type gitHubEnterpriseLog struct {
	Timestamp                int64         `json:"@timestamp"`
	DocumentID               string        `json:"document_id"`
	Action                   string        `json:"action"`
	Actor                    string        `json:"actor"`
	ActorID                  int64         `json:"actor_id"`
	ActorIsBot               bool          `json:"actor_is_bot,omitempty"`
	ActorIP                  string        `json:"actor_ip,omitempty"`
	ActorLocation            ActorLocation `json:"actor_location,omitempty"`
	CountryCode              string        `json:"country_code,omitempty"`
	Business                 string        `json:"business,omitempty"`
	BusinessID               int64         `json:"business_id,omitempty"`
	CreatedAt                int64         `json:"created_at"`
	OperationType            string        `json:"operation_type,omitempty"`
	UserAgent                string        `json:"user_agent,omitempty"`
	ActorLogin               string        `json:"actor_login,omitempty"`
	ActorLocationCountryCode string        `json:"actor_location_country_code,omitempty"`
	ActorAvatarURL           string        `json:"actor_avatar_url,omitempty"`
	ActorIsEnterpriseOwner   bool          `json:"actor_is_enterprise_owner,omitempty"`
	ActorUserAgent           string        `json:"actor_user_agent,omitempty"`
	Repository               string        `json:"repository,omitempty"`
	RepoPrivate              bool          `json:"repo_private,omitempty"`
	RepoVisibility           string        `json:"repo_visibility,omitempty"`
	TargetLogin              string        `json:"target_login,omitempty"`
	TargetType               string        `json:"target_type,omitempty"`
	Team                     string        `json:"team,omitempty"`
	TeamID                   int64         `json:"team_id,omitempty"`
	TeamSlug                 string        `json:"team_slug,omitempty"`
	Enterprise               string        `json:"enterprise,omitempty"`
	EnterpriseID             int64         `json:"enterprise_id,omitempty"`
	EnterpriseSlug           string        `json:"enterprise_slug,omitempty"`
	User                     string        `json:"user,omitempty"`
	UserID                   int64         `json:"user_id,omitempty"`
	UserLogin                string        `json:"user_login,omitempty"`
	Permission               string        `json:"permission,omitempty"`
	Ref                      string        `json:"ref,omitempty"`
	Branch                   string        `json:"branch,omitempty"`
	Environment              string        `json:"environment,omitempty"`
	Workflow                 string        `json:"workflow,omitempty"`
	Deployment               string        `json:"deployment,omitempty"`
	RunID                    int64         `json:"run_id,omitempty"`
	InstallationID           int64         `json:"installation_id,omitempty"`
	InvitationID             int64         `json:"invitation_id,omitempty"`
	Integration              string        `json:"integration,omitempty"`
	IntegrationID            int64         `json:"integration_id,omitempty"`
	ExternalURL              string        `json:"external_url,omitempty"`
	DocumentationURL         string        `json:"documentation_url,omitempty"`
	EnvironmentName          string        `json:"environment_name,omitempty"`
	JobName                  string        `json:"job_name,omitempty"`
	JobStatus                string        `json:"job_status,omitempty"`
	OrganizationUpgrade      bool          `json:"organization_upgrade,omitempty"`
	Plan                     string        `json:"plan,omitempty"`
	BillingEmail             string        `json:"billing_email,omitempty"`
	AuditLogStreamSink       string        `json:"audit_log_stream_sink,omitempty"`
	AuditLogStreamResult     string        `json:"audit_log_stream_result,omitempty"`
	DeploymentEnvironment    string        `json:"deployment_environment,omitempty"`
	Member                   string        `json:"member,omitempty"`
	MemberLogin              string        `json:"member_login,omitempty"`
	SSHKey                   string        `json:"ssh_key,omitempty"`
	SSHKeyID                 int64         `json:"ssh_key_id,omitempty"`
	TargetID                 int64         `json:"target_id,omitempty"`
	RepositoryID             int64         `json:"repository_id,omitempty"`
	RepositoryPublic         bool          `json:"repository_public,omitempty"`
	RepoOwner                string        `json:"repo_owner,omitempty"`
	RepoOwnerID              int64         `json:"repo_owner_id,omitempty"`
	ProtectedBranch          bool          `json:"protected_branch,omitempty"`
	RefName                  string        `json:"ref_name,omitempty"`
	OrganizationBillingEmail string        `json:"organization_billing_email,omitempty"`
	PreviousPermission       string        `json:"previous_permission,omitempty"`
	HookID                   int64         `json:"hook_id,omitempty"`
	HookURL                  string        `json:"hook_url,omitempty"`
	HookName                 string        `json:"hook_name,omitempty"`
	BranchProtection         string        `json:"branch_protection,omitempty"`
	WorkflowRunID            int64         `json:"workflow_run_id,omitempty"`
	WorkflowFileName         string        `json:"workflow_file_name,omitempty"`
	WorkflowFilePath         string        `json:"workflow_file_path,omitempty"`
	RunAttempt               int64         `json:"run_attempt,omitempty"`
	WorkflowRunStartedAt     int64         `json:"workflow_run_started_at,omitempty"`
	WorkflowRunConclusion    string        `json:"workflow_run_conclusion,omitempty"`
	SSHKeyTitle              string        `json:"ssh_key_title,omitempty"`
	SSHKeyFingerprint        string        `json:"ssh_key_fingerprint,omitempty"`
	OAuthTokenID             int64         `json:"oauth_token_id,omitempty"`
	OAuthTokenName           string        `json:"oauth_token_name,omitempty"`
	ApplicationID            int64         `json:"application_id,omitempty"`
	ApplicationName          string        `json:"application_name,omitempty"`
	License                  string        `json:"license,omitempty"`
	LicenseExpiry            int64         `json:"license_expiry,omitempty"`
	SAMLNameID               string        `json:"saml_name_id,omitempty"`
	SAMLNameIDEmail          string        `json:"saml_name_id_email,omitempty"`
	SAMLNameIDUser           string        `json:"saml_name_id_user,omitempty"`
	SSOID                    int64         `json:"sso_id,omitempty"`
	SSOName                  string        `json:"sso_name,omitempty"`
	TwoFAEnforcement         bool          `json:"2fa_enforcement,omitempty"`
	TwoFAType                string        `json:"2fa_type,omitempty"`
	OrgName                  string        `json:"org_name,omitempty"`
	OrgRole                  string        `json:"org_role,omitempty"`
	OrgLogin                 string        `json:"org_login,omitempty"`
	ActionDescription        string        `json:"action_description,omitempty"`
	LDAPDN                   string        `json:"ldap_dn,omitempty"`
	MFA                      string        `json:"mfa,omitempty"`
	MFAEnrollment            bool          `json:"mfa_enrollment,omitempty"`
	Name                     string        `json:"name,omitempty"`
	Org                      string        `json:"org,omitempty"`
	OrgID                    int64         `json:"org_id,omitempty"`
	OwnerType                string        `json:"owner_type,omitempty"`
}

// gitHubOrganizationLog represents a log entry for GitHub organization events.
type gitHubOrganizationLog struct {
	Timestamp                int64         `json:"@timestamp"`
	DocumentID               string        `json:"document_id"`
	Action                   string        `json:"action"`
	Actor                    string        `json:"actor"`
	ActorID                  int64         `json:"actor_id"`
	ActorIP                  string        `json:"actor_ip,omitempty"`
	ActorLocation            ActorLocation `json:"actor_location,omitempty"`
	CountryCode              string        `json:"country_code,omitempty"`
	ActorLogin               string        `json:"actor_login,omitempty"`
	ActorLocationCountryCode string        `json:"actor_location_country_code,omitempty"`
	ActorAvatarURL           string        `json:"actor_avatar_url,omitempty"`
	Business                 string        `json:"business,omitempty"`
	BusinessID               int64         `json:"business_id,omitempty"`
	CreatedAt                int64         `json:"created_at"`
	OperationType            string        `json:"operation_type,omitempty"`
	Repository               string        `json:"repository,omitempty"`
	RepoPrivate              bool          `json:"repo_private,omitempty"`
	RepoVisibility           string        `json:"repo_visibility,omitempty"`
	TargetLogin              string        `json:"target_login,omitempty"`
	TargetType               string        `json:"target_type,omitempty"`
	Team                     string        `json:"team,omitempty"`
	TeamID                   int64         `json:"team_id,omitempty"`
	TeamName                 string        `json:"team_name,omitempty"`
	TeamSlug                 string        `json:"team_slug,omitempty"`
	Org                      string        `json:"org,omitempty"`
	OrgID                    int64         `json:"org_id,omitempty"`
	OrgLogin                 string        `json:"org_login,omitempty"`
	User                     string        `json:"user,omitempty"`
	UserID                   int64         `json:"user_id,omitempty"`
	UserLogin                string        `json:"user_login,omitempty"`
	Permission               string        `json:"permission,omitempty"`
	Ref                      string        `json:"ref,omitempty"`
	Branch                   string        `json:"branch,omitempty"`
	Environment              string        `json:"environment,omitempty"`
	Workflow                 string        `json:"workflow,omitempty"`
	Deployment               string        `json:"deployment,omitempty"`
	RunID                    int64         `json:"run_id,omitempty"`
	InstallationID           int64         `json:"installation_id,omitempty"`
	InvitationID             int64         `json:"invitation_id,omitempty"`
	Integration              string        `json:"integration,omitempty"`
	IntegrationID            int64         `json:"integration_id,omitempty"`
	ExternalURL              string        `json:"external_url,omitempty"`
	DocumentationURL         string        `json:"documentation_url,omitempty"`
	EnvironmentName          string        `json:"environment_name,omitempty"`
	JobName                  string        `json:"job_name,omitempty"`
	JobStatus                string        `json:"job_status,omitempty"`
	OrganizationUpgrade      bool          `json:"organization_upgrade,omitempty"`
	Plan                     string        `json:"plan,omitempty"`
	BillingEmail             string        `json:"billing_email,omitempty"`
	AuditLogStreamSink       string        `json:"audit_log_stream_sink,omitempty"`
	AuditLogStreamResult     string        `json:"audit_log_stream_result,omitempty"`
	DeploymentEnvironment    string        `json:"deployment_environment,omitempty"`
	Enterprise               string        `json:"enterprise,omitempty"`
	EnterpriseID             int64         `json:"enterprise_id,omitempty"`
	Member                   string        `json:"member,omitempty"`
	MemberLogin              string        `json:"member_login,omitempty"`
	ActorIsBot               bool          `json:"actor_is_bot,omitempty"`
	ActorEnterpriseOwner     bool          `json:"actor_enterprise_owner,omitempty"`
	SSHKey                   string        `json:"ssh_key,omitempty"`
	SSHKeyID                 int64         `json:"ssh_key_id,omitempty"`
	TargetID                 int64         `json:"target_id,omitempty"`
	RepositoryID             int64         `json:"repository_id,omitempty"`
	RepositoryPublic         bool          `json:"repository_public,omitempty"`
	RepoOwner                string        `json:"repo_owner,omitempty"`
	RepoOwnerID              int64         `json:"repo_owner_id,omitempty"`
	ProtectedBranch          bool          `json:"protected_branch,omitempty"`
	RefName                  string        `json:"ref_name,omitempty"`
	OrganizationBillingEmail string        `json:"organization_billing_email,omitempty"`
	PreviousPermission       string        `json:"previous_permission,omitempty"`
	HookID                   int64         `json:"hook_id,omitempty"`
	HookURL                  string        `json:"hook_url,omitempty"`
	HookName                 string        `json:"hook_name,omitempty"`
	BranchProtection         string        `json:"branch_protection,omitempty"`
	WorkflowRunID            int64         `json:"workflow_run_id,omitempty"`
	WorkflowFileName         string        `json:"workflow_file_name,omitempty"`
	WorkflowFilePath         string        `json:"workflow_file_path,omitempty"`
	RunAttempt               int64         `json:"run_attempt,omitempty"`
	WorkflowRunStartedAt     int64         `json:"workflow_run_started_at,omitempty"`
	WorkflowRunConclusion    string        `json:"workflow_run_conclusion,omitempty"`
	SSHKeyTitle              string        `json:"ssh_key_title,omitempty"`
	SSHKeyFingerprint        string        `json:"ssh_key_fingerprint,omitempty"`
}

// gitHubUserLog represents a log entry for GitHub user events.
type gitHubUserLog struct {
	ID        string       `json:"id"`
	Type      string       `json:"type"`
	Actor     Actor        `json:"actor"`
	Repo      Repository   `json:"repo"`
	Payload   interface{}  `json:"payload,omitempty"`
	Org       Organization `json:"org,omitempty"`
	Public    bool         `json:"public"`
	CreatedAt time.Time    `json:"created_at"`
}

// Actor represents the user who performed the action.
type Actor struct {
	ID           int64  `json:"id"`
	Login        string `json:"login"`
	DisplayLogin string `json:"display_login,omitempty"`
	GravatarID   string `json:"gravatar_id,omitempty"`
	URL          string `json:"url"`
	AvatarURL    string `json:"avatar_url"`
}

// Repository represents a GitHub repository.
type Repository struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Organization represents a GitHub organization.
type Organization struct {
	ID         int64  `json:"id,omitempty"`
	Login      string `json:"login,omitempty"`
	GravatarID string `json:"gravatar_id,omitempty"`
	URL        string `json:"url,omitempty"`
	AvatarURL  string `json:"avatar_url,omitempty"`
}

// PushEvent represents a push event from GitHub.
type PushEvent struct {
	Ref        string     `json:"ref"`
	Before     string     `json:"before"`
	After      string     `json:"after"`
	Commits    []Commit   `json:"commits"`
	Size       int        `json:"size"`
	Pusher     User       `json:"pusher"`
	Repository Repository `json:"repository"`
}

// Commit represents a commit in a push event.
type Commit struct {
	SHA      string `json:"sha"`
	Author   Author `json:"author"`
	Message  string `json:"message"`
	Distinct bool   `json:"distinct"`
	URL      string `json:"url"`
}

// Author represents the author of a commit.
type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Date  string `json:"date"`
}

// User represents a GitHub user
type User struct {
	Login             string `json:"login"`
	ID                int    `json:"id"`
	NodeID            string `json:"node_id"`
	AvatarURL         string `json:"avatar_url"`
	GravatarID        string `json:"gravatar_id"`
	URL               string `json:"url"`
	HTMLURL           string `json:"html_url"`
	FollowersURL      string `json:"followers_url"`
	FollowingURL      string `json:"following_url"`
	GistsURL          string `json:"gists_url"`
	StarredURL        string `json:"starred_url"`
	SubscriptionsURL  string `json:"subscriptions_url"`
	OrganizationsURL  string `json:"organizations_url"`
	ReposURL          string `json:"repos_url"`
	EventsURL         string `json:"events_url"`
	ReceivedEventsURL string `json:"received_events_url"`
	Type              string `json:"type"`
	SiteAdmin         bool   `json:"site_admin"`
}

// Repo represents a GitHub repository.
type Repo struct {
	ID                       int       `json:"id"`
	NodeID                   string    `json:"node_id"`
	Name                     string    `json:"name"`
	FullName                 string    `json:"full_name"`
	Private                  bool      `json:"private"`
	Owner                    User      `json:"owner"`
	HTMLURL                  string    `json:"html_url"`
	Description              string    `json:"description"`
	Fork                     bool      `json:"fork"`
	URL                      string    `json:"url"`
	ForksURL                 string    `json:"forks_url"`
	KeysURL                  string    `json:"keys_url"`
	CollaboratorsURL         string    `json:"collaborators_url"`
	TeamsURL                 string    `json:"teams_url"`
	HooksURL                 string    `json:"hooks_url"`
	IssueEventsURL           string    `json:"issue_events_url"`
	EventsURL                string    `json:"events_url"`
	AssigneesURL             string    `json:"assignees_url"`
	BranchesURL              string    `json:"branches_url"`
	TagsURL                  string    `json:"tags_url"`
	BlobsURL                 string    `json:"blobs_url"`
	GitTagsURL               string    `json:"git_tags_url"`
	GitRefsURL               string    `json:"git_refs_url"`
	TreesURL                 string    `json:"trees_url"`
	StatusesURL              string    `json:"statuses_url"`
	LanguagesURL             string    `json:"languages_url"`
	StargazersURL            string    `json:"stargazers_url"`
	ContributorsURL          string    `json:"contributors_url"`
	SubscribersURL           string    `json:"subscribers_url"`
	SubscriptionURL          string    `json:"subscription_url"`
	CommitsURL               string    `json:"commits_url"`
	GitCommitsURL            string    `json:"git_commits_url"`
	CommentsURL              string    `json:"comments_url"`
	IssueCommentURL          string    `json:"issue_comment_url"`
	ContentsURL              string    `json:"contents_url"`
	CompareURL               string    `json:"compare_url"`
	MergesURL                string    `json:"merges_url"`
	ArchiveURL               string    `json:"archive_url"`
	DownloadsURL             string    `json:"downloads_url"`
	IssuesURL                string    `json:"issues_url"`
	PullsURL                 string    `json:"pulls_url"`
	MilestonesURL            string    `json:"milestones_url"`
	NotificationsURL         string    `json:"notifications_url"`
	LabelsURL                string    `json:"labels_url"`
	ReleasesURL              string    `json:"releases_url"`
	DeploymentsURL           string    `json:"deployments_url"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
	PushedAt                 time.Time `json:"pushed_at"`
	GitURL                   string    `json:"git_url"`
	SSHURL                   string    `json:"ssh_url"`
	CloneURL                 string    `json:"clone_url"`
	SVNURL                   string    `json:"svn_url"`
	Homepage                 string    `json:"homepage"`
	Size                     int       `json:"size"`
	StargazersCount          int       `json:"stargazers_count"`
	WatchersCount            int       `json:"watchers_count"`
	Language                 string    `json:"language"`
	HasIssues                bool      `json:"has_issues"`
	HasProjects              bool      `json:"has_projects"`
	HasDownloads             bool      `json:"has_downloads"`
	HasWiki                  bool      `json:"has_wiki"`
	HasPages                 bool      `json:"has_pages"`
	HasDiscussions           bool      `json:"has_discussions"`
	ForksCount               int       `json:"forks_count"`
	MirrorURL                string    `json:"mirror_url"`
	Archived                 bool      `json:"archived"`
	Disabled                 bool      `json:"disabled"`
	OpenIssuesCount          int       `json:"open_issues_count"`
	License                  License   `json:"license"`
	AllowForking             bool      `json:"allow_forking"`
	IsTemplate               bool      `json:"is_template"`
	WebCommitSignoffRequired bool      `json:"web_commit_signoff_required"`
	Topics                   []string  `json:"topics"`
	Visibility               string    `json:"visibility"`
	Forks                    int       `json:"forks"`
	OpenIssues               int       `json:"open_issues"`
	Watchers                 int       `json:"watchers"`
	DefaultBranch            string    `json:"default_branch"`
}

// License represents the license of a repository.
type License struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	SpdxID string `json:"spdx_id"`
	URL    string `json:"url"`
	NodeID string `json:"node_id"`
}

// Links represents the links for a pull request.
type Links struct {
	Self           Href `json:"self"`
	HTML           Href `json:"html"`
	Issue          Href `json:"issue"`
	Comments       Href `json:"comments"`
	ReviewComments Href `json:"review_comments"`
	ReviewComment  Href `json:"review_comment"`
	Commits        Href `json:"commits"`
	Statuses       Href `json:"statuses"`
}

// Href represents a link.
type Href struct {
	Href string `json:"href"`
}

// PullRequestEvent represents a pull request event from GitHub.
type PullRequestEvent struct {
	Action      string      `json:"action"`
	Number      int         `json:"number"`
	PullRequest PullRequest `json:"pull_request"`
}

// PullRequest represents a GitHub pull request.
type PullRequest struct {
	URL                string     `json:"url"`
	ID                 int        `json:"id"`
	NodeID             string     `json:"node_id"`
	HTMLURL            string     `json:"html_url"`
	DiffURL            string     `json:"diff_url"`
	PatchURL           string     `json:"patch_url"`
	IssueURL           string     `json:"issue_url"`
	Number             int        `json:"number"`
	State              string     `json:"state"`
	Locked             bool       `json:"locked"`
	Title              string     `json:"title"`
	User               User       `json:"user"`
	Body               string     `json:"body"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	ClosedAt           *time.Time `json:"closed_at"`
	MergedAt           *time.Time `json:"merged_at"`
	MergeCommitSHA     string     `json:"merge_commit_sha"`
	Assignee           *User      `json:"assignee"`
	Assignees          []User     `json:"assignees"`
	RequestedReviewers []User     `json:"requested_reviewers"`
	RequestedTeams     []string   `json:"requested_teams"`
	Labels             []string   `json:"labels"`
	Milestone          *string    `json:"milestone"`
	Draft              bool       `json:"draft"`
	CommitsURL         string     `json:"commits_url"`
	ReviewCommentsURL  string     `json:"review_comments_url"`
	ReviewCommentURL   string     `json:"review_comment_url"`
	CommentsURL        string     `json:"comments_url"`
	StatusesURL        string     `json:"statuses_url"`
	Head               Branch     `json:"head"`
	Base               Branch     `json:"base"`
	Links              Links      `json:"_links"`
	AuthorAssociation  string     `json:"author_association"`
	AutoMerge          *string    `json:"auto_merge"`
	ActiveLockReason   *string    `json:"active_lock_reason"`
}

// Branch represents a GitHub branch.
type Branch struct {
	Label string `json:"label"`
	Ref   string `json:"ref"`
	SHA   string `json:"sha"`
	User  User   `json:"user"`
	Repo  Repo   `json:"repo"`
}

// DeleteEvent represents a delete event from GitHub.
type DeleteEvent struct {
	Ref        string `json:"ref,omitempty"`
	RefType    string `json:"ref_type,omitempty"`
	PusherType string `json:"pusher_type,omitempty"`
}

// WatchEvent represents a watch event from GitHub.
type WatchEvent struct {
	Action string `json:"action,omitempty"`
}

// ReleaseEvent represents a release event from GitHub.
type ReleaseEvent struct {
	Action  string `json:"action,omitempty"`
	Release struct {
		ID              int64  `json:"id,omitempty"`
		TagName         string `json:"tag_name,omitempty"`
		TargetCommitish string `json:"target_commitish,omitempty"`
		Name            string `json:"name,omitempty"`
		Body            string `json:"body,omitempty"`
	} `json:"release,omitempty"`
}
