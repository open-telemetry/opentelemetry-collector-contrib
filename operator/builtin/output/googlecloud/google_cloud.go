// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package googlecloud

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"sync"
	"time"

	vkit "cloud.google.com/go/logging/apiv2"
	"github.com/golang/protobuf/ptypes"
	"github.com/opentelemetry/opentelemetry-log-collection/entry"
	"github.com/opentelemetry/opentelemetry-log-collection/errors"
	"github.com/opentelemetry/opentelemetry-log-collection/operator"
	"github.com/opentelemetry/opentelemetry-log-collection/operator/buffer"
	"github.com/opentelemetry/opentelemetry-log-collection/operator/flusher"
	"github.com/opentelemetry/opentelemetry-log-collection/operator/helper"
	"github.com/opentelemetry/opentelemetry-log-collection/version"
	"go.uber.org/zap"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	mrpb "google.golang.org/genproto/googleapis/api/monitoredres"
	logpb "google.golang.org/genproto/googleapis/logging/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"
)

func init() {
	operator.Register("google_cloud_output", func() operator.Builder { return NewGoogleCloudOutputConfig("") })
}

// NewGoogleCloudOutputConfig creates a new google cloud output config with default
func NewGoogleCloudOutputConfig(operatorID string) *GoogleCloudOutputConfig {
	return &GoogleCloudOutputConfig{
		OutputConfig:   helper.NewOutputConfig(operatorID, "google_cloud_output"),
		BufferConfig:   buffer.NewConfig(),
		FlusherConfig:  flusher.NewConfig(),
		Timeout:        helper.Duration{Duration: 30 * time.Second},
		UseCompression: true,
	}
}

// GoogleCloudOutputConfig is the configuration of a google cloud output operator.
type GoogleCloudOutputConfig struct {
	helper.OutputConfig `yaml:",inline"`
	BufferConfig        buffer.Config   `json:"buffer,omitempty" yaml:"buffer,omitempty"`
	FlusherConfig       flusher.Config  `json:"flusher,omitempty" yaml:"flusher,omitempty"`
	Credentials         string          `json:"credentials,omitempty"      yaml:"credentials,omitempty"`
	CredentialsFile     string          `json:"credentials_file,omitempty" yaml:"credentials_file,omitempty"`
	ProjectID           string          `json:"project_id"                 yaml:"project_id"`
	LogNameField        *entry.Field    `json:"log_name_field,omitempty"   yaml:"log_name_field,omitempty"`
	TraceField          *entry.Field    `json:"trace_field,omitempty"      yaml:"trace_field,omitempty"`
	SpanIDField         *entry.Field    `json:"span_id_field,omitempty"    yaml:"span_id_field,omitempty"`
	Timeout             helper.Duration `json:"timeout,omitempty"          yaml:"timeout,omitempty"`
	UseCompression      bool            `json:"use_compression,omitempty"  yaml:"use_compression,omitempty"`
}

// Build will build a google cloud output operator.
func (c GoogleCloudOutputConfig) Build(bc operator.BuildContext) ([]operator.Operator, error) {
	outputOperator, err := c.OutputConfig.Build(bc)
	if err != nil {
		return nil, err
	}

	newBuffer, err := c.BufferConfig.Build(bc, c.ID())
	if err != nil {
		return nil, err
	}

	newFlusher := c.FlusherConfig.Build(bc.Logger.SugaredLogger)
	ctx, cancel := context.WithCancel(context.Background())

	googleCloudOutput := &GoogleCloudOutput{
		OutputOperator:  outputOperator,
		credentials:     c.Credentials,
		credentialsFile: c.CredentialsFile,
		projectID:       c.ProjectID,
		buffer:          newBuffer,
		flusher:         newFlusher,
		logNameField:    c.LogNameField,
		traceField:      c.TraceField,
		spanIDField:     c.SpanIDField,
		timeout:         c.Timeout.Raw(),
		useCompression:  c.UseCompression,
		ctx:             ctx,
		cancel:          cancel,
	}

	return []operator.Operator{googleCloudOutput}, nil
}

// GoogleCloudOutput is an operator that sends logs to google cloud logging.
type GoogleCloudOutput struct {
	helper.OutputOperator
	buffer  buffer.Buffer
	flusher *flusher.Flusher

	credentials     string
	credentialsFile string
	projectID       string

	logNameField   *entry.Field
	traceField     *entry.Field
	spanIDField    *entry.Field
	useCompression bool

	client  *vkit.Client
	timeout time.Duration

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Start will start the google cloud logger.
func (g *GoogleCloudOutput) Start() error {
	var credentials *google.Credentials
	var err error
	scope := "https://www.googleapis.com/auth/logging.write"
	switch {
	case g.credentials != "" && g.credentialsFile != "":
		return errors.NewError("at most one of credentials or credentials_file can be configured", "")
	case g.credentials != "":
		credentials, err = google.CredentialsFromJSON(context.Background(), []byte(g.credentials), scope)
		if err != nil {
			return fmt.Errorf("parse credentials: %s", err)
		}
	case g.credentialsFile != "":
		credentialsBytes, err := ioutil.ReadFile(g.credentialsFile)
		if err != nil {
			return fmt.Errorf("read credentials file: %s", err)
		}
		credentials, err = google.CredentialsFromJSON(context.Background(), credentialsBytes, scope)
		if err != nil {
			return fmt.Errorf("parse credentials: %s", err)
		}
	default:
		credentials, err = google.FindDefaultCredentials(context.Background(), scope)
		if err != nil {
			return fmt.Errorf("get default credentials: %s", err)
		}
	}

	if g.projectID == "" && credentials.ProjectID == "" {
		return fmt.Errorf("no project id found on google creds")
	}

	if g.projectID == "" {
		g.projectID = credentials.ProjectID
	}

	options := make([]option.ClientOption, 0, 2)
	options = append(options, option.WithCredentials(credentials))
	options = append(options, option.WithUserAgent("StanzaLogAgent/"+version.GetVersion()))
	if g.useCompression {
		options = append(options, option.WithGRPCDialOption(grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name))))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := vkit.NewClient(ctx, options...)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	g.client = client

	// Test writing a log message
	ctx, cancel = context.WithTimeout(context.Background(), g.timeout)
	defer cancel()
	err = g.testConnection(ctx)
	if err != nil {
		return err
	}

	g.startFlushing()
	return nil
}

