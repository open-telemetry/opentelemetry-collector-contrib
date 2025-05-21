// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	"errors"
	"strings"

	"github.com/google/go-github/v72/github"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
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
	AttributeCICDPipelineTaskRunStatusSkip         = "skip"

	// The following attributes are not part of the semantic conventions yet.
	AttributeCICDPipelineRunSenderLogin         = "cicd.pipeline.run.sender.login"      // GitHub's Run Sender Login
	AttributeCICDPipelineTaskRunSenderLogin     = "cicd.pipeline.task.run.sender.login" // GitHub's Task Sender Login
	AttributeCICDPipelineFilePath               = "cicd.pipeline.file.path"             // GitHub's Path in workflow_run
	AttributeCICDPipelinePreviousAttemptURLFull = "cicd.pipeline.run.previous_attempt.url.full"
	AttributeCICDPipelineWorkerID               = "cicd.pipeline.worker.id"          // GitHub's Runner ID
	AttributeCICDPipelineWorkerGroupID          = "cicd.pipeline.worker.group.id"    // GitHub's Runner Group ID
	AttributeCICDPipelineWorkerName             = "cicd.pipeline.worker.name"        // GitHub's Runner Name
	AttributeCICDPipelineWorkerGroupName        = "cicd.pipeline.worker.group.name"  // GitHub's Runner Group Name
	AttributeCICDPipelineWorkerNodeID           = "cicd.pipeline.worker.node.id"     // GitHub's Runner Node ID
	AttributeCICDPipelineWorkerLabels           = "cicd.pipeline.worker.labels"      // GitHub's Runner Labels
	AttributeCICDPipelineRunQueueDuration       = "cicd.pipeline.run.queue.duration" // GitHub's Queue Duration

	// The following attributes are exclusive to GitHub but not listed under
	// Vendor Extensions within Semantic Conventions yet.
	AttributeGitHubAppInstallationID            = "github.app.installation.id"             // GitHub's Installation ID
	AttributeGitHubWorkflowRunAttempt           = "github.workflow.run.attempt"            // GitHub's Run Attempt
	AttributeGitHubWorkflowTriggerActorUsername = "github.workflow.trigger.actor.username" // GitHub's Triggering Actor Username

	// github.reference.workflow acts as a template attribute where it'll be
	// joined with a `name` and a `version` value. There is an unknown amount of
	// reference workflows that are sent as a list of strings by GitHub making
	// it necessary to leverage template attributes. One key thing to note is
	// the length of the names. Evaluate if this causes issues.
	// WARNING: Extremely long workflow file names could create extremely long
	// attribute keys which could lead to unknown issues in the backend and
	// create additional memory usage overhead when processing data (though
	// unlikely).
	// TODO: Evaluate if there is a need to truncate long workflow files names.
	// eg. github.reference.workflow.my-great-workflow.path
	// eg. github.reference.workflow.my-great-workflow.version
	// eg. github.reference.workflow.my-great-workflow.revision
	AttributeGitHubReferenceWorkflow = "github.reference.workflow"

	// SECURITY: This information will always exist on the repository, but may
	// be considered private if the repository is set to private. Care should be
	// taken in the data pipeline for sanitizing sensitive user information if
	// the user deems it as such.
	AttributeVCSRefHeadRevisionAuthorName  = "vcs.ref.head.revision.author.name"  // GitHub's Head Revision Author Name
	AttributeVCSRefHeadRevisionAuthorEmail = "vcs.ref.head.revision.author.email" // GitHub's Head Revision Author Email
	AttributeVCSRepositoryOwner            = "vcs.repository.owner"               // GitHub's Owner Login
	AttributeVCSVendorName                 = "vcs.vendor.name"                    // GitHub
)

