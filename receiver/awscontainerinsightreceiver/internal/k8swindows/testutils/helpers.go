package testutils

import (
	"encoding/json"
	"os"
	"testing"

	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/stretchr/testify/assert"
)

func LoadKubeletSummary(t *testing.T, file string) *stats.Summary {
	info, err := os.ReadFile(file)
	assert.Nil(t, err, "Fail to read sample kubelet summary response file content")

	var kSummary stats.Summary
	err = json.Unmarshal(info, &kSummary)
	assert.Nil(t, err, "Fail to parse json string")

	return &kSummary
}
