package processscraper

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestProcNameFilter (t *testing.T) {
	var filterConfigs[] FilterConfig

	filterConfigs = append(filterConfigs, FilterConfig{})
	procFilter, err := createFilters(filterConfigs)
	assert.Nil(t, err)

	//First filter should match any command
	matches := procFilter.includeCommand("anyString", "anyString", []int{0})
	assert.True(t, len(matches) == 1)
	assert.True(t, matches[0] == 0)

	filter2Config := FilterConfig{
		IncludeCommands: CommandMatchConfig{
			Config: filterset.Config{
				MatchType: filterset.Strict,
			},
			Commands: []string{"command1Match"},
		} ,
		IncludeCommandLines: CommandLineMatchConfig{
			Config: filterset.Config{
				MatchType: filterset.Strict,
			},
			CommandLines: []string {"commandLine1Match"},
		},
	}

	filter3Config := FilterConfig{
		IncludeCommands: CommandMatchConfig{
			Config: filterset.Config{
				MatchType: filterset.Strict,
			},
			Commands: []string{"command1Match"},
		} ,
	}

	filter4Config := FilterConfig{
		IncludeCommands: CommandMatchConfig{
			Config: filterset.Config{
				MatchType: filterset.Strict,
			},
			Commands: []string{"noMatch"},
		} ,
		IncludeCommandLines: CommandLineMatchConfig{
			Config: filterset.Config{
				MatchType: filterset.Strict,
			},
			CommandLines: []string {"noMatch"},
		},
	}

	filterConfigs = append(filterConfigs, filter2Config)
	filterConfigs = append(filterConfigs, filter3Config)
	filterConfigs = append(filterConfigs, filter4Config)
	procFilter, err = createFilters(filterConfigs)
	assert.Nil(t, err)

	matches = procFilter.includeCommand("command1Match", "commandLine1Match", []int{0, 1, 2})
	assert.True(t, len(matches) == 3)
	assert.True(t, matches[0] == 0)
	assert.True(t, matches[1] == 1)
	assert.True(t, matches[2] == 2)

	matches = procFilter.includeCommand("command1Match", "commandLine1Match", []int{1, 2})
	assert.True(t, len(matches) == 2)
	assert.True(t, matches[0] == 1)
	assert.True(t, matches[1] == 2)

	matches = procFilter.includeCommand("command1Match", "noMatch", []int{0, 1, 2})
	assert.True(t, len(matches) == 2)
	assert.True(t, matches[0] == 0)
	assert.True(t, matches[1] == 2)

	matches = procFilter.includeCommand("newCommand", "newCommand", []int{0, 1, 2})
	assert.True(t, len(matches) == 1)
	assert.True(t, matches[0] == 0)

}

