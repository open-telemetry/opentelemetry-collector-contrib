// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	"errors"
	"strings"

	"github.com/google/go-github/v68/github"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/collector/semconv/v1.27.0"
)

// model.go contains specific attributes from the 1.28 and 1.29 releases of
// SemConv. They are manually added due to issue
// https://github.com/open-telemetry/weaver/issues/227 which will migrate code
// gen to weaver. Once that is done, these attributes will be migrated to the
// semantic conventions package.
const (
	// vcs.change.state with enum values of open, closed, or merged.
	AttributeVCSChangeState       = "vcs.change.state"
	AttributeVCSChangeStateOpen   = "open"
	AttributeVCSChangeStateClosed = "closed"
	AttributeVCSChangeStateMerged = "merged"

	// TODO: Eveluate whether or not this should be a head title attribute
	// vcs.change.title
	AttributeVCSChangeTitle = "vcs.change.title"

	// vcs.change.id
	AttributeVCSChangeID = "vcs.change.id"

	// vcs.revision_delta.direction with enum values of behind or ahead.
	AttributeVCSRevisionDeltaDirection       = "vcs.revision_delta.direction"
	AttributeVCSRevisionDeltaDirectionBehind = "behind"
	AttributeVCSRevisionDeltaDirectionAhead  = "ahead"

	// vcs.line_change.type with enum values of added or removed.
	AttributeVCSLineChangeType        = "vcs.line_change.type"
	AttributeVCSLineChangeTypeAdded   = "added"
	AttributeVCSLineChangeTypeRemoved = "removed"

	// vcs.ref.type with enum values of branch or tag.
	AttributeVCSRefType       = "vcs.ref.type"
	AttributeVCSRefTypeBranch = "branch"
	AttributeVCSRefTypeTag    = "tag"

	// vcs.repository.name
	AttributeVCSRepositoryName = "vcs.repository.name"

	// vcs.ref.base.name
	AttributeVCSRefBase = "vcs.ref.base"

	// vcs.ref.base.revision
	AttributeVCSRefBaseRevision = "vcs.ref.base.revision"

	// vcs.ref.base.type with enum values of branch or tag.
	AttributeVCSRefBaseType       = "vcs.ref.base.type"
	AttributeVCSRefBaseTypeBranch = "branch"
	AttributeVCSRefBaseTypeTag    = "tag"

	// vcs.ref.head.name
	AttributeVCSRefHead = "vcs.ref.head"

	// vcs.ref.head.revision
	AttributeVCSRefHeadRevision = "vcs.ref.head.revision"

	// vcs.ref.head.type with enum values of branch or tag.
	AttributeVCSRefHeadType       = "vcs.ref.head.type"
	AttributeVCSRefHeadTypeBranch = "branch"
	AttributeVCSRefHeadTypeTag    = "tag"

	// The following prototype attributes that do not exist yet in semconv.
	// They are highly experimental and subject to change.

	AttributeCICDPipelineRunURLFull = "cicd.pipeline.run.url.full" // equivalent to GitHub's `html_url`

	// These are being added in https://github.com/open-telemetry/semantic-conventions/pull/1681
	AttributeCICDPipelineRunStatus             = "cicd.pipeline.run.status" // equivalent to GitHub's `conclusion`
	AttributeCICDPipelineRunStatusSuccess      = "success"
	AttributeCICDPipelineRunStatusFailure      = "failure"
	AttributeCICDPipelineRunStatusCancellation = "cancellation"
	AttributeCICDPipelineRunStatusError        = "error"
	AttributeCICDPipelineRunStatusSkip         = "skip"

	AttributeCICDPipelineTaskRunStatus             = "cicd.pipeline.run.task.status" // equivalent to GitHub's `conclusion`
	AttributeCICDPipelineTaskRunStatusSuccess      = "success"
	AttributeCICDPipelineTaskRunStatusFailure      = "failure"
	AttributeCICDPipelineTaskRunStatusCancellation = "cancellation"
	AttributeCICDPipelineTaskRunStatusError        = "error"
	AttributeCICDPipelineTaskRunStatusSkip         = "skip"

	// The following attributes are not part of the semantic conventions yet.
	AttributeCICDPipelineRunSenderLogin     = "cicd.pipeline.run.sender.login"      // GitHub's Run Sender Login
	AttributeCICDPipelineTaskRunSenderLogin = "cicd.pipeline.task.run.sender.login" // GitHub's Task Sender Login
	AttributeVCSVendorName                  = "vcs.vendor.name"                     // GitHub
	AttributeVCSRepositoryOwner             = "vcs.repository.owner"                // GitHub's Owner Login
	AttributeCICDPipelineFilePath           = "cicd.pipeline.file.path"             // GitHub's Path in workflow_run

	AttributeGitHubAppInstallationID  = "github.app.installation.id"  // GitHub's Installation ID
	AttributeGitHubWorkflowRunAttempt = "github.workflow.run.attempt" // GitHub's Run Attempt

	// TODO: Evaluate whether or not these should be added. Always iffy on adding specific usernames and emails.
	AttributeVCSRefHeadRevisionAuthorName  = "vcs.ref.head.revision.author.name"  // GitHub's Head Revision Author Name
	AttributeVCSRefHeadRevisionAuthorEmail = "vcs.ref.head.revision.author.email" // GitHub's Head Revision Author Email

	// The following attributes are exclusive to GitHub but not listed under
	// Vendor Extensions within Semantic Conventions yet.
	AttributeGitHubWorkflowTriggerActorUsername = "github.workflow.trigger.actor.username" // GitHub's Triggering Actor Username

	// github.reference.workflow acts as a template attribute where it'll be
	// joined with a `name` and a `version` value. There is an unknown amount of
	// reference workflows that are sent as a list of string by GitHub making it
	// necessary to leverage template attributes. One key thing to note is the
	// length of the names. Evaluate if this causes issues.
	// eg. github.reference.workflow.my-great-workflow.path
	// eg. github.reference.workflow.my-great-workflow.version
	// eg. github.reference.workflow.my-great-workflow.revision
	AttributeGitHubReferenceWorkflow = "github.reference.workflow"
)

