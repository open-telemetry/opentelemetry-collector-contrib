package redisreceiver

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var _ client = (*fakeClient)(nil)

type fakeClient struct{}

func newFakeClient() *fakeClient {
	return &fakeClient{}
}

func (c fakeClient) delimiter() string {
	return "\n"
}

func (fakeClient) retrieveInfo() (string, error) {
	return readFile("info")
}

func readFile(fname string) (string, error) {
	file, err := ioutil.ReadFile("testdata/" + fname + ".txt")
	if err != nil {
		return "", err
	}
	return string(file), nil
}

func TestInfo(t *testing.T) {
	g := fakeClient{}
	res, err := g.retrieveInfo()
	require.Nil(t, err)
	require.True(t, strings.HasPrefix(res, "# Server"))
}
