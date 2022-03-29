package processscraper

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
	includePidFilter            []int32
	excludePidFilter            []int32
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
func (p *processFilter) includeOwner (owner string) bool {
	return includeExcludeMatch(p.includeOwnerFilter, p.excludeOwnerFilter, owner)
}

// includePid return a boolean value indicating if the pid matches the filter.
func (p *processFilter) includePid(pid int32) bool {
	return (len(p.includePidFilter) == 0 || findInt(p.includePidFilter, pid)) &&
		(len(p.excludePidFilter) == 0 || !findInt(p.excludePidFilter, pid))
}


// findInt searches an int slice for a specific integeter.  If the
// int is found in the slice return true.  Otherwise return false
func findInt (intSlice []int32, intMatch int32) bool{
	for _, val := range intSlice {
		if intMatch == val {
			return true
		}
	}
	return false
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

	filter.excludeExecutableNameFilter, err = filterhelper.NewExcludeFilterHelper(filterConfig.IncludeExecutablePaths.ExecutablePaths, &filterConfig.IncludeExecutablePaths.Config, "executable_path")
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

	filter.includePidFilter = filterConfig.IncludePids.Pids
	filter.excludePidFilter = filterConfig.ExcludePids.Pids

	return &filter, err
}

// includeExcludeMatch checks the include and exclude filter
func includeExcludeMatch(include filterset.FilterSet, exclude filterset.FilterSet, matchString string) bool {
	return (include == nil ||  include.Matches(matchString)) &&
		(exclude == nil || !exclude.Matches(matchString))
}
