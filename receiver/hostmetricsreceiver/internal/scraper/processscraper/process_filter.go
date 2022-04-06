// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package processscraper // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/filterhelper"
)

type processFilter struct {
	includeExecutableNameFilter filterset.FilterSet
	excludeExecutableNameFilter filterset.FilterSet
	includeExecutablePathFilter filterset.FilterSet
	excludeExecutablePathFilter filterset.FilterSet
	includeCommandFilter        filterset.FilterSet
	excludeCommandFilter        filterset.FilterSet
	includeCommandLineFilter    filterset.FilterSet
	excludeCommandLineFilter    filterset.FilterSet
	includeOwnerFilter          filterset.FilterSet
	excludeOwnerFilter          filterset.FilterSet
	includePidFilter            map[int32]struct{}
	excludePidFilter            map[int32]struct{}
}

// includeExecutable return a boolean value indicating if executableName and executablePath matches the filter.
func (p *processFilter) includeExecutable(executableName string, executablePath string) bool {
	return includeExcludeMatch(p.includeExecutableNameFilter, p.excludeExecutableNameFilter, executableName) &&
		includeExcludeMatch(p.includeExecutablePathFilter, p.excludeExecutablePathFilter, executablePath)
}

// includeCommand return a boolean value indicating if command and commandLine matches the filter.
func (p *processFilter) includeCommand(command string, commandLine string) bool {
	return includeExcludeMatch(p.includeCommandFilter, p.excludeCommandFilter, command) &&
		includeExcludeMatch(p.includeCommandLineFilter, p.excludeCommandLineFilter, commandLine)
}

// includeOwner return a boolean value indicating if owner matches the filter.
func (p *processFilter) includeOwner(owner string) bool {
	return includeExcludeMatch(p.includeOwnerFilter, p.excludeOwnerFilter, owner)
}

// includePid return a boolean value indicating if the pid matches the filter.
func (p *processFilter) includePid(pid int32) bool {
	return (len(p.includePidFilter) == 0 || findInt(p.includePidFilter, pid)) &&
		(len(p.excludePidFilter) == 0 || !findInt(p.excludePidFilter, pid))
}

// findInt searches an int map for a specific integer.  If the
// int is found in the map return true.  Otherwise return false
func findInt(intMap map[int32]struct{}, intMatch int32) bool {
	_, ok := intMap[intMatch]
	return ok
}

// createFilter receives a filter config and createa a processFilter based on the config settings
func createFilter(filterConfig FilterConfig) (*processFilter, error) {
	var err error
	filter := processFilter{}

	filter.includeExecutableNameFilter, err = filterhelper.NewIncludeFilterHelper(filterConfig.IncludeExecutableNames.ExecutableNames, &filterConfig.IncludeExecutableNames.Config, "executable_name")
	if err != nil {
		return nil, err
	}

	filter.excludeExecutableNameFilter, err = filterhelper.NewExcludeFilterHelper(filterConfig.ExcludeExecutableNames.ExecutableNames, &filterConfig.ExcludeExecutableNames.Config, "executable_name")
	if err != nil {
		return nil, err
	}

	filter.includeExecutablePathFilter, err = filterhelper.NewIncludeFilterHelper(filterConfig.IncludeExecutablePaths.ExecutablePaths, &filterConfig.IncludeExecutablePaths.Config, "executable_path")
	if err != nil {
		return nil, err
	}

	filter.excludeExecutablePathFilter, err = filterhelper.NewExcludeFilterHelper(filterConfig.ExcludeExecutablePaths.ExecutablePaths, &filterConfig.ExcludeExecutablePaths.Config, "executable_path")
	if err != nil {
		return nil, err
	}

	filter.includeCommandFilter, err = filterhelper.NewIncludeFilterHelper(filterConfig.IncludeCommands.Commands, &filterConfig.IncludeCommands.Config, "commands")
	if err != nil {
		return nil, err
	}

	filter.excludeCommandFilter, err = filterhelper.NewExcludeFilterHelper(filterConfig.ExcludeCommands.Commands, &filterConfig.ExcludeCommands.Config, "commands")
	if err != nil {
		return nil, err
	}

	filter.includeCommandLineFilter, err = filterhelper.NewIncludeFilterHelper(filterConfig.IncludeCommandLines.CommandLines, &filterConfig.IncludeCommandLines.Config, "command_lines")
	if err != nil {
		return nil, err
	}

	filter.excludeCommandLineFilter, err = filterhelper.NewExcludeFilterHelper(filterConfig.ExcludeCommandLines.CommandLines, &filterConfig.ExcludeCommandLines.Config, "command_lines")
	if err != nil {
		return nil, err
	}

	filter.includeOwnerFilter, err = filterhelper.NewIncludeFilterHelper(filterConfig.IncludeOwners.Owners, &filterConfig.IncludeOwners.Config, "owners")
	if err != nil {
		return nil, err
	}

	filter.excludeOwnerFilter, err = filterhelper.NewExcludeFilterHelper(filterConfig.ExcludeOwners.Owners, &filterConfig.ExcludeOwners.Config, "owners")
	if err != nil {
		return nil, err
	}

	filter.includePidFilter = make(map[int32]struct{})
	// convert slice to map for quick lookup
	for _, val := range filterConfig.IncludePids {
		filter.includePidFilter[val] = struct{}{}
	}

	filter.excludePidFilter = make(map[int32]struct{})
	// convert slice to map for quick lookup
	for _, val := range filterConfig.ExcludePids {
		filter.excludePidFilter[val] = struct{}{}
	}

	return &filter, err
}

// includeExcludeMatch checks the include and exclude filter
func includeExcludeMatch(include filterset.FilterSet, exclude filterset.FilterSet, matchString string) bool {
	return (include == nil || include.Matches(matchString)) &&
		(exclude == nil || !exclude.Matches(matchString))
}
