// Copyright 2020 The OpenTelemetry Authors
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

package kinesisexporter

import (
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kinesis"
	omnition "github.com/signalfx/omnition-kinesis-producer"
	"github.com/signalfx/omnition-kinesis-producer/loggers/kpzap"
	"go.uber.org/zap"
)

// producer provides the interface for a kinesis producer. The producer
// implementation abstracts the interaction with the kinesis producer library
// used from the exporter
type producer interface {
	start()
	stop()
	put(data []byte, partitionKey string) error
}

type client struct {
	client *omnition.Producer
	logger *zap.Logger
}

func newKinesisProducer(c *Config, logger *zap.Logger) (producer, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	awsConfig := aws.NewConfig().WithRegion(c.AWS.Region).WithEndpoint(c.AWS.KinesisEndpoint)
	// If AWS role is provided, use sts credentials to assume the role
	if len(c.AWS.Role) > 0 {
		creds := stscreds.NewCredentials(sess, c.AWS.Role)
		awsConfig = awsConfig.WithCredentials(creds)
	}

	o := omnition.New(&omnition.Config{
		Logger:     &kpzap.Logger{Logger: logger},
		Client:     kinesis.New(sess, awsConfig),
		StreamName: c.AWS.StreamName,
		// KPL parameters
		FlushInterval:       time.Duration(c.KPL.FlushIntervalSeconds) * time.Second,
		BatchCount:          c.KPL.BatchCount,
		BatchSize:           c.KPL.BatchSize,
		AggregateBatchCount: c.KPL.AggregateBatchCount,
		AggregateBatchSize:  c.KPL.AggregateBatchSize,
		BacklogCount:        c.KPL.BacklogCount,
		MaxConnections:      c.KPL.MaxConnections,
		MaxRetries:          c.KPL.MaxRetries,
		MaxBackoffTime:      time.Duration(c.KPL.MaxBackoffSeconds) * time.Second,
	}, nil)

	return client{client: o, logger: logger}, nil
}

func (c client) start() {
	c.client.Start()
	go c.notifyErrors()
}

// notifyErrors logs the failures within the kinesis exporter
func (c client) notifyErrors() {
	for r := range c.client.NotifyFailures() {
		// Logging error for now, these are normally unrecoverable failures
		c.logger.Error("error putting record on kinesis",
			zap.String("partitionKey", r.PartitionKey),
			zap.Error(r.Err),
		)
	}
}

func (c client) stop() {
	c.client.Stop()
}

func (c client) put(data []byte, partitionKey string) error {
	return c.client.Put(data, partitionKey)
}
