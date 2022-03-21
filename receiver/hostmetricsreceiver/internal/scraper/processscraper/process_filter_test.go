package processscraper

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCommandNameFilter (t *testing.T) {
	var filterConfig FilterConfig

	filterConfig.Command = "stringMatch"
	filter, err := createFilter(filterConfig)
	assert.Nil(t, err)

	match := filter.MatchesCommand("stringMatch", "")
	assert.True(t, match)

	match = filter.MatchesCommand("noMatch", "args")
	assert.False(t, match)

	// Test Regular Expression
	filterConfig.Command = RegExPrefix + "^([a-zA-Z0-9_\\-\\.]+)TestRegex"
	filter, err = createFilter(filterConfig)
	assert.Nil(t, err)

	match = filter.MatchesCommand("astring" + "TestRegex", "--command args")
	assert.True(t, match)

	match = filter.MatchesCommand("astring" + "FailTest", "--command args")
	assert.False(t, match)

	// Test Regular Expression with quotes
	filterConfig.Command = RegExPrefix + "\"^([a-zA-Z0-9_\\-\\.]+)TestRegex\""
	filter, err = createFilter(filterConfig)
	assert.Nil(t, err)

	match = filter.MatchesCommand("astring" + "TestRegex", "--command args")
	assert.True(t, match)

	match = filter.MatchesCommand("astring" + "FailTest", "--command args")
	assert.False(t, match)

	// Test Regular Expression with command line
	filterConfig.Command = RegExPrefix + "\"^([a-zA-Z0-9_\\-\\.]+)TestRegex\""
	filterConfig.CommandLine = RegExPrefix + "pas*word"
	filter, err = createFilter(filterConfig)
	assert.Nil(t, err)

	match = filter.MatchesCommand("astring" + "TestRegex", "passsssssword")
	assert.True(t, match)

	match = filter.MatchesCommand("NoMatchCommand", "passsssssword")
	assert.False(t, match)

	match = filter.MatchesCommand("astring" + "TestRegex", "NoMatchCommandLine")
	assert.False(t, match)

}


func TestPid (t *testing.T) {
	var filterConfig FilterConfig

	filterConfig.PID = 123454
	filter, err := createFilter(filterConfig)
	assert.Nil(t, err)

	match := filter.MatchesPid(123454)
	assert.True(t, match)

	match = filter.MatchesPid(11111)
	assert.False(t, match)
}

func TestOwner (t *testing.T) {
	var filterConfig FilterConfig

	filterConfig.Owner = "owner"
	filter, err := createFilter(filterConfig)
	assert.Nil(t, err)

	match := filter.MatchOwner("owner")
	assert.True(t, match)

	match = filter.MatchOwner("wrongowner")
	assert.False(t, match)

	filterConfig.Owner = RegExPrefix + "^owner"
	filter, err = createFilter(filterConfig)
	assert.Nil(t, err)

	match = filter.MatchOwner("ownerOwner")
	assert.True(t, match)

	match = filter.MatchOwner("notowner")
	assert.False(t, match)
}

func TestExecutable (t *testing.T) {
	var filterConfig FilterConfig

	filterConfig.ExecutableName = "executableName"
	filter, err := createFilter(filterConfig)
	assert.Nil(t, err)

	match := filter.MatchesExecutable("executableName", "//executable//path")
	assert.True(t, match)

	match = filter.MatchesExecutable("noMatch", "//executable//path")
	assert.False(t, match)


	filterConfig.ExecutableName = ""
	filterConfig.ExecutablePath = "//executable//path"
	filter, err = createFilter(filterConfig)
	assert.Nil(t, err)

	match = filter.MatchesExecutable("executableName", "//executable//path")
	assert.True(t, match)

	match = filter.MatchesExecutable("executableName", "//nomatch//path")
	assert.False(t, match)


	filterConfig.ExecutableName = "executableName"
	filterConfig.ExecutablePath = "//executable//path"
	filter, err = createFilter(filterConfig)
	assert.Nil(t, err)

	match = filter.MatchesExecutable("executableName", "//executable//path")
	assert.True(t, match)

	match = filter.MatchesExecutable("executableName", "//nomatch//path")
	assert.False(t, match)

	match = filter.MatchesExecutable("noMatch", "//executable//path")
	assert.False(t, match)
}
