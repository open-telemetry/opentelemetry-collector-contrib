// Copyright 2020, OpenTelemetry Authors
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

package metadata

import (
	"bufio"
	"io"
	"os"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

const (
	containerIDLength = 64
	defaultCGroupPath = "/proc/self/cgroup"
)

type Metadata interface {
	GetCollectorIdentifier() (string, error)
}

type metadata struct {
	ec2    *ec2metadata.EC2Metadata
	docker *DockerHelper
	hostID string
}

func NewMetadata(s *session.Session, cGroupPath string) Metadata {
	if cGroupPath == "" {
		cGroupPath = defaultCGroupPath
	}
	return &metadata{
		ec2: ec2metadata.New(s),
		docker: &DockerHelper{
			cGroupPath: cGroupPath,
		},
	}
}

func (m *metadata) getEC2InstanceID() (string, error) {
	instance, err := m.ec2.GetInstanceIdentityDocument()
	if err != nil {
		return "", err
	}
	return instance.InstanceID, nil
}

func (m *metadata) getContainerID() (string, error) {
	return m.docker.getContainerID()
}

func (m *metadata) GetCollectorIdentifier() (string, error) {
	if m.hostID != "" {
		return m.hostID, nil
	}
	var id string
	var err error
	id, err = m.getContainerID()
	if err == nil {
		m.hostID = id
		return id, nil
	}
	id, err = m.getEC2InstanceID()
	if err == nil {
		m.hostID = id
	}
	return id, err
}

type DockerHelper struct {
	cGroupPath string
}

func (d *DockerHelper) getContainerID() (string, error) {
	file, err := os.Open(d.cGroupPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	containerID := ""
	var line string
	for {
		line, err = reader.ReadString('\n')
		if len(line) > containerIDLength {
			startIndex := len(line) - containerIDLength
			containerID = line[startIndex:]
			return containerID, nil
		}
		if err == io.EOF {
			break
		}
	}
	return containerID, nil
}
