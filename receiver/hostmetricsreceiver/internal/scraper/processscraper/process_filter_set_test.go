package processscraper

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestProcNameFilter (t *testing.T) {
	var filterConfigs[] FilterConfig

	filterConfigs = append(filterConfigs, FilterConfig{})
	procFilter, err := createFilters(filterConfigs)
	assert.Nil(t, err)

	//First filter should match any command
	matches := procFilter.MatchesCommand("anyString", "anyString", []int{0})
	assert.True(t, len(matches) == 1)
	assert.True(t, matches[0] == 0)

	filter2Config := FilterConfig{
		Command: "command1Match",
		CommandLine: "commandLine1Match",
	}

	filter3Config := FilterConfig{
		Command: "command1Match",
		CommandLine: "",
	}

	filter4Config := FilterConfig{
		Command: "noMatch",
		CommandLine: "noMatch",
	}
	filterConfigs = append(filterConfigs, filter2Config)
	filterConfigs = append(filterConfigs, filter3Config)
	filterConfigs = append(filterConfigs, filter4Config)
	procFilter, err = createFilters(filterConfigs)
	assert.Nil(t, err)

	matches = procFilter.MatchesCommand("command1Match", "commandLine1Match", []int{0, 1, 2})
	assert.True(t, len(matches) == 3)
	assert.True(t, matches[0] == 0)
	assert.True(t, matches[1] == 1)
	assert.True(t, matches[2] == 2)

	matches = procFilter.MatchesCommand("command1Match", "commandLine1Match", []int{1, 2})
	assert.True(t, len(matches) == 2)
	assert.True(t, matches[0] == 1)
	assert.True(t, matches[1] == 2)

	matches = procFilter.MatchesCommand("command1Match", "noMatch", []int{0, 1, 2})
	assert.True(t, len(matches) == 2)
	assert.True(t, matches[0] == 0)
	assert.True(t, matches[1] == 2)

	matches = procFilter.MatchesCommand("newCommand", "newCommand", []int{0, 1, 2})
	assert.True(t, len(matches) == 1)
	assert.True(t, matches[0] == 0)

}

func TestProcExecutableFilter (t *testing.T) {
	var filterConfigs[] FilterConfig

	filterConfigs = append(filterConfigs, FilterConfig{})
	procFilter, err := createFilters(filterConfigs)
	assert.Nil(t, err)

	//First filter should match any command
	matches := procFilter.MatchesExecutable("execName", "execPath", []int{0})
	assert.True(t, len(matches) == 1)
	assert.True(t, matches[0] == 0)

	filter1Config := FilterConfig{
		ExecutableName: "exec1",
		ExecutablePath: "path1",
	}

	filter2Config := FilterConfig{
		ExecutableName: "exec1",
		ExecutablePath: "",
	}

	filter3Config := FilterConfig{
		ExecutableName: "noMatch",
		ExecutablePath: "noMatch",
	}
	filterConfigs = append(filterConfigs, filter1Config)
	filterConfigs = append(filterConfigs, filter2Config)
	filterConfigs = append(filterConfigs, filter3Config)
	procFilter, err = createFilters(filterConfigs)
	assert.Nil(t, err)

	matches = procFilter.MatchesExecutable("exec1", "path1", []int{0, 1, 2})
	assert.True(t, len(matches) == 3)
	assert.True(t, matches[0] == 0)
	assert.True(t, matches[1] == 1)
	assert.True(t, matches[2] == 2)

	matches = procFilter.MatchesExecutable("exec1", "path1", []int{1, 2})
	assert.True(t, len(matches) == 2)
	assert.True(t, matches[0] == 1)
	assert.True(t, matches[1] == 2)

	matches = procFilter.MatchesExecutable("exec1", "noMatch", []int{0, 1, 2})
	assert.True(t, len(matches) == 2)
	assert.True(t, matches[0] == 0)
	assert.True(t, matches[1] == 2)

	matches = procFilter.MatchesExecutable("newCommand", "newCommand", []int{0, 1, 2})
	assert.True(t, len(matches) == 1)
	assert.True(t, matches[0] == 0)

}


func TestProcOwnerFilter (t *testing.T) {
	var filterConfigs[] FilterConfig

	filterConfigs = append(filterConfigs, FilterConfig{})
	procFilter, err := createFilters(filterConfigs)
	assert.Nil(t, err)

	//First filter should match any command
	matches := procFilter.MatchesOwner("anyUser", []int{0})
	assert.True(t, len(matches) == 1)
	assert.True(t, matches[0] == 0)

	filter1Config := FilterConfig{
		Owner: "user1",
	}

	filter2Config := FilterConfig{
		Owner: "user2",
	}

	filter3Config := FilterConfig{
		Owner: RegExPrefix + "^user",
	}
	filterConfigs = append(filterConfigs, filter1Config)
	filterConfigs = append(filterConfigs, filter2Config)
	filterConfigs = append(filterConfigs, filter3Config)
	procFilter, err = createFilters(filterConfigs)
	assert.Nil(t, err)

	matches = procFilter.MatchesOwner("user1", []int{0, 1, 2, 3})
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
	matches := procFilter.MatchesPid(1234)
	assert.True(t, len(matches) == 1)
	assert.True(t, matches[0] == 0)

	filter1Config := FilterConfig{
		PID: 1234,
	}

	filter2Config := FilterConfig{
		PID: 5678,
	}

	filterConfigs = append(filterConfigs, filter1Config)
	filterConfigs = append(filterConfigs, filter2Config)
	procFilter, err = createFilters(filterConfigs)
	assert.Nil(t, err)

	matches = procFilter.MatchesPid(1234)
	assert.True(t, len(matches) == 2)
	assert.True(t, matches[0] == 0)
	assert.True(t, matches[1] == 1)
}
