// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package procx

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type fakeProcessWrapper struct {
	pid     int32
	name    func() (string, error)
	cmdLine func() (string, error)
}

func (pw *fakeProcessWrapper) Pid() int32 {
	return pw.pid
}

func (pw *fakeProcessWrapper) Name() (string, error) {
	return pw.name()
}

func (pw *fakeProcessWrapper) Cmdline() (string, error) {
	return pw.cmdLine()
}

func TestFilteredProcessList(t *testing.T) {
	tests := []struct {
		name         string
		getProcesses func() ([]Process, error)
		want         []string
	}{
		{
			name: "list processes",
			getProcesses: func() ([]Process, error) {
				ret := []Process{
					&fakeProcessWrapper{
						pid: 8080,
						name: func() (string, error) {
							return "apache.exe", nil
						},
						cmdLine: func() (string, error) {
							return "", nil
						},
					},
					&fakeProcessWrapper{
						pid: 8081,
						name: func() (string, error) {
							return "mysqld.exe", nil
						},
						cmdLine: func() (string, error) {
							return "", nil
						},
					},
				}
				return ret, nil
			},
			want: []string{"apache", "mysql"},
		},
		{
			name: "list process with partial error",
			getProcesses: func() ([]Process, error) {
				ret := []Process{
					&fakeProcessWrapper{
						pid: 8080,
						name: func() (string, error) {
							return "", errors.New("invalid process")
						},
						cmdLine: func() (string, error) {
							return "", nil
						},
					},
					&fakeProcessWrapper{
						pid: 8081,
						name: func() (string, error) {
							return "mysqld.exe", nil
						},
						cmdLine: func() (string, error) {
							return "", nil
						},
					},
				}
				return ret, nil
			},
			want: []string{"mysql"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			procx := &Procx{
				logger:       zap.NewNop(),
				getProcesses: tt.getProcesses,
			}
			pl, err := procx.FilteredProcessList()
			assert.Equal(t, tt.want, pl)
			assert.NoError(t, err)
		})
	}
}
