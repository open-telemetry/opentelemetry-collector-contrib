// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubactionsreceiver

import (
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"
)

func createResourceAttributes(resource pcommon.Resource, event interface{}, config *Config, logger *zap.Logger) {
	attrs := resource.Attributes()

	switch e := event.(type) {
	case *WorkflowJobEvent:
		serviceName := generateServiceName(config, e.Repository.FullName)
		attrs.PutStr("service.name", serviceName)

		attrs.PutStr("ci.github.workflow.name", e.WorkflowJob.WorkflowName)

		attrs.PutStr("ci.github.workflow.job.created_at", e.WorkflowJob.CreatedAt.Format(time.RFC3339))
		attrs.PutStr("ci.github.workflow.job.completed_at", e.WorkflowJob.CompletedAt.Format(time.RFC3339))
		attrs.PutStr("ci.github.workflow.job.conclusion", e.WorkflowJob.Conclusion)
		attrs.PutStr("ci.github.workflow.job.head_branch", e.WorkflowJob.HeadBranch)
		attrs.PutStr("ci.github.workflow.job.head_sha", e.WorkflowJob.HeadSha)
		attrs.PutStr("ci.github.workflow.job.html_url", e.WorkflowJob.HTMLURL)
		attrs.PutInt("ci.github.workflow.job.id", e.WorkflowJob.ID)

		if len(e.WorkflowJob.Labels) > 0 {
			for i, label := range e.WorkflowJob.Labels {
				e.WorkflowJob.Labels[i] = strings.ToLower(label)
			}
			sort.Strings(e.WorkflowJob.Labels)
			joinedLabels := strings.Join(e.WorkflowJob.Labels, ",")
			attrs.PutStr("ci.github.workflow.job.labels", joinedLabels)
		} else {
			attrs.PutStr("ci.github.workflow.job.labels", "no labels")
		}

		attrs.PutStr("ci.github.workflow.job.name", e.WorkflowJob.Name)
		attrs.PutInt("ci.github.workflow.job.run_attempt", int64(e.WorkflowJob.RunAttempt))
		attrs.PutInt("ci.github.workflow.job.run_id", e.WorkflowJob.RunID)
		attrs.PutStr("ci.github.workflow.job.runner.group_name", e.WorkflowJob.RunnerGroupName)
		attrs.PutStr("ci.github.workflow.job.runner.name", e.WorkflowJob.RunnerName)
		attrs.PutStr("ci.github.workflow.job.sender.login", e.Sender.Login)
		attrs.PutStr("ci.github.workflow.job.started_at", e.WorkflowJob.StartedAt.Format(time.RFC3339))
		attrs.PutStr("ci.github.workflow.job.status", e.WorkflowJob.Status)

		attrs.PutStr("ci.system", "github")

		attrs.PutStr("scm.git.repo.owner.login", e.Repository.Owner.Login)
		attrs.PutStr("scm.git.repo", e.Repository.FullName)

	case *WorkflowRunEvent:
		serviceName := generateServiceName(config, e.Repository.FullName)
		attrs.PutStr("service.name", serviceName)

		attrs.PutStr("ci.github.workflow.run.actor.login", e.WorkflowRun.Actor.Login)

		attrs.PutStr("ci.github.workflow.run.conclusion", e.WorkflowRun.Conclusion)
		attrs.PutStr("ci.github.workflow.run.created_at", e.WorkflowRun.CreatedAt.Format(time.RFC3339))
		attrs.PutStr("ci.github.workflow.run.display_title", e.WorkflowRun.DisplayTitle)
		attrs.PutStr("ci.github.workflow.run.event", e.WorkflowRun.Event)
		attrs.PutStr("ci.github.workflow.run.head_branch", e.WorkflowRun.HeadBranch)
		attrs.PutStr("ci.github.workflow.run.head_sha", e.WorkflowRun.HeadSha)
		attrs.PutStr("ci.github.workflow.run.html_url", e.WorkflowRun.HTMLURL)
		attrs.PutInt("ci.github.workflow.run.id", e.WorkflowRun.ID)
		attrs.PutStr("ci.github.workflow.run.name", e.WorkflowRun.Name)
		attrs.PutStr("ci.github.workflow.run.path", e.WorkflowRun.Path)

		if e.WorkflowRun.PreviousAttemptURL != "" {
			htmlURL := transformGitHubAPIURL(e.WorkflowRun.PreviousAttemptURL)
			attrs.PutStr("ci.github.workflow.run.previous_attempt_url", htmlURL)
		}

		if len(e.WorkflowRun.ReferencedWorkflows) > 0 {
			var referencedWorkflows []string
			for _, workflow := range e.WorkflowRun.ReferencedWorkflows {
				referencedWorkflows = append(referencedWorkflows, workflow.Path)
			}
			attrs.PutStr("ci.github.workflow.run.referenced_workflows", strings.Join(referencedWorkflows, ";"))
		}

		attrs.PutInt("ci.github.workflow.run.run_attempt", int64(e.WorkflowRun.RunAttempt))
		attrs.PutStr("ci.github.workflow.run.run_started_at", e.WorkflowRun.RunStartedAt.Format(time.RFC3339))
		attrs.PutStr("ci.github.workflow.run.status", e.WorkflowRun.Status)
		attrs.PutStr("ci.github.workflow.run.sender.login", e.Sender.Login)
		attrs.PutStr("ci.github.workflow.run.triggering_actor.login", e.WorkflowRun.TriggeringActor.Login)
		attrs.PutStr("ci.github.workflow.run.updated_at", e.WorkflowRun.UpdatedAt.Format(time.RFC3339))

		attrs.PutStr("ci.system", "github")

		attrs.PutStr("scm.system", "git")

		attrs.PutStr("scm.git.head_branch", e.WorkflowRun.HeadBranch)
		attrs.PutStr("scm.git.head_commit.author.email", e.WorkflowRun.HeadCommit.Author.Email)
		attrs.PutStr("scm.git.head_commit.author.name", e.WorkflowRun.HeadCommit.Author.Name)
		attrs.PutStr("scm.git.head_commit.committer.email", e.WorkflowRun.HeadCommit.Committer.Email)
		attrs.PutStr("scm.git.head_commit.committer.name", e.WorkflowRun.HeadCommit.Committer.Name)
		attrs.PutStr("scm.git.head_commit.message", e.WorkflowRun.HeadCommit.Message)
		attrs.PutStr("scm.git.head_commit.timestamp", e.WorkflowRun.HeadCommit.Timestamp.Format(time.RFC3339))
		attrs.PutStr("scm.git.head_sha", e.WorkflowRun.HeadSha)

		if len(e.WorkflowRun.PullRequests) > 0 {
			var prUrls []string
			for _, pr := range e.WorkflowRun.PullRequests {
				prUrls = append(prUrls, convertPRURL(pr.URL))
			}
			attrs.PutStr("scm.git.pull_requests.url", strings.Join(prUrls, ";"))
		}

		attrs.PutStr("scm.git.repo", e.Repository.FullName)

	default:
		logger.Error("unknown event type")
	}
}
