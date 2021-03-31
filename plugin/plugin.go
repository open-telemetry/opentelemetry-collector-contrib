// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package plugin

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	yaml "gopkg.in/yaml.v2"

	"github.com/open-telemetry/opentelemetry-log-collection/errors"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
)

// Plugin is the rendered result of a plugin template.
type Plugin struct {
	Definition `yaml:",inline"`
	ID         string `json:"id" yaml:"id"`
	Template   *template.Template
}

// Definition contains metadata for rendering the plugin
type Definition struct {
	Version            string      `json:"version"     yaml:"version"`
	Title              string      `json:"title"       yaml:"title"`
	Description        string      `json:"description" yaml:"description"`
	Parameters         []Parameter `json:"parameters"  yaml:"parameters"`
	MinStanzaVersion   string      `json:"minStanzaVersion" yaml:"min_stanza_version"`
	MaxStanzaVersion   string      `json:"maxStanzaVersion" yaml:"max_stanza_version"`
	SupportedPlatforms []string    `json:"supportedPlatforms" yaml:"supported_platforms"`
}

// NewBuilder creates a new, empty config that can build into an operator
func (p *Plugin) NewBuilder() operator.Builder {
	return &Config{
		Plugin: p,
	}
}

// Render will render a plugin's template with the given parameters
func (p *Plugin) Render(params map[string]interface{}) ([]byte, error) {
	if err := p.Validate(params); err != nil {
		return nil, err
	}

	var writer bytes.Buffer
	if err := p.Template.Execute(&writer, params); err != nil {
		return nil, errors.NewError(
			"failed to render template for plugin",
			"ensure that all parameters are valid for the plugin",
			"plugin_type", p.ID,
			"error_message", err.Error(),
		)
	}

	return writer.Bytes(), nil
}

var versionRegex = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// Validate checks the provided params against the parameter definitions to ensure they are valid
func (p *Plugin) Validate(params map[string]interface{}) error {
	if p.Version != "" && !versionRegex.MatchString(p.Version) {
		return errors.NewError("invalid plugin version", "", "plugin_type", p.ID)
	}

	if p.MaxStanzaVersion != "" && !versionRegex.MatchString(p.MaxStanzaVersion) {
		return errors.NewError("invalid max stanza version", "", "plugin_type", p.ID)
	}

	if p.MinStanzaVersion != "" && !versionRegex.MatchString(p.MinStanzaVersion) {
		return errors.NewError("invalid min stanza version", "", "plugin_type", p.ID)
	}

	for _, param := range p.Parameters {
		value, ok := params[param.Name]
		if !ok && !param.Required {
			continue
		}

		if !ok && param.Required {
			return errors.NewError(
				"missing required parameter for plugin",
				"ensure that the parameter is defined for the plugin",
				"plugin_type", p.ID,
				"plugin_parameter", param.Name,
			)
		}

		if err := param.validateValue(value); err != nil {
			return errors.NewError(
				"plugin parameter failed validation",
				"review the underlying error message for details",
				"plugin_type", p.ID,
				"plugin_parameter", param.Name,
				"error_message", err.Error(),
			)
		}
	}

	return nil
}

// UnmarshalText unmarshals a plugin from a text file
func (p *Plugin) UnmarshalText(text []byte) error {
	metadataBytes, templateBytes, err := splitPluginFile(text)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(metadataBytes, p); err != nil {
		return err
	}

	p.Template, err = template.New(p.Title).
		Funcs(pluginFuncs()).
		Parse(string(templateBytes))
	return err
}

func splitPluginFile(text []byte) (metadata, template []byte, err error) {
	// Split the file into the metadata and the template by finding the pipeline,
	// then navigating backwards until we find a non-commented, non-empty line
	var metadataBuf bytes.Buffer
	var templateBuf bytes.Buffer

	lines := bytes.Split(text, []byte("\n"))
	if len(lines) != 0 && len(lines[len(lines)-1]) == 0 {
		// Delete empty trailing line
		lines = lines[:len(lines)-1]
	}

	// Find the index of the pipeline line
	pipelineRegex := regexp.MustCompile(`^pipeline:`)
	pipelineIndex := -1
	for i, line := range lines {
		if pipelineRegex.Match(line) {
			pipelineIndex = i
			break
		}
	}

	if pipelineIndex == -1 {
		return nil, nil, errors.NewError(
			"missing the pipeline block in plugin template",
			"ensure that the plugin file contains a pipeline",
		)
	}

	// Iterate backwards from the pipeline start to find the first non-commented, non-empty line
	emptyRegexp := regexp.MustCompile(`^\s*$`)
	commentedRegexp := regexp.MustCompile(`^\s*#`)
	templateStartIndex := pipelineIndex
	for i := templateStartIndex - 1; i >= 0; i-- {
		line := lines[i]
		if emptyRegexp.Match(line) || commentedRegexp.Match(line) {
			templateStartIndex = i
			continue
		}
		break
	}

	for _, line := range lines[:templateStartIndex] {
		metadataBuf.Write(line)
		metadataBuf.WriteByte('\n')
	}

	for _, line := range lines[templateStartIndex:] {
		templateBuf.Write(line)
		templateBuf.WriteByte('\n')
	}

	return metadataBuf.Bytes(), templateBuf.Bytes(), nil
}

// NewPluginFromFile builds a new plugin from a file
func NewPluginFromFile(path string) (*Plugin, error) {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read plugin file: %s", err)
	}

	pluginID := strings.TrimSuffix(filepath.Base(path), ".yaml")
	return NewPlugin(pluginID, contents)
}

// NewPlugin builds a new plugin from an ID and file contents
func NewPlugin(pluginID string, contents []byte) (*Plugin, error) {
	p := &Plugin{}
	if err := p.UnmarshalText(contents); err != nil {
		return nil, fmt.Errorf("could not unmarshal plugin file: %s", err)
	}
	p.ID = pluginID

	// Validate the parameter definitions
	for _, param := range p.Parameters {
		if err := param.validateDefinition(); err != nil {
			return nil, errors.NewError(
				"invalid parameter found in plugin",
				"ensure that all parameters are valid for the plugin",
				"plugin_type", p.ID,
				"plugin_parameter", param.Name,
				"error_message", err.Error(),
			)
		}
	}

	return p, nil
}

// RegisterPlugins adds every plugin in a directory to the global plugin registry
func RegisterPlugins(pluginDir string, registry *operator.Registry) []error {
	errs := []error{}
	glob := filepath.Join(pluginDir, "*.yaml")
	filePaths, err := filepath.Glob(glob)
	if err != nil {
		errs = append(errs, errors.NewError(
			"failed to find plugins with glob pattern",
			"ensure that the plugin directory and file pattern are valid",
			"glob_pattern", glob,
		))
		return errs
	}

	for _, path := range filePaths {
		plugin, err := NewPluginFromFile(path)
		if err != nil {
			errs = append(errs, errors.Wrap(err, "parse plugin file").WithDetails("path", path))
			continue
		}
		registry.RegisterPlugin(plugin.ID, plugin.NewBuilder)
	}

	return errs
}

// pluginFuncs returns a map of custom plugin functions used for templating.
func pluginFuncs() template.FuncMap {
	funcs := make(map[string]interface{})
	funcs["default"] = defaultPluginFunc
	return funcs
}

// defaultPluginFunc is a plugin function that returns a default value if the supplied value is nil.
func defaultPluginFunc(def interface{}, val interface{}) interface{} {
	if val == nil {
		return def
	}
	return val
}
