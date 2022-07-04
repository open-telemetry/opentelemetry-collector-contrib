// Copyright  OpenTelemetry Authors
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

// nolint:errcheck
package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal"

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/mongodb-forks/digest"
	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/metadata"
)

type clientRoundTripper struct {
	originalTransport http.RoundTripper
	log               *zap.Logger
	retrySettings     exporterhelper.RetrySettings
	isStopped         bool
	shutdownChan      chan struct{}
}

func newClientRoundTripper(
	originalTransport http.RoundTripper,
	log *zap.Logger,
	retrySettings exporterhelper.RetrySettings,
) *clientRoundTripper {
	return &clientRoundTripper{
		originalTransport: originalTransport,
		log:               log,
		retrySettings:     retrySettings,
		shutdownChan:      make(chan struct{}, 1),
	}
}

func (rt *clientRoundTripper) Shutdown() error {
	rt.isStopped = true
	rt.shutdownChan <- struct{}{}
	close(rt.shutdownChan)
	return nil
}

func (rt *clientRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	if rt.isStopped {
		return nil, fmt.Errorf("request cancelled due to shutdown")
	}

	resp, err := rt.originalTransport.RoundTrip(r)
	if err != nil {
		return nil, err // Can't do anything
	}
	if resp.StatusCode == 429 {
		expBackoff := &backoff.ExponentialBackOff{
			InitialInterval:     rt.retrySettings.InitialInterval,
			RandomizationFactor: backoff.DefaultRandomizationFactor,
			Multiplier:          backoff.DefaultMultiplier,
			MaxInterval:         rt.retrySettings.MaxInterval,
			MaxElapsedTime:      rt.retrySettings.MaxElapsedTime,
			Stop:                backoff.Stop,
			Clock:               backoff.SystemClock,
		}
		expBackoff.Reset()
		attempts := 0
		for {
			attempts++
			delay := expBackoff.NextBackOff()
			if delay == backoff.Stop {
				return resp, err
			}
			rt.log.Warn("server busy, retrying request",
				zap.Int("attempts", attempts),
				zap.Duration("delay", delay))
			select {
			case <-r.Context().Done():
				return resp, fmt.Errorf("request was cancelled or timed out")
			case <-rt.shutdownChan:
				return resp, fmt.Errorf("request is cancelled due to server shutdown")
			case <-time.After(delay):
			}

			resp, err = rt.originalTransport.RoundTrip(r)
			if err != nil {
				return nil, err
			}
			if resp.StatusCode != 429 {
				break
			}
		}
	}
	return resp, err
}

// MongoDBAtlasClient wraps the official MongoDB Atlas client to manage pagination
// and mapping to OpenTelmetry metric and log structures.
type MongoDBAtlasClient struct {
	log          *zap.Logger
	client       *mongodbatlas.Client
	roundTripper *clientRoundTripper
}

// NewMongoDBAtlasClient creates a new MongoDB Atlas client wrapper
func NewMongoDBAtlasClient(
	publicKey string,
	privateKey string,
	retrySettings exporterhelper.RetrySettings,
	log *zap.Logger,
) (*MongoDBAtlasClient, error) {
	t := digest.NewTransport(publicKey, privateKey)
	roundTripper := newClientRoundTripper(t, log, retrySettings)
	tc := &http.Client{Transport: roundTripper}
	client := mongodbatlas.NewClient(tc)
	return &MongoDBAtlasClient{
		log,
		client,
		roundTripper,
	}, nil
}

func (s *MongoDBAtlasClient) Shutdown() error {
	s.roundTripper.Shutdown()
	return nil
}

// Check both the returned error and the status of the HTTP response
func checkMongoDBClientErr(err error, response *mongodbatlas.Response) error {
	if err != nil {
		return err
	}
	if response != nil {
		return mongodbatlas.CheckResponse(response.Response)
	}
	return nil
}

func hasNext(links []*mongodbatlas.Link) bool {
	for _, link := range links {
		if link.Rel == "next" {
			return true
		}
	}
	return false
}

// Organizations returns a list of all organizations available with the supplied credentials
func (s *MongoDBAtlasClient) Organizations(ctx context.Context) ([]*mongodbatlas.Organization, error) {
	allOrgs := make([]*mongodbatlas.Organization, 0)
	page := 1

	for {
		orgs, hasNext, err := s.getOrganizationsPage(ctx, page)
		page++
		if err != nil {
			// TODO: Add error to a metric
			// Stop, returning what we have (probably empty slice)
			return allOrgs, fmt.Errorf("error retrieving organizations from MongoDB Atlas API: %w", err)
		}
		allOrgs = append(allOrgs, orgs...)
		if !hasNext {
			break
		}
	}
	return allOrgs, nil
}

func (s *MongoDBAtlasClient) getOrganizationsPage(
	ctx context.Context,
	pageNum int,
) ([]*mongodbatlas.Organization, bool, error) {
	orgs, response, err := s.client.Organizations.List(ctx, &mongodbatlas.OrganizationsListOptions{
		ListOptions: mongodbatlas.ListOptions{
			PageNum: pageNum,
		},
	})
	err = checkMongoDBClientErr(err, response)
	if err != nil {
		return nil, false, fmt.Errorf("error in retrieving organizations: %w", err)
	}
	return orgs.Results, hasNext(orgs.Links), nil
}

