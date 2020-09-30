package metadata

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetEC2InstanceId(t *testing.T) {
	s, _ := session.NewSession()
	metadata := NewMetadata(s)
	instanceId, err := metadata.getEC2InstanceId()
	assert.NotNil(t, instanceId)
	assert.Nil(t, err)
}

func TestGetHostIdentifier(t *testing.T) {
	s, _ := session.NewSession()
	metadata := NewMetadata(s)
	id, err := metadata.GetHostIdentifier()
	fmt.Printf("id is %s, error is %v\n", id, err)
}
