// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubactionsreceiver

import (
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/v61/github"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

func createResourceAttributes(resource pcommon.Resource, event interface{}, config *Config, logger *zap.Logger) {
	attrs := resource.Attributes()

	switch e := event.(type) {
	case *github.WorkflowJobEvent:
		serviceName := generateServiceName(config, e.GetRepo().GetFullName())
		attrs.PutStr("service.name", serviceName)

		attrs.PutStr("ci.github.workflow.name", e.GetWorkflowJob().GetWorkflowName())

		attrs.PutStr("ci.github.workflow.job.created_at", e.GetWorkflowJob().GetCreatedAt().Format(time.RFC3339))
		attrs.PutStr("ci.github.workflow.job.completed_at", e.GetWorkflowJob().GetCompletedAt().Format(time.RFC3339))
		attrs.PutStr("ci.github.workflow.job.conclusion", e.GetWorkflowJob().GetConclusion())
		attrs.PutStr("ci.github.workflow.job.head_branch", e.GetWorkflowJob().GetHeadBranch())
		attrs.PutStr("ci.github.workflow.job.head_sha", e.GetWorkflowJob().GetHeadSHA())
		attrs.PutStr("ci.github.workflow.job.html_url", e.GetWorkflowJob().GetHTMLURL())
		attrs.PutInt("ci.github.workflow.job.id", e.GetWorkflowJob().GetID())

		if len(e.WorkflowJob.Labels) > 0 {
			labels := e.GetWorkflowJob().Labels
			for i, label := range labels {
				labels[i] = strings.ToLower(label)
			}
			sort.Strings(labels)
			joinedLabels := strings.Join(labels, ",")
			attrs.PutStr("ci.github.workflow.job.labels", joinedLabels)
		} else {
			attrs.PutStr("ci.github.workflow.job.labels", "no labels")
		}

		attrs.PutStr("ci.github.workflow.job.name", e.GetWorkflowJob().GetName())
		attrs.PutInt("ci.github.workflow.job.run_attempt", e.GetWorkflowJob().GetRunAttempt())
		attrs.PutInt("ci.github.workflow.job.run_id", e.GetWorkflowJob().GetRunID())
		attrs.PutStr("ci.github.workflow.job.runner.group_name", e.GetWorkflowJob().GetRunnerGroupName())
		attrs.PutStr("ci.github.workflow.job.runner.name", e.GetWorkflowJob().GetRunnerName())
		attrs.PutStr("ci.github.workflow.job.sender.login", e.GetSender().GetLogin())
		attrs.PutStr("ci.github.workflow.job.started_at", e.GetWorkflowJob().GetStartedAt().Format(time.RFC3339))
		attrs.PutStr("ci.github.workflow.job.status", e.GetWorkflowJob().GetStatus())

		attrs.PutStr("ci.system", "github")

		attrs.PutStr("scm.git.repo.owner.login", e.GetRepo().GetOwner().GetLogin())
		attrs.PutStr("scm.git.repo", e.GetRepo().GetFullName())

	case *github.WorkflowRunEvent:
		serviceName := generateServiceName(config, e.GetRepo().GetFullName())
		attrs.PutStr("service.name", serviceName)

		attrs.PutStr("ci.github.workflow.run.actor.login", e.GetWorkflowRun().GetActor().GetLogin())

		attrs.PutStr("ci.github.workflow.run.conclusion", e.GetWorkflowRun().GetConclusion())
		attrs.PutStr("ci.github.workflow.run.created_at", e.GetWorkflowRun().GetCreatedAt().Format(time.RFC3339))
		attrs.PutStr("ci.github.workflow.run.display_title", e.GetWorkflowRun().GetDisplayTitle())
		attrs.PutStr("ci.github.workflow.run.event", e.GetWorkflowRun().GetEvent())
		attrs.PutStr("ci.github.workflow.run.head_branch", e.GetWorkflowRun().GetHeadBranch())
		attrs.PutStr("ci.github.workflow.run.head_sha", e.GetWorkflowRun().GetHeadSHA())
		attrs.PutStr("ci.github.workflow.run.html_url", e.GetWorkflowRun().GetHTMLURL())
		attrs.PutInt("ci.github.workflow.run.id", e.GetWorkflowRun().GetID())
		attrs.PutStr("ci.github.workflow.run.name", e.GetWorkflowRun().GetName())
		attrs.PutStr("ci.github.workflow.run.path", e.GetWorkflow().GetPath())

		if e.GetWorkflowRun().GetPreviousAttemptURL() != "" {
			htmlURL := transformGitHubAPIURL(e.GetWorkflowRun().GetPreviousAttemptURL())
			attrs.PutStr("ci.github.workflow.run.previous_attempt_url", htmlURL)
		}

		if len(e.GetWorkflowRun().ReferencedWorkflows) > 0 {
			var referencedWorkflows []string
			for _, workflow := range e.GetWorkflowRun().ReferencedWorkflows {
				referencedWorkflows = append(referencedWorkflows, workflow.GetPath())
			}
			attrs.PutStr("ci.github.workflow.run.referenced_workflows", strings.Join(referencedWorkflows, ";"))
		}

		attrs.PutInt("ci.github.workflow.run.run_attempt", int64(e.GetWorkflowRun().GetRunAttempt()))
		attrs.PutStr("ci.github.workflow.run.run_started_at", e.GetWorkflowRun().RunStartedAt.Format(time.RFC3339))
		attrs.PutStr("ci.github.workflow.run.status", e.GetWorkflowRun().GetStatus())
		attrs.PutStr("ci.github.workflow.run.sender.login", e.GetSender().GetLogin())
		attrs.PutStr("ci.github.workflow.run.triggering_actor.login", e.GetWorkflowRun().GetTriggeringActor().GetLogin())
		attrs.PutStr("ci.github.workflow.run.updated_at", e.GetWorkflowRun().GetUpdatedAt().Format(time.RFC3339))

		attrs.PutStr("ci.system", "github")

		attrs.PutStr("scm.system", "git")

		attrs.PutStr("scm.git.head_branch", e.GetWorkflowRun().GetHeadBranch())
		attrs.PutStr("scm.git.head_commit.author.email", e.GetWorkflowRun().GetHeadCommit().GetAuthor().GetEmail())
		attrs.PutStr("scm.git.head_commit.author.name", e.GetWorkflowRun().GetHeadCommit().GetAuthor().GetName())
		attrs.PutStr("scm.git.head_commit.committer.email", e.GetWorkflowRun().GetHeadCommit().GetCommitter().GetEmail())
		attrs.PutStr("scm.git.head_commit.committer.name", e.GetWorkflowRun().GetHeadCommit().GetCommitter().GetName())
		attrs.PutStr("scm.git.head_commit.message", e.GetWorkflowRun().GetHeadCommit().GetMessage())
		attrs.PutStr("scm.git.head_commit.timestamp", e.GetWorkflowRun().GetHeadCommit().GetTimestamp().Format(time.RFC3339))
		attrs.PutStr("scm.git.head_sha", e.GetWorkflowRun().GetHeadSHA())

		if len(e.GetWorkflowRun().PullRequests) > 0 {
			var prUrls []string
			for _, pr := range e.GetWorkflowRun().PullRequests {
				prUrls = append(prUrls, convertPRURL(pr.GetURL()))
			}
			attrs.PutStr("scm.git.pull_requests.url", strings.Join(prUrls, ";"))
		}

		attrs.PutStr("scm.git.repo", e.GetRepo().GetFullName())

	default:
		logger.Error("unknown event type")
	}
}