// getWorkflowAttrs returns a pcommon.Map of attributes for the Workflow Run
// GitHub event type and an error if one occurs. The attributes are associated
// with the originally provided resource.
func (gtr *githubTracesReceiver) getWorkflowAttrs(resource pcommon.Resource, e *github.WorkflowRunEvent) error {
	attrs := resource.Attributes()
	var err error

	svc, err := gtr.getServiceName(e.GetRepo().CustomProperties["service_name"], e.GetRepo().GetName())
	if err != nil {
		err = errors.New("failed to get service.name")
	}

	attrs.PutStr(semconv.AttributeServiceName, svc)

	// VCS Attributes
	attrs.PutStr(AttributeVCSRepositoryName, e.GetRepo().GetName())
	attrs.PutStr(AttributeVCSVendorName, "github")
	attrs.PutStr(AttributeVCSRefHead, e.GetWorkflowRun().GetHeadBranch())
	attrs.PutStr(AttributeVCSRefHeadType, AttributeVCSRefHeadTypeBranch)
	attrs.PutStr(AttributeVCSRefHeadRevision, e.GetWorkflowRun().GetHeadSHA())
	attrs.PutStr(AttributeVCSRefHeadRevisionAuthorName, e.GetWorkflowRun().GetHeadCommit().GetCommitter().GetName())
	attrs.PutStr(AttributeVCSRefHeadRevisionAuthorEmail, e.GetWorkflowRun().GetHeadCommit().GetCommitter().GetEmail())

	// CICD Attributes
	attrs.PutStr(semconv.AttributeCicdPipelineName, e.GetWorkflowRun().GetName())
	attrs.PutStr(AttributeCICDPipelineRunSenderLogin, e.GetSender().GetLogin())
	attrs.PutStr(AttributeCICDPipelineRunURLFull, e.GetWorkflowRun().GetHTMLURL())
	attrs.PutInt(semconv.AttributeCicdPipelineRunID, e.GetWorkflowRun().GetID()) // TODO: GitHub events have runIDs, but not available in the SDK??
	switch status := e.GetWorkflowRun().GetConclusion(); status {
	case "success":
		attrs.PutStr(AttributeCICDPipelineRunStatus, AttributeCICDPipelineRunStatusSuccess)
	case "failure":
		attrs.PutStr(AttributeCICDPipelineRunStatus, AttributeCICDPipelineRunStatusFailure)
	case "skipped":
		attrs.PutStr(AttributeCICDPipelineRunStatus, AttributeCICDPipelineRunStatusSkip)
	case "cancelled":
		attrs.PutStr(AttributeCICDPipelineRunStatus, AttributeCICDPipelineRunStatusCancellation)
	// Default sets to whatever is provided by the event. GitHub provides the following additional values:
	// neutral, timed_out, action_required, stale, startup_failure, and null.
	default:
		attrs.PutStr(AttributeCICDPipelineRunStatus, status)
	}

	// TODO: Add function and put previous	run_attempt_url
	// if e.GetWorkflowRun().GetPreviousAttemptURL() != "" {
	// 	htmlURL := transformGitHubAPIURL(e.GetWorkflowRun().GetPreviousAttemptURL())
	// 	attrs.PutStr("cicd.pipeline.run.previous_attempt_url", htmlURL)
	// }

	// TODO: This attribute is important for determining what shared workflows
	// exist, and what versions (or shas). Previously, it would all get
	// appended. But this should be a slice of values instead.
	if len(e.GetWorkflowRun().ReferencedWorkflows) > 0 {
		var referencedWorkflows []string
		for _, w := range e.GetWorkflowRun().ReferencedWorkflows {
			referencedWorkflows = append(referencedWorkflows, w.GetPath())
			referencedWorkflows = append(referencedWorkflows, w.GetSHA())
			referencedWorkflows = append(referencedWorkflows, w.GetRef())
		}
		attrs.PutStr("cicd.pipeline.run.referenced_workflows", strings.Join(referencedWorkflows, ";"))
	}

	// TODO: Convert this to the VCS Change Request.
	// if len(e.GetWorkflowRun().PullRequests) > 0 {
	// 	var prUrls []string
	// 	for _, pr := range e.GetWorkflowRun().PullRequests {
	// 		prUrls = append(prUrls, convertPRURL(pr.GetURL()))
	// 	}
	// 	attrs.PutStr("vcs.change.url", strings.Join(prUrls, ";"))
	// }

	return err
}