// Projects returns a list of projects accessible within the provided organization
func (s *MongoDBAtlasClient) Projects(
	ctx context.Context,
	orgID string,
) ([]*mongodbatlas.Project, error) {
	allProjects := make([]*mongodbatlas.Project, 0)
	page := 1

	for {
		projects, hasNext, err := s.getProjectsPage(ctx, orgID, page)
		page++
		if err != nil {
			return allProjects, fmt.Errorf("error retrieving list of projects from MongoDB Atlas API: %w", err)
		}
		allProjects = append(allProjects, projects...)
		if !hasNext {
			break
		}
	}
	return allProjects, nil
}

func (s *MongoDBAtlasClient) getProjectsPage(
	ctx context.Context,
	orgID string,
	pageNum int,
) ([]*mongodbatlas.Project, bool, error) {
	projects, response, err := s.client.Organizations.Projects(
		ctx,
		orgID,
		&mongodbatlas.ListOptions{PageNum: pageNum},
	)
	err = checkMongoDBClientErr(err, response)
	if err != nil {
		return nil, false, fmt.Errorf("error retrieving project page: %w", err)
	}
	return projects.Results, hasNext(projects.Links), nil
}

// Processes returns the list of processes running for a given project.
func (s *MongoDBAtlasClient) Processes(
	ctx context.Context,
	projectID string,
) ([]*mongodbatlas.Process, error) {
	// A paginated API, but the MongoDB client just returns the values from the first page

	// Note: MongoDB Atlas also has the idea of a Cluster- we can retrieve a list of clusters from
	// the Project, but a Cluster does not have a link to its Process list and a Process does not
	// have a link to its Cluster (save through the hostname, which is not a documented relationship).
	processes, response, err := s.client.Processes.List(
		ctx,
		projectID,
		&mongodbatlas.ProcessesListOptions{
			ListOptions: mongodbatlas.ListOptions{
				PageNum:      0,
				ItemsPerPage: 0,
				IncludeCount: true,
			},
		},
	)
	err = checkMongoDBClientErr(err, response)
	if err != nil {
		return make([]*mongodbatlas.Process, 0), fmt.Errorf("error retrieving processes from MongoDB Atlas API: %w", err)
	}
	return processes, nil
}

func (s *MongoDBAtlasClient) getProcessDatabasesPage(
	ctx context.Context,
	projectID string,
	host string,
	port int,
	pageNum int,
) ([]*mongodbatlas.ProcessDatabase, bool, error) {
	databases, response, err := s.client.ProcessDatabases.List(
		ctx,
		projectID,
		host,
		port,
		&mongodbatlas.ListOptions{PageNum: pageNum},
	)
	err = checkMongoDBClientErr(err, response)
	if err != nil {
		return nil, false, err
	}
	return databases.Results, hasNext(databases.Links), nil
}

// ProcessDatabases lists databases that are running in a given MongoDB Atlas process
func (s *MongoDBAtlasClient) ProcessDatabases(
	ctx context.Context,
	projectID string,
	host string,
	port int,
) ([]*mongodbatlas.ProcessDatabase, error) {
	allProcessDatabases := make([]*mongodbatlas.ProcessDatabase, 0)
	pageNum := 1
	for {
		processes, hasMore, err := s.getProcessDatabasesPage(ctx, projectID, host, port, pageNum)
		pageNum++
		if err != nil {
			return allProcessDatabases, err
		}
		allProcessDatabases = append(allProcessDatabases, processes...)
		if !hasMore {
			break
		}
	}
	return allProcessDatabases, nil
}

// ProcessMetrics returns a set of metrics associated with the specified running process.
func (s *MongoDBAtlasClient) ProcessMetrics(
	ctx context.Context,
	mb *metadata.MetricsBuilder,
	projectID string,
	host string,
	port int,
	start string,
	end string,
	resolution string,
) error {
	allMeasurements := make([]*mongodbatlas.Measurements, 0)
	pageNum := 1
	for {
		measurements, hasMore, err := s.getProcessMeasurementsPage(
			ctx,
			projectID,
			host,
			port,
			pageNum,
			start,
			end,
			resolution,
		)
		if err != nil {
			s.log.Debug("Error retrieving process metrics from MongoDB Atlas API", zap.Error(err))
			break // Return partial results
		}
		pageNum++
		allMeasurements = append(allMeasurements, measurements...)
		if !hasMore {
			break
		}
	}
	return processMeasurements(mb, allMeasurements)
}

func (s *MongoDBAtlasClient) getProcessMeasurementsPage(
	ctx context.Context,
	projectID string,
	host string,
	port int,
	pageNum int,
	start string,
	end string,
	resolution string,
) ([]*mongodbatlas.Measurements, bool, error) {
	measurements, result, err := s.client.ProcessMeasurements.List(
		ctx,
		projectID,
		host,
		port,
		&mongodbatlas.ProcessMeasurementListOptions{
			ListOptions: &mongodbatlas.ListOptions{PageNum: pageNum},
			Granularity: resolution,
			Start:       start,
			End:         end,
		},
	)
	err = checkMongoDBClientErr(err, result)
	if err != nil {
		return nil, false, err
	}
	return measurements.Measurements, hasNext(measurements.Links), nil
}