func TestProcExecutableFilter (t *testing.T) {
	var filterConfigs[] FilterConfig

	filterConfigs = append(filterConfigs, FilterConfig{})
	procFilter, err := createFilters(filterConfigs)
	assert.Nil(t, err)

	//First filter should match any command
	matches := procFilter.includeExecutable("execName", "execPath", []int{0})
	assert.True(t, len(matches) == 1)
	assert.True(t, matches[0] == 0)



	filter2Config := FilterConfig{
		IncludeExecutableNames: ExecutableNameMatchConfig{
			Config: filterset.Config{
				MatchType: filterset.Strict,
			},
			ExecutableNames: []string{"exec1"},
		} ,
		IncludeExecutablePaths: ExecutablePathMatchConfig{
			Config: filterset.Config{
				MatchType: filterset.Strict,
			},
			ExecutablePaths: []string {"path1"},
		},
	}

	filter3Config := FilterConfig{
		IncludeExecutableNames: ExecutableNameMatchConfig{
			Config: filterset.Config{
				MatchType: filterset.Strict,
			},
			ExecutableNames: []string{"exec1"},
		} ,
	}

	filter4Config := FilterConfig{
		IncludeExecutableNames: ExecutableNameMatchConfig{
			Config: filterset.Config{
				MatchType: filterset.Strict,
			},
			ExecutableNames: []string{"noMatch"},
		} ,
		IncludeExecutablePaths: ExecutablePathMatchConfig{
			Config: filterset.Config{
				MatchType: filterset.Strict,
			},
			ExecutablePaths: []string {"noMatch"},
		},
	}

	filterConfigs = append(filterConfigs, filter2Config)
	filterConfigs = append(filterConfigs, filter3Config)
	filterConfigs = append(filterConfigs, filter4Config)
	procFilter, err = createFilters(filterConfigs)
	assert.Nil(t, err)

	matches = procFilter.includeExecutable("exec1", "path1", []int{0, 1, 2})
	assert.True(t, len(matches) == 3)
	assert.True(t, matches[0] == 0)
	assert.True(t, matches[1] == 1)
	assert.True(t, matches[2] == 2)

	matches = procFilter.includeExecutable("exec1", "path1", []int{1, 2})
	assert.True(t, len(matches) == 2)
	assert.True(t, matches[0] == 1)
	assert.True(t, matches[1] == 2)

	matches = procFilter.includeExecutable("exec1", "noMatch", []int{0, 1, 2})
	assert.True(t, len(matches) == 2)
	assert.True(t, matches[0] == 0)
	assert.True(t, matches[1] == 2)

	matches = procFilter.includeExecutable("newCommand", "newCommand", []int{0, 1, 2})
	assert.True(t, len(matches) == 1)
	assert.True(t, matches[0] == 0)

}

func TestProcOwnerFilter (t *testing.T) {
	var filterConfigs[] FilterConfig

	filterConfigs = append(filterConfigs, FilterConfig{})
	procFilter, err := createFilters(filterConfigs)
	assert.Nil(t, err)

	//First filter should match any command
	matches := procFilter.includeOwner("anyUser", []int{0})
	assert.True(t, len(matches) == 1)
	assert.True(t, matches[0] == 0)

	filter2Config := FilterConfig{
		IncludeOwners: OwnerMatchConfig{
			Config: filterset.Config{
				MatchType: filterset.Strict,
			},
			Owners: []string{"user1"},
		} ,
	}

	filter3Config := FilterConfig{
		IncludeOwners: OwnerMatchConfig{
			Config: filterset.Config{
				MatchType: filterset.Strict,
			},
			Owners: []string{"user2"},
		} ,
	}

	filter4Config := FilterConfig{
		IncludeOwners: OwnerMatchConfig{
			Config: filterset.Config{
				MatchType: filterset.Regexp,
			},
			Owners: []string{"^user"},
		} ,
	}

	filterConfigs = append(filterConfigs, filter2Config)
	filterConfigs = append(filterConfigs, filter3Config)
	filterConfigs = append(filterConfigs, filter4Config)
	procFilter, err = createFilters(filterConfigs)
	assert.Nil(t, err)

	matches = procFilter.includeOwner("user1", []int{0, 1, 2, 3})
	assert.True(t, len(matches) == 3)
	assert.True(t, matches[0] == 0)
	assert.True(t, matches[1] == 1)
	assert.True(t, matches[2] == 3)

}


func TestProcPidFilter (t *testing.T) {
	var filterConfigs[] FilterConfig

	filterConfigs = append(filterConfigs, FilterConfig{})
	procFilter, err := createFilters(filterConfigs)
	assert.Nil(t, err)

	//First filter should match any command
	matches := procFilter.includePid(1234)
	assert.True(t, len(matches) == 1)
	assert.True(t, matches[0] == 0)

	filter2Config := FilterConfig{
		IncludePids: PidMatchConfig{
			Pids: []int32 {1234},
		},
	}

	filter3Config := FilterConfig{
		IncludePids: PidMatchConfig{
			Pids: []int32 {5678},
		},
	}

	filterConfigs = append(filterConfigs, filter2Config)
	filterConfigs = append(filterConfigs, filter3Config)
	procFilter, err = createFilters(filterConfigs)
	assert.Nil(t, err)

	matches = procFilter.includePid(1234)
	assert.True(t, len(matches) == 2)
	assert.True(t, matches[0] == 0)
	assert.True(t, matches[1] == 1)
}
