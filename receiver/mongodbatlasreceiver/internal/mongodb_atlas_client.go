// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal"

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
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
	stopped           bool
	mutex             sync.Mutex
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

func (rt *clientRoundTripper) isStopped() bool {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()

	return rt.stopped
}

func (rt *clientRoundTripper) stop() {
	rt.mutex.Lock()
	defer rt.mutex.Unlock()

	rt.stopped = true
}

func (rt *clientRoundTripper) Shutdown() error {
	if rt.isStopped() {
		return nil
	}

	rt.stop()
	rt.shutdownChan <- struct{}{}
	close(rt.shutdownChan)
	return nil
}

func (rt *clientRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	if rt.isStopped() {
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
) *MongoDBAtlasClient {
	t := digest.NewTransport(publicKey, privateKey)
	roundTripper := newClientRoundTripper(t, log, retrySettings)
	tc := &http.Client{Transport: roundTripper}
	client := mongodbatlas.NewClient(tc)
	return &MongoDBAtlasClient{
		log,
		client,
		roundTripper,
	}
}

func (s *MongoDBAtlasClient) Shutdown() error {
	return s.roundTripper.Shutdown()
}

// Check both the returned error and the status of the HTTP response
func checkMongoDBClientErr(err error, response *mongodbatlas.Response) error {
	if err != nil {
		return err
	}
	if response != nil {
		return response.CheckResponse(response.Body)
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
	var allOrgs []*mongodbatlas.Organization
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

// GetOrganization retrieves a single organization specified by orgID
func (s *MongoDBAtlasClient) GetOrganization(ctx context.Context, orgID string) (*mongodbatlas.Organization, error) {
	org, response, err := s.client.Organizations.Get(ctx, orgID)
	err = checkMongoDBClientErr(err, response)
	if err != nil {
		return nil, fmt.Errorf("error retrieving project page: %w", err)
	}
	return org, nil

}

// Projects returns a list of projects accessible within the provided organization
func (s *MongoDBAtlasClient) Projects(
	ctx context.Context,
	orgID string,
) ([]*mongodbatlas.Project, error) {
	var allProjects []*mongodbatlas.Project
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

// GetProject returns a single project specified by projectName
func (s *MongoDBAtlasClient) GetProject(ctx context.Context, projectName string) (*mongodbatlas.Project, error) {
	project, response, err := s.client.Projects.GetOneProjectByName(ctx, projectName)
	err = checkMongoDBClientErr(err, response)
	if err != nil {
		return nil, fmt.Errorf("error retrieving project page: %w", err)
	}
	return project, nil
}

func (s *MongoDBAtlasClient) getProjectsPage(
	ctx context.Context,
	orgID string,
	pageNum int,
) ([]*mongodbatlas.Project, bool, error) {
	projects, response, err := s.client.Organizations.Projects(
		ctx,
		orgID,
		&mongodbatlas.ProjectsListOptions{
			ListOptions: mongodbatlas.ListOptions{PageNum: pageNum},
		},
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
		return nil, fmt.Errorf("error retrieving processes from MongoDB Atlas API: %w", err)
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
	var allProcessDatabases []*mongodbatlas.ProcessDatabase
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
	var allMeasurements []*mongodbatlas.Measurements
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
	var allMeasurements []*mongodbatlas.Measurements
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
	var allDisks []*mongodbatlas.ProcessDisk
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
	var allMeasurements []*mongodbatlas.Measurements
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

// GetLogs retrieves the logs from the mongo API using API call: https://www.mongodb.com/docs/atlas/reference/api/logs/#syntax
func (s *MongoDBAtlasClient) GetLogs(ctx context.Context, groupID, hostname, logName string, start, end time.Time) (*bytes.Buffer, error) {
	buf := bytes.NewBuffer([]byte{})

	dateRange := &mongodbatlas.DateRangetOptions{StartDate: toUnixString(start), EndDate: toUnixString(end)}
	resp, err := s.client.Logs.Get(ctx, groupID, hostname, logName, buf, dateRange)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received status code: %d", resp.StatusCode)
	}

	return buf, nil
}

// GetClusters retrieves the clusters from the mongo API using API call: https://www.mongodb.com/docs/atlas/reference/api/clusters-get-all/#request
func (s *MongoDBAtlasClient) GetClusters(ctx context.Context, groupID string) ([]mongodbatlas.Cluster, error) {
	options := mongodbatlas.ListOptions{}

	clusters, _, err := s.client.Clusters.List(ctx, groupID, &options)
	if err != nil {
		return nil, err
	}

	return clusters, nil
}

type AlertPollOptions struct {
	PageNum  int
	PageSize int
}

// GetAlerts returns the alerts specified for the set projects
func (s *MongoDBAtlasClient) GetAlerts(ctx context.Context, groupID string, opts *AlertPollOptions) (ret []mongodbatlas.Alert, nextPage bool, err error) {
	lo := mongodbatlas.ListOptions{
		PageNum:      opts.PageNum,
		ItemsPerPage: opts.PageSize,
	}
	options := mongodbatlas.AlertsListOptions{ListOptions: lo}
	alerts, response, err := s.client.Alerts.List(ctx, groupID, &options)
	err = checkMongoDBClientErr(err, response)
	if err != nil {
		return nil, false, err
	}
	return alerts.Results, hasNext(response.Links), nil
}

// GetEventsOptions are the options to use for making a request to get Project Events
type GetEventsOptions struct {
	// Which page of the paginated events
	PageNum int
	// How large the Pages will be
	PageSize int
	// The list of Event Types https://www.mongodb.com/docs/atlas/reference/api/events-projects-get-all/#event-type-values
	// to grab from the API
	EventTypes []string
	// The oldest date to look back for the events
	MinDate time.Time
	// the newest time to accept events
	MaxDate time.Time
}

// GetProjectEvents returns the events specified for the set projects
func (s *MongoDBAtlasClient) GetProjectEvents(ctx context.Context, groupID string, opts *GetEventsOptions) (ret []*mongodbatlas.Event, nextPage bool, err error) {
	lo := mongodbatlas.ListOptions{
		PageNum:      opts.PageNum,
		ItemsPerPage: opts.PageSize,
	}
	options := mongodbatlas.EventListOptions{
		ListOptions: lo,
		// Earliest Timestamp in ISO 8601 date and time format in UTC from when Atlas should return events.
		MinDate: opts.MinDate.Format(time.RFC3339),
	}

	if len(opts.EventTypes) > 0 {
		options.EventType = opts.EventTypes
	}

	events, response, err := s.client.Events.ListProjectEvents(ctx, groupID, &options)
	err = checkMongoDBClientErr(err, response)
	if err != nil {
		return nil, false, err
	}
	return events.Results, hasNext(response.Links), nil
}

// GetOrgEvents returns the events specified for the set organizations
func (s *MongoDBAtlasClient) GetOrganizationEvents(ctx context.Context, orgID string, opts *GetEventsOptions) (ret []*mongodbatlas.Event, nextPage bool, err error) {
	lo := mongodbatlas.ListOptions{
		PageNum:      opts.PageNum,
		ItemsPerPage: opts.PageSize,
	}
	options := mongodbatlas.EventListOptions{
		ListOptions: lo,
		// Earliest Timestamp in ISO 8601 date and time format in UTC from when Atlas should return events.
		MinDate: opts.MinDate.Format(time.RFC3339),
	}

	if len(opts.EventTypes) > 0 {
		options.EventType = opts.EventTypes
	}

	events, response, err := s.client.Events.ListOrganizationEvents(ctx, orgID, &options)
	err = checkMongoDBClientErr(err, response)
	if err != nil {
		return nil, false, err
	}
	return events.Results, hasNext(response.Links), nil
}

// GetAccessLogsOptions are the options to use for making a request to get Access Logs
type GetAccessLogsOptions struct {
	// The oldest date to look back for the events
	MinDate time.Time
	// the newest time to accept events
	MaxDate time.Time
	// If true, only return successful access attempts; if false, only return failed access attempts
	// If nil, return both successful and failed access attempts
	AuthResult *bool
	// Maximum number of entries to return
	NLogs int
}

// GetAccessLogs returns the access logs specified for the cluster requested
func (s *MongoDBAtlasClient) GetAccessLogs(ctx context.Context, groupID string, clusterName string, opts *GetAccessLogsOptions) (ret []*mongodbatlas.AccessLogs, err error) {

	options := mongodbatlas.AccessLogOptions{
		// Earliest Timestamp in epoch milliseconds from when Atlas should access log results
		Start: fmt.Sprintf("%d", opts.MinDate.UTC().UnixMilli()),
		// Latest Timestamp in epoch milliseconds from when Atlas should access log results
		End: fmt.Sprintf("%d", opts.MaxDate.UTC().UnixMilli()),
		// If true, only return successful access attempts; if false, only return failed access attempts
		// If nil, return both successful and failed access attempts
		AuthResult: opts.AuthResult,
		// Maximum number of entries to return (0-20000)
		NLogs: opts.NLogs,
	}

	accessLogs, response, err := s.client.AccessTracking.ListByCluster(ctx, groupID, clusterName, &options)
	err = checkMongoDBClientErr(err, response)
	if err != nil {
		return nil, err
	}
	return accessLogs.AccessLogs, nil
}

func toUnixString(t time.Time) string {
	return strconv.Itoa(int(t.Unix()))
}