func (g *GoogleCloudOutput) startFlushing() {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		g.feedFlusher(g.ctx)
	}()
}

// Stop will flush the google cloud logger and close the underlying connection
func (g *GoogleCloudOutput) Stop() error {
	g.cancel()
	g.wg.Wait()
	g.flusher.Stop()
	if err := g.buffer.Close(); err != nil {
		return err
	}
	return g.client.Close()
}

// Process processes an entry
func (g *GoogleCloudOutput) Process(ctx context.Context, e *entry.Entry) error {
	return g.buffer.Add(ctx, e)
}

// testConnection will attempt to send a test entry to google cloud logging
func (g *GoogleCloudOutput) testConnection(ctx context.Context) error {
	testEntry := entry.New()
	testEntry.Record = map[string]interface{}{"message": "Test connection"}
	req := g.createWriteRequest([]*entry.Entry{testEntry})
	if _, err := g.client.WriteLogEntries(ctx, req); err != nil {
		return fmt.Errorf("test connection: %s", err)
	}
	return nil
}

func (g *GoogleCloudOutput) feedFlusher(ctx context.Context) {
	for {
		entries, clearer, err := g.buffer.ReadChunk(ctx)
		if err != nil && err == context.Canceled {
			return
		} else if err != nil {
			g.Errorf("Failed to read chunk", zap.Error(err))
			continue
		}

		g.flusher.Do(func(ctx context.Context) error {
			req := g.createWriteRequest(entries)
			_, err := g.client.WriteLogEntries(ctx, req)
			if err != nil {
				return err
			}
			if err = clearer.MarkAllAsFlushed(); err != nil {
				g.Errorw("Failed to mark entries as flushed", zap.Error(err))
			}
			return nil
		})
	}
}

func (g *GoogleCloudOutput) createWriteRequest(entries []*entry.Entry) *logpb.WriteLogEntriesRequest {
	pbEntries := make([]*logpb.LogEntry, 0, len(entries))
	for _, entry := range entries {
		pbEntry, err := g.createProtobufEntry(entry)
		if err != nil {
			g.Errorw("Failed to create protobuf entry. Dropping entry", zap.Any("error", err))
			continue
		}
		pbEntries = append(pbEntries, pbEntry)
	}

	return &logpb.WriteLogEntriesRequest{
		LogName:  g.toLogNamePath("default"),
		Entries:  pbEntries,
		Resource: g.defaultResource(),
	}
}

func (g *GoogleCloudOutput) defaultResource() *mrpb.MonitoredResource {
	return &mrpb.MonitoredResource{
		Type: "global",
		Labels: map[string]string{
			"project_id": g.projectID,
		},
	}
}

func (g *GoogleCloudOutput) toLogNamePath(logName string) string {
	return fmt.Sprintf("projects/%s/logs/%s", g.projectID, url.PathEscape(logName))
}

func (g *GoogleCloudOutput) createProtobufEntry(e *entry.Entry) (newEntry *logpb.LogEntry, err error) {
	ts, err := ptypes.TimestampProto(e.Timestamp)
	if err != nil {
		return nil, err
	}

	newEntry = &logpb.LogEntry{
		Timestamp: ts,
		Labels:    e.Labels,
	}

	if g.logNameField != nil {
		var rawLogName string
		err := e.Read(*g.logNameField, &rawLogName)
		if err != nil {
			g.Warnw("Failed to set log name", zap.Error(err), "entry", e)
		} else {
			newEntry.LogName = g.toLogNamePath(rawLogName)
			e.Delete(*g.logNameField)
		}
	}

	if g.traceField != nil {
		err := e.Read(*g.traceField, &newEntry.Trace)
		if err != nil {
			g.Warnw("Failed to set trace", zap.Error(err), "entry", e)
		} else {
			e.Delete(*g.traceField)
		}
	}

	if g.spanIDField != nil {
		err := e.Read(*g.spanIDField, &newEntry.SpanId)
		if err != nil {
			g.Warnw("Failed to set span ID", zap.Error(err), "entry", e)
		} else {
			e.Delete(*g.spanIDField)
		}
	}

	newEntry.Severity = convertSeverity(e.Severity)
	err = setPayload(newEntry, e.Record)
	if err != nil {
		return nil, errors.Wrap(err, "set entry payload")
	}

	newEntry.Resource = getResource(e)

	return newEntry, nil
}
