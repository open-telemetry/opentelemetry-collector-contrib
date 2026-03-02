// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver"

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/go-github/v83/github"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
)

// model.go contains custom attributes that complement the standardized attributes
// from OpenTelemetry semantic conventions v1.37.0. While many VCS and CICD attributes
// are now standardized, these custom attributes provide GitHub-specific functionality
// not yet covered by the standard semantic conventions.
const (
	// Note: Many VCS attributes are now standardized in semantic conventions v1.37.0
	// and available through the generated metadata package.

	// vcs.repository.name
	AttributeVCSRepositoryName = "vcs.repository.name"

	// vcs.ref.head.name (used in trace generation)
	AttributeVCSRefHead = "vcs.ref.head"

	// vcs.ref.head.revision
	AttributeVCSRefHeadRevision = "vcs.ref.head.revision"

	// vcs.ref.head.type with enum values of branch or tag.
	// Note: This is now standardized in semantic conventions v1.37.0
	AttributeVCSRefHeadType       = "vcs.ref.head.type"
	AttributeVCSRefHeadTypeBranch = "branch"
	AttributeVCSRefHeadTypeTag    = "tag"

	// The following CICD attributes are not yet standardized in semantic conventions v1.37.0.
	// They provide GitHub-specific functionality and may be subject to change.

	AttributeCICDPipelineRunURLFull = "cicd.pipeline.run.url.full" // equivalent to GitHub's `html_url`

	// CICD pipeline and task run status attributes for GitHub workflow integration
	AttributeCICDPipelineRunStatus             = "cicd.pipeline.run.status" // equivalent to GitHub's `conclusion`
	AttributeCICDPipelineRunStatusSuccess      = "success"
	AttributeCICDPipelineRunStatusFailure      = "failure"
	AttributeCICDPipelineRunStatusCancellation = "cancellation"

	AttributeCICDPipelineRunStatusSkip = "skip"

	AttributeCICDPipelineTaskRunStatus             = "cicd.pipeline.run.task.status" // equivalent to GitHub's `conclusion`
	AttributeCICDPipelineTaskRunStatusSuccess      = "success"
	AttributeCICDPipelineTaskRunStatusFailure      = "failure"
	AttributeCICDPipelineTaskRunStatusCancellation = "cancellation"
	AttributeCICDPipelineTaskRunStatusSkip         = "skip"

	// The following GitHub-specific attributes are not part of semantic conventions v1.37.0.
	AttributeCICDPipelineRunSenderLogin     = "cicd.pipeline.run.sender.login"      // GitHub's Run Sender Login
	AttributeCICDPipelineTaskRunSenderLogin = "cicd.pipeline.task.run.sender.login" // GitHub's Task Sender Login

	AttributeCICDPipelinePreviousAttemptURLFull = "cicd.pipeline.run.previous_attempt.url.full"
	AttributeCICDPipelineWorkerID               = "cicd.pipeline.worker.id"          // GitHub's Runner ID
	AttributeCICDPipelineWorkerGroupID          = "cicd.pipeline.worker.group.id"    // GitHub's Runner Group ID
	AttributeCICDPipelineWorkerName             = "cicd.pipeline.worker.name"        // GitHub's Runner Name
	AttributeCICDPipelineWorkerGroupName        = "cicd.pipeline.worker.group.name"  // GitHub's Runner Group Name
	AttributeCICDPipelineWorkerNodeID           = "cicd.pipeline.worker.node.id"     // GitHub's Runner Node ID
	AttributeCICDPipelineWorkerLabels           = "cicd.pipeline.worker.labels"      // GitHub's Runner Labels
	AttributeCICDPipelineRunQueueDuration       = "cicd.pipeline.run.queue.duration" // GitHub's Queue Duration

	// The following attributes are exclusive to GitHub but not listed under
	// vendor extensions within semantic conventions v1.37.0.
	AttributeGitHubRepositoryCustomProperty = "github.repository.custom_properties" // GitHub's Repository Custom Properties (used in custom property processing)

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

	attrs.PutStr(string(conventions.ServiceNameKey), svc)

	// Add all custom properties from the repository as resource attributes
	addCustomPropertiesToAttrs(attrs, e.GetRepo().CustomProperties)

	// VCS Attributes
	attrs.PutStr(AttributeVCSRepositoryName, e.GetRepo().GetName())
	attrs.PutStr("vcs.provider.name", "github")
	attrs.PutStr(AttributeVCSRefHead, e.GetWorkflowRun().GetHeadBranch())
	attrs.PutStr("vcs.ref.type", "branch")
	attrs.PutStr(AttributeVCSRefHeadRevision, e.GetWorkflowRun().GetHeadSHA())
	attrs.PutStr(AttributeVCSRefHeadRevisionAuthorName, e.GetWorkflowRun().GetHeadCommit().GetCommitter().GetName())
	attrs.PutStr(AttributeVCSRefHeadRevisionAuthorEmail, e.GetWorkflowRun().GetHeadCommit().GetCommitter().GetEmail())

	// CICD Attributes
	attrs.PutStr(string(conventions.CICDPipelineNameKey), e.GetWorkflowRun().GetName())
	attrs.PutStr(AttributeCICDPipelineRunSenderLogin, e.GetSender().GetLogin())
	attrs.PutStr(AttributeCICDPipelineRunURLFull, e.GetWorkflowRun().GetHTMLURL())
	attrs.PutInt(string(conventions.CICDPipelineRunIDKey), e.GetWorkflowRun().GetID())
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

	attrs.PutStr(string(conventions.ServiceNameKey), svc)

	// Add all custom properties from the repository as resource attributes
	addCustomPropertiesToAttrs(attrs, e.GetRepo().CustomProperties)

	// VCS Attributes
	attrs.PutStr(AttributeVCSRepositoryName, e.GetRepo().GetName())
	attrs.PutStr("vcs.provider.name", "github")
	attrs.PutStr(AttributeVCSRefHead, e.GetWorkflowJob().GetHeadBranch())
	attrs.PutStr("vcs.ref.type", "branch")
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
	attrs.PutStr(string(conventions.CICDPipelineNameKey), e.GetWorkflowJob().GetName())
	attrs.PutStr(AttributeCICDPipelineTaskRunSenderLogin, e.GetSender().GetLogin())
	attrs.PutStr(string(conventions.CICDPipelineTaskRunURLFullKey), e.GetWorkflowJob().GetHTMLURL())
	attrs.PutInt(string(conventions.CICDPipelineTaskRunIDKey), e.GetWorkflowJob().GetID())
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
		// and an error will be returned to abide by semantic conventions.
		return "unknown_service", errors.New("unable to generate service.name resource attribute")
	}
}

