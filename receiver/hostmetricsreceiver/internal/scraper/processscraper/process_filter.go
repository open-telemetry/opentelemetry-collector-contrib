package processscraper

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset/regexp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset/strict"
	"strings"
)

const RegExPrefix = "regex "

type processFilter struct {
	executableNameFilter filterset.FilterSet
	executablePathFilter filterset.FilterSet
	commandFilter        filterset.FilterSet
	commandLineFilter    filterset.FilterSet
	ownerFilter          filterset.FilterSet
	pidFilter            int32
}

// MatchesExecutable return a boolean value indicating if executableName and executablePath matches the filter.
func (p *processFilter) MatchesExecutable(executableName string, executablePath string) bool {
	return (p.executableNameFilter == nil || p.executableNameFilter.Matches(executableName)) &&
		(p.executablePathFilter == nil || p.executablePathFilter.Matches(executablePath))
}

// MatchesCommand return a boolean value indicating if command and commandLine matches the filter.
func (p *processFilter) MatchesCommand(command string, commandLine string) bool {
	return (p.commandFilter == nil || p.commandFilter.Matches(command)) &&
		(p.commandLineFilter == nil || p.commandLineFilter.Matches(commandLine))
}


// MatchOwner return a boolean value indicating if owner matches the filter.
func (p *processFilter) MatchOwner(owner string) bool {
	return p.ownerFilter == nil || p.ownerFilter.Matches(owner)
}

// MatchesPid return a boolean value indicating if the pid matches the filter.
func (p *processFilter) MatchesPid(pid int32) bool {
	return p.pidFilter == 0 || (p.pidFilter == pid)
}

// regexParse indicates if a filter string is a regex.  The return value is
// the filter string and a boolean indicating if the input string is a regex string.
func regexParse(regexp string) (string, bool) {
	// this is not a regular expression
	if !strings.HasPrefix(regexp, RegExPrefix) {
		return regexp, false
	}

	// trim regex indicator
	regexp = strings.TrimPrefix(regexp, RegExPrefix)

	//remove leading and trailing '"' character from a regex string
	if len(regexp) > 1 && regexp[0] == '"' &&
		regexp[len(regexp)-1] == '"' {
		regexp = regexp[1:]
		regexp = regexp[:len(regexp)-1]
	}
	return regexp, true
}

// createFilter receives a filter config and createa a processFilter based on the config settings
func createFilter(filterConfig FilterConfig) (processFilter, error) {
	var err error
	filter := processFilter{}

	if filterConfig.ExecutableName != "" {
		if executableNameFilter, isRegex := regexParse(filterConfig.ExecutableName); isRegex {
			filter.executableNameFilter, err = regexp.NewFilterSet([]string{executableNameFilter}, filterConfig.RegexpConfig)
		} else {
			filter.executableNameFilter = strict.NewFilterSet([]string{executableNameFilter})
		}
	}

	if filterConfig.ExecutablePath != "" {
		if executablePathFilter, isRegex := regexParse(filterConfig.ExecutablePath); isRegex {
			filter.executablePathFilter, err = regexp.NewFilterSet([]string{executablePathFilter}, filterConfig.RegexpConfig)
		} else {
			filter.executablePathFilter = strict.NewFilterSet([]string{executablePathFilter})
		}
	}

	if filterConfig.Command != "" {
		if commandFilter, isRegex := regexParse(filterConfig.Command); isRegex {
			filter.commandFilter, err = regexp.NewFilterSet([]string{commandFilter}, filterConfig.RegexpConfig)
		} else {
			filter.commandFilter = strict.NewFilterSet([]string{commandFilter})
		}
	}

	if filterConfig.CommandLine != "" {
		if commandLineFilter, isRegex := regexParse(filterConfig.CommandLine); isRegex {
			filter.commandLineFilter, err = regexp.NewFilterSet([]string{commandLineFilter}, filterConfig.RegexpConfig)
		} else {
			filter.commandLineFilter = strict.NewFilterSet([]string{commandLineFilter})
		}
	}

	if filterConfig.Owner != "" {
		if ownerFilter, isRegex := regexParse(filterConfig.Owner); isRegex {
			filter.ownerFilter, err = regexp.NewFilterSet([]string{ownerFilter}, filterConfig.RegexpConfig)
		} else {
			filter.ownerFilter = strict.NewFilterSet([]string{ownerFilter})
		}
	}

	filter.pidFilter = filterConfig.PID

	return filter, err
}