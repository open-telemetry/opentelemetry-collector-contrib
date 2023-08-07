package datadogconnector

import (
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"reflect"
	"testing"
)

var _ component.Component = (*connectorImp)(nil) // testing that the connectorImp properly implements the type Component interface

// create test to create a connector, check that basic code compiles
func TestNewConnector(t *testing.T) {
	logger := zap.NewNop()
	config := createDefaultConfig()

	connector, err := newConnector(logger, config)
	if err != nil {
		t.Errorf("Failed to create new connector: %v", err)
	}

	if reflect.TypeOf(*connector) != reflect.TypeOf(connectorImp{}) {
		t.Errorf("Wrong type for created connector")
	}
}

// test for flag