// addCustomPropertiesToAttrs adds all custom properties from the repository as resource attributes
// with the prefix AttributeGitHubCustomProperty. Keys are converted to snake_case to follow
// resource attribute naming convention.
func addCustomPropertiesToAttrs(attrs pcommon.Map, customProps map[string]any) {
	if len(customProps) == 0 {
		return
	}

	for key, value := range customProps {
		// Skip service_name as it's already handled separately
		if key == "service_name" {
			continue
		}

		// Convert key to snake_case
		snakeCaseKey := toSnakeCase(key)

		// Use dot notation for keys, following resource attribute naming convention
		attrKey := fmt.Sprintf("%s.%s", AttributeGitHubRepositoryCustomProperty, snakeCaseKey)

		// Handle different value types
		switch v := value.(type) {
		case string:
			attrs.PutStr(attrKey, v)
		case int:
			attrs.PutInt(attrKey, int64(v))
		case int64:
			attrs.PutInt(attrKey, v)
		case float64:
			attrs.PutDouble(attrKey, v)
		case bool:
			attrs.PutBool(attrKey, v)
		default:
			// For any other types, convert to string
			attrs.PutStr(attrKey, fmt.Sprintf("%v", v))
		}
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

// toSnakeCase converts a string to snake_case format.
// It handles all GitHub supported characters for custom property names: a-z, A-Z, 0-9, _, -, $, #.
// This function ensures that the resulting string follows snake_case convention.
func toSnakeCase(s string) string {
	// Replace hyphens, spaces, and dots with underscores
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, ".", "_")

	// Replace special characters with underscores
	s = strings.ReplaceAll(s, "$", "_dollar_")
	s = strings.ReplaceAll(s, "#", "_hash_")

	// Handle camelCase and PascalCase
	var result strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			// If current char is uppercase and previous char is lowercase or a digit,
			// or if current char is uppercase and next char is lowercase,
			// add an underscore before the current char
			prevIsLower := i > 0 && (unicode.IsLower(rune(s[i-1])) || unicode.IsDigit(rune(s[i-1])))
			nextIsLower := i < len(s)-1 && unicode.IsLower(rune(s[i+1]))
			if prevIsLower || nextIsLower {
				result.WriteRune('_')
			}
		}
		result.WriteRune(unicode.ToLower(r))
	}

	// Replace multiple consecutive underscores with a single one
	output := result.String()
	for strings.Contains(output, "__") {
		output = strings.ReplaceAll(output, "__", "_")
	}

	return output
}