// splitRefWorkflow takes in the workl
func splitRefWorkflow(workflow string) (name string, version string, err error) {
	return "", "", nil
}

// getServiceName returns a generated service.name resource attribute derived
// from 1) the service_name defined in the webhook configuration 2) a
// service.name value set in the custom_properties section of a GitHub event, or
// 3) the repository name. The value returned in those cases will always be a
// formatted string; where the string will be lowercase and underscores will be
// replaced by hyphens. If none of these are set, it returns "unknown_service"
// and an error.
// func (gtr *githubTracesReceiver) getServiceName(customProps map[string]interface{}, repoName string) (string, error) {
func (gtr *githubTracesReceiver) getServiceName(customProps any, repoName string) (string, error) {
	switch {
	case gtr.cfg.WebHook.ServiceName != "":

		return formatted, nil
	// customProps would be an index map[string]interface{} passed in but should
	// only be non-nil if the index of `service_name` exists
	case customProps != nil:
		formatted := formatString(customProps.(string))
		return formatted, nil
	case repoName != "":
		formatted := formatString(repoName)
		return formatted, nil
	default:
		// This should never happen, but in the event it does, unknown_service
		// and a error will be returned to abide by semantic conventions.
		return "unknown_service", errors.New("unable to generate service.name resource attribute")
	}
}

// formatString formats a string to lowercase and replaces underscores with
// hyphens.
func formatString(input string) string {
	return strings.ToLower(strings.ReplaceAll(input, "_", "-"))
}