// getWorkflowRunAttrs returns a pcommon.Map of attributes for the Workflow Run
// GitHub event type and an error if one occurs. The attributes are associated
// with the originally provided resource.
func (gtr *githubTracesReceiver) getWorkflowRunAttrs(resource pcommon.Resource, e *github.WorkflowRunEvent) error {
	attrs := resource.Attributes()
	var err error

	svc, err := gtr.getServiceName(e.GetRepo().CustomProperties["service_name"], e.GetRepo().GetName())
	if err != nil {
		err = errors.New("failed to get service.name")
	}

	attrs.PutStr(string(semconv.ServiceNameKey), svc)

	// VCS Attributes
	attrs.PutStr(AttributeVCSRepositoryName, e.GetRepo().GetName())
	attrs.PutStr(AttributeVCSVendorName, "github")
	attrs.PutStr(AttributeVCSRefHead, e.GetWorkflowRun().GetHeadBranch())
	attrs.PutStr(AttributeVCSRefHeadType, AttributeVCSRefHeadTypeBranch)
	attrs.PutStr(AttributeVCSRefHeadRevision, e.GetWorkflowRun().GetHeadSHA())
	attrs.PutStr(AttributeVCSRefHeadRevisionAuthorName, e.GetWorkflowRun().GetHeadCommit().GetCommitter().GetName())
	attrs.PutStr(AttributeVCSRefHeadRevisionAuthorEmail, e.GetWorkflowRun().GetHeadCommit().GetCommitter().GetEmail())

	// CICD Attributes
	attrs.PutStr(string(semconv.CICDPipelineNameKey), e.GetWorkflowRun().GetName())
	attrs.PutStr(AttributeCICDPipelineRunSenderLogin, e.GetSender().GetLogin())
	attrs.PutStr(AttributeCICDPipelineRunURLFull, e.GetWorkflowRun().GetHTMLURL())
	attrs.PutInt(string(semconv.CICDPipelineRunIDKey), e.GetWorkflowRun().GetID())
	switch status := strings.ToLower(e.GetWorkflowRun().GetConclusion()); status {
	case "success":
		attrs.PutStr(AttributeCICDPipelineRunStatus, AttributeCICDPipelineRunStatusSuccess)
	case "failure":
		attrs.PutStr(AttributeCICDPipelineRunStatus, AttributeCICDPipelineRunStatusFailure)
	case "skipped":
		attrs.PutStr(AttributeCICDPipelineRunStatus, AttributeCICDPipelineRunStatusSkip)
	case "cancelled":
		attrs.PutStr(AttributeCICDPipelineRunStatus, AttributeCICDPipelineRunStatusCancellation)
	// Default sets to whatever is provided by the event. GitHub provides the
	// following additional values: neutral, timed_out, action_required, stale,
	// startup_failure, and null.
	default:
		attrs.PutStr(AttributeCICDPipelineRunStatus, status)
	}

	if e.GetWorkflowRun().GetPreviousAttemptURL() != "" {
		htmlURL := replaceAPIURL(e.GetWorkflowRun().GetPreviousAttemptURL())
		attrs.PutStr(AttributeCICDPipelinePreviousAttemptURLFull, htmlURL)
	}

	// Determine if there are any referenced (shared) workflows listed in the
	// Workflow Run event and generate the templated attributes for them.
	if len(e.GetWorkflowRun().ReferencedWorkflows) > 0 {
		for _, w := range e.GetWorkflowRun().ReferencedWorkflows {
			var name string
			name, err = splitRefWorkflowPath(w.GetPath())
			if err != nil {
				return err
			}

			template := AttributeGitHubReferenceWorkflow + "." + name
			pathAttr := template + ".path"
			revAttr := template + ".revision"
			versionAttr := template + ".version"

			attrs.PutStr(pathAttr, w.GetPath())
			attrs.PutStr(revAttr, w.GetSHA())
			attrs.PutStr(versionAttr, w.GetRef())
		}
	}

	return err
}