// ProcessDatabaseMetrics returns metrics about a particular database running within a MongoDB Atlas process
func (s *MongoDBAtlasClient) ProcessDatabaseMetrics(
	ctx context.Context,
	mb *metadata.MetricsBuilder,
	projectID string,
	host string,
	port int,
	dbname string,
	start string,
	end string,
	resolution string,
) error {
	allMeasurements := make([]*mongodbatlas.Measurements, 0)
	pageNum := 1
	for {
		measurements, hasMore, err := s.getProcessDatabaseMeasurementsPage(
			ctx,
			projectID,
			host,
			port,
			dbname,
			pageNum,
			start,
			end,
			resolution,
		)
		if err != nil {
			return err
		}
		pageNum++
		allMeasurements = append(allMeasurements, measurements...)
		if !hasMore {
			break
		}
	}
	return processMeasurements(mb, allMeasurements)
}

func (s *MongoDBAtlasClient) getProcessDatabaseMeasurementsPage(
	ctx context.Context,
	projectID string,
	host string,
	port int,
	dbname string,
	pageNum int,
	start string,
	end string,
	resolution string,
) ([]*mongodbatlas.Measurements, bool, error) {
	measurements, result, err := s.client.ProcessDatabaseMeasurements.List(
		ctx,
		projectID,
		host,
		port,
		dbname,
		&mongodbatlas.ProcessMeasurementListOptions{
			ListOptions: &mongodbatlas.ListOptions{PageNum: pageNum},
			Granularity: resolution,
			Start:       start,
			End:         end,
		},
	)
	err = checkMongoDBClientErr(err, result)
	if err != nil {
		return nil, false, err
	}
	return measurements.Measurements, hasNext(measurements.Links), nil
}

// ProcessDisks enumerates the disks accessible to a specified MongoDB Atlas process
func (s *MongoDBAtlasClient) ProcessDisks(
	ctx context.Context,
	projectID string,
	host string,
	port int,
) []*mongodbatlas.ProcessDisk {
	allDisks := make([]*mongodbatlas.ProcessDisk, 0)
	pageNum := 1
	for {
		disks, hasMore, err := s.getProcessDisksPage(ctx, projectID, host, port, pageNum)
		if err != nil {
			s.log.Debug("Error retrieving disk metrics from MongoDB Atlas API", zap.Error(err))
			break // Return partial results
		}
		pageNum++
		allDisks = append(allDisks, disks...)
		if !hasMore {
			break
		}
	}
	return allDisks
}

func (s *MongoDBAtlasClient) getProcessDisksPage(
	ctx context.Context,
	projectID string,
	host string,
	port int,
	pageNum int,
) ([]*mongodbatlas.ProcessDisk, bool, error) {
	disks, result, err := s.client.ProcessDisks.List(
		ctx,
		projectID,
		host,
		port,
		&mongodbatlas.ListOptions{PageNum: pageNum},
	)
	err = checkMongoDBClientErr(err, result)
	if err != nil {
		return nil, false, err
	}
	return disks.Results, hasNext(disks.Links), nil
}

// ProcessDiskMetrics returns metrics supplied for a particular disk partition used by a MongoDB Atlas process
func (s *MongoDBAtlasClient) ProcessDiskMetrics(
	ctx context.Context,
	mb *metadata.MetricsBuilder,
	projectID string,
	host string,
	port int,
	partitionName string,
	start string,
	end string,
	resolution string,
) error {
	allMeasurements := make([]*mongodbatlas.Measurements, 0)
	pageNum := 1
	for {
		measurements, hasMore, err := s.processDiskMeasurementsPage(
			ctx,
			projectID,
			host,
			port,
			partitionName,
			pageNum,
			start,
			end,
			resolution,
		)
		if err != nil {
			return err
		}
		pageNum++
		allMeasurements = append(allMeasurements, measurements...)
		if !hasMore {
			break
		}
	}
	return processMeasurements(mb, allMeasurements)
}

func (s *MongoDBAtlasClient) processDiskMeasurementsPage(
	ctx context.Context,
	projectID string,
	host string,
	port int,
	partitionName string,
	pageNum int,
	start string,
	end string,
	resolution string,
) ([]*mongodbatlas.Measurements, bool, error) {
	measurements, result, err := s.client.ProcessDiskMeasurements.List(
		ctx,
		projectID,
		host,
		port,
		partitionName,
		&mongodbatlas.ProcessMeasurementListOptions{
			ListOptions: &mongodbatlas.ListOptions{PageNum: pageNum},
			Granularity: resolution,
			Start:       start,
			End:         end,
		},
	)
	err = checkMongoDBClientErr(err, result)
	if err != nil {
		return nil, false, err
	}
	return measurements.Measurements, hasNext(measurements.Links), nil
}
