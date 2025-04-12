// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8slogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8slogreceiver"

import (
	"fmt"

	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

const (
	ModeDaemonSetStdout = "daemonset-stdout"
)

const (
	DefaultMode        = ModeDaemonSetStdout
	DefaultHostRoot    = "/host_root"
	DefaultNodeFromEnv = "KUBE_NODE_NAME"
)

// Config is the configuration of a k8slog receiver
type Config struct {
	Discovery SourceConfig  `mapstructure:"discovery"`
	Extract   ExtractConfig `mapstructure:"extract"`

	// TODO: refactor fileconsumer and add it's config of k8s implementation here.
}

// ExtractConfig allows specifying how to extract resource attributes from pod.
type ExtractConfig struct {
	// Metadata represents the list of metadata fields to extract from pod.
	// TODO: supported metadata fields and default values.
	Metadata []string `mapstructure:"metadata"`

	// Annotations represents the rules to extract from pod annotations.
	Annotations []FieldExtractConfig `mapstructure:"annotations"`

	// Labels represents the rules to extract from pod labels.
	Labels []FieldExtractConfig `mapstructure:"labels"`

	// Env represents the rules to extract from container environment variables.
	Env []FieldExtractConfig `mapstructure:"env"`
}

// FieldExtractConfig allows specifying an extraction rule to extract a resource attribute from pod (or namespace)
// annotations (or labels).
// This is a copy of the config from the k8sattributes processor.
type FieldExtractConfig struct {
	// TagName represents the name of the resource attribute that will be added to logs, metrics or spans.
	// When not specified, a default tag name will be used of the format:
	//   - k8s.pod.annotations.<annotation key>
	//   - k8s.pod.labels.<label key>
	//   - k8s.pod.env.<env key>
	//   - otel.env.<env key>
	// For example, if tag_name is not specified and the key is git_sha,
	// then the attribute name will be `k8s.pod.annotations.git_sha`.
	// When key_regex is present, tag_name supports back reference to both named capturing and positioned capturing.
	// For example, if your pod spec contains the following labels,
	//
	// app.kubernetes.io/component: mysql
	// app.kubernetes.io/version: 5.7.21
	//
	// and you'd like to add tags for all labels with prefix app.kubernetes.io/ and also trim the prefix,
	// then you can specify the following extraction rules:
	//
	// extract:
	//   labels:
	//     - tag_name: $$1
	//       key_regex: kubernetes.io/(.*)
	//
	// this will add the `component` and `version` tags to the spans or metrics.
	TagName string `mapstructure:"tag_name"`

	// Key represents the key (annotation, label or etc.) name. It uses exact match.
	Key string `mapstructure:"key"`
	// KeyRegex is a regular expression used to extract a Key that matches the regex.
	// Out of Key or KeyRegex, only one option is expected to be configured at a time.
	KeyRegex string `mapstructure:"key_regex"`

	// Regex is an optional field used to extract a sub-string from a complex field value.
	// The supplied regular expression must contain one named parameter with the string "value"
	// as the name. For example, if your pod spec contains the following annotation,
	//
	// kubernetes.io/change-cause: 2019-08-28T18:34:33Z APP_NAME=my-app GIT_SHA=58a1e39 CI_BUILD=4120
	//
	// and you'd like to extract the GIT_SHA and the CI_BUILD values as tags, then you must
	// specify the following two extraction rules:
	//
	// extract:
	//   annotations:
	//     - tag_name: git.sha
	//       key: kubernetes.io/change-cause
	//       regex: GIT_SHA=(?P<value>\w+)
	//     - tag_name: ci.build
	//       key: kubernetes.io/change-cause
	//       regex: JENKINS=(?P<value>[\w]+)
	//
	// this will add the `git.sha` and `ci.build` resource attributes.
	Regex string `mapstructure:"regex"`

	// From represents the source of the labels/annotations.
	// Env is always from container environment variables.
	// Supported values:
	// - pod (default): extract from pod labels/annotations.
	// May be supported in the future:
	// - namespace
	From string `mapstructure:"from"`
}

func (c Config) Validate() error {
	return c.Discovery.Validate()
}

// SourceConfig allows specifying how to discover containers to collect logs from.
type SourceConfig struct {
	// Mode represents the mode of the k8slog receiver.
	// Valid values are:
	// - "daemonset-stdout": (default) otel is deployed as a daemonset and collects logs from stdout of containers.
	//
	// Will be supported in the future:
	// - "daemonset-file": otel is deployed as a daemonset and collects logs from files inside containers.
	// - "sidecar": otel is deployed as a sidecar and collects logs from files.
	Mode string `mapstructure:"mode"`

	// NodeFromEnv represents the environment variable which contains the node name.
	NodeFromEnv string `mapstructure:"node_from_env"`

	// HostRoot represents the path which is used to mount the host's root filesystem.
	HostRoot string `mapstructure:"host_root"`

	// K8sAPI represents the configuration for the k8s API.
	K8sAPI k8sconfig.APIConfig `mapstructure:"k8s_api"`

	// RuntimeAPIs represents the configuration for the runtime APIs.
	RuntimeAPIs []RuntimeAPIConfig `mapstructure:"runtime_apis"`

	Filter []FilterConfig `mapstructure:"filter"`
}

func (c SourceConfig) Validate() error {
	var err error
	if c.Mode != ModeDaemonSetStdout {
		return fmt.Errorf("invalid mode %q", c.Mode)
	}
	if c.HostRoot == "" {
		err = multierr.Append(err, fmt.Errorf("host_root must be specified when mode is %q", c.Mode))
	}
	err = multierr.Append(err, c.K8sAPI.Validate())
	for _, r := range c.RuntimeAPIs {
		err = multierr.Append(err, r.Validate())
	}
	return err
}

// FilterConfig allows specifying how to filter containers to collect logs from.
// By default, all containers are collected from.
type FilterConfig struct {
	// Annotations represents the rules to filter containers based on pod annotations.
	Annotations []MapFilterConfig `mapstructure:"annotations"`

	// Labels represents the rules to filter containers based on pod labels.
	Labels []MapFilterConfig `mapstructure:"labels"`

	// Env represents the rules to filter containers based on pod environment variables.
	Env []MapFilterConfig `mapstructure:"env"`

	// Namespaces represents the rules to filter containers based on pod namespaces.
	Namespaces []ValueFilterConfig `mapstructure:"namespaces"`

	// Containers represents the rules to filter containers based on container names.
	Containers []ValueFilterConfig `mapstructure:"containers"`

	// Pods represents the rules to filter containers based on pod names.
	Pods []ValueFilterConfig `mapstructure:"pods"`
}

// ValueFilterConfig allows specifying a filter rule to filter containers based on string values,
// such as pod names, namespaces, container names or pod UIDs.
// If any of the values match, this rule is considered to match.
type ValueFilterConfig struct {
	// Op represents how to compare the value.
	// Valid values are:
	// - "equals": (default) the value must be equal to the specified value.
	// - "not-equals": the value must not be equal to the specified value.
	// - "matches": the value must match the specified regular expression.
	// - "not-matches": the value must not match the specified regular expression.
	Op string `mapstructure:"op"`

	// Value represents the value to compare against.
	Value string `mapstructure:"value"`
}

// MapFilterConfig allows specifying a filter rule to filter containers based on key value pairs,
// such as pod annotations, labels or environment variables.
// Only if all the keys match, this rule is considered to match.
type MapFilterConfig struct {
	// Op represents how to compare the values.
	// Valid values are:
	// - "equals": (default) the value must be equal to the specified value.
	// - "not-equals": the value must not be equal to the specified value.
	// - "exists": the value must exist.
	// - "not-exists": the value must not exist.
	// - "matches": the value must match the specified regular expression.
	// - "not-matches": the value must not match the specified regular expression.
	Op string `mapstructure:"op"`

	// Key represents the key to compare against.
	Key string `mapstructure:"key"`

	// Value represents the value to compare against.
	// If Op is "exists" or "not-exists", this field is ignored.
	// If any of the values match, this rule is considered to match.
	Value string `mapstructure:"value"`
}