// getWorkflowJobAttrs returns a pcommon.Map of attributes for the Workflow Job
// GitHub event type and an error if one occurs. The attributes are associated
// with the originally provided resource.
func (gtr *githubTracesReceiver) getWorkflowJobAttrs(resource pcommon.Resource, e *github.WorkflowJobEvent) error {
	attrs := resource.Attributes()
	var err error

	svc, err := gtr.getServiceName(e.GetRepo().CustomProperties["service_name"], e.GetRepo().GetName())
	if err != nil {
		err = errors.New("failed to get service.name")
	}

	attrs.PutStr(string(semconv.ServiceNameKey), svc)

	// VCS Attributes
	attrs.PutStr(AttributeVCSRepositoryName, e.GetRepo().GetName())
	attrs.PutStr(AttributeVCSVendorName, "github")
	attrs.PutStr(AttributeVCSRefHead, e.GetWorkflowJob().GetHeadBranch())
	attrs.PutStr(AttributeVCSRefHeadType, AttributeVCSRefHeadTypeBranch)
	attrs.PutStr(AttributeVCSRefHeadRevision, e.GetWorkflowJob().GetHeadSHA())

	// CICD Worker (GitHub Runner) Attributes
	attrs.PutInt(AttributeCICDPipelineWorkerID, e.GetWorkflowJob().GetRunnerID())
	attrs.PutInt(AttributeCICDPipelineWorkerGroupID, e.GetWorkflowJob().GetRunnerGroupID())
	attrs.PutStr(AttributeCICDPipelineWorkerName, e.GetWorkflowJob().GetRunnerName())
	attrs.PutStr(AttributeCICDPipelineWorkerGroupName, e.GetWorkflowJob().GetRunnerGroupName())
	attrs.PutStr(AttributeCICDPipelineWorkerNodeID, e.GetWorkflowJob().GetNodeID())

	if len(e.GetWorkflowJob().Labels) > 0 {
		labels := attrs.PutEmptySlice(AttributeCICDPipelineWorkerLabels)
		labels.EnsureCapacity(len(e.GetWorkflowJob().Labels))
		for _, label := range e.GetWorkflowJob().Labels {
			l := strings.ToLower(label)
			labels.AppendEmpty().SetStr(l)
		}
	}

	// CICD Attributes
	attrs.PutStr(string(semconv.CICDPipelineNameKey), e.GetWorkflowJob().GetName())
	attrs.PutStr(AttributeCICDPipelineTaskRunSenderLogin, e.GetSender().GetLogin())
	attrs.PutStr(string(semconv.CICDPipelineTaskRunURLFullKey), e.GetWorkflowJob().GetHTMLURL())
	attrs.PutInt(string(semconv.CICDPipelineTaskRunIDKey), e.GetWorkflowJob().GetID())
	switch status := strings.ToLower(e.GetWorkflowJob().GetConclusion()); status {
	case "success":
		attrs.PutStr(AttributeCICDPipelineTaskRunStatus, AttributeCICDPipelineTaskRunStatusSuccess)
	case "failure":
		attrs.PutStr(AttributeCICDPipelineTaskRunStatus, AttributeCICDPipelineTaskRunStatusFailure)
	case "skipped":
		attrs.PutStr(AttributeCICDPipelineTaskRunStatus, AttributeCICDPipelineTaskRunStatusSkip)
	case "cancelled":
		attrs.PutStr(AttributeCICDPipelineTaskRunStatus, AttributeCICDPipelineTaskRunStatusCancellation)
	// Default sets to whatever is provided by the event. GitHub provides the
	// following additional values: neutral, timed_out, action_required, stale,
	// and null.
	default:
		attrs.PutStr(AttributeCICDPipelineRunStatus, status)
	}

	return err
}

// splitRefWorkflowPath splits the reference workflow path into just the file
// name normalized to lowercase without the file type.
func splitRefWorkflowPath(path string) (fileName string, err error) {
	parts := strings.Split(path, "@")
	if len(parts) != 2 {
		return "", errors.New("invalid reference workflow path")
	}

	parts = strings.Split(parts[0], "/")
	if len(parts) == 0 {
		return "", errors.New("invalid reference workflow path")
	}

	last := parts[len(parts)-1]
	parts = strings.Split(last, ".")
	if len(parts) == 0 {
		return "", errors.New("invalid reference workflow path")
	}

	return strings.ToLower(parts[0]), nil
}

// getServiceName returns a generated service.name resource attribute derived
// from 1) the service_name defined in the webhook configuration 2) a
// service.name value set in the custom_properties section of a GitHub event, or
// 3) the repository name. The value returned in those cases will always be a
// formatted string; where the string will be lowercase and underscores will be
// replaced by hyphens. If none of these are set, it returns "unknown_service"
// according to the semantic conventions for service.name and an error.
// https://opentelemetry.io/docs/specs/semconv/attributes-registry/service/#service-attributes
func (gtr *githubTracesReceiver) getServiceName(customProps any, repoName string) (string, error) {
	switch {
	case gtr.cfg.WebHook.ServiceName != "":
		formatted := formatString(gtr.cfg.WebHook.ServiceName)
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

// replaceAPIURL replaces a GitHub API URL with the HTML URL version.
func replaceAPIURL(apiURL string) (htmlURL string) {
	// TODO: Support enterpise server configuration with custom domain.
	return strings.Replace(apiURL, "api.github.com/repos", "github.com", 1)
}
