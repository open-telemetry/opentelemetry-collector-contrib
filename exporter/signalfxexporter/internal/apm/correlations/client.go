// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Originally copied from https://github.com/signalfx/signalfx-agent/blob/fbc24b0fdd3884bd0bbfbd69fe3c83f49d4c0b77/pkg/apm/correlations/client.go

package correlations // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/correlations"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/log"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/requests"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/requests/requestcounter"
)

var ErrChFull = errors.New("request channel full")
var errRetryChFull = errors.New("retry channel full")
var errMaxAttempts = errors.New("maximum attempts exceeded")
var errRequestCancelled = errors.New("request cancelled")

// ErrMaxEntries is an error returned when the correlation endpoint returns a 418 http status
// code indicating that the set of services or environments is too large to add another value
type ErrMaxEntries struct {
	MaxEntries int64 `json:"max,omitempty"`
}

func (m *ErrMaxEntries) Error() string {
	return fmt.Sprintf("max entries %d", m.MaxEntries)
}

var _ error = (*ErrMaxEntries)(nil)

// CorrelationClient is an interface for correlations.Client
type CorrelationClient interface {
	Correlate(*Correlation, CorrelateCB)
	Delete(*Correlation, SuccessfulDeleteCB)
	Get(dimName string, dimValue string, cb SuccessfulGetCB)
	Start()
}

type request struct {
	*Correlation
	ctx       context.Context
	cancel    context.CancelFunc
	operation string
	callback  func(body []byte, statuscode int, err error)
	sendAt    time.Time
}

// Client is a client for making dimensional correlations
type Client struct {
	sync.RWMutex
	log           log.Logger
	ctx           context.Context
	wg            sync.WaitGroup
	Token         string
	APIURL        *url.URL
	client        *http.Client
	requestSender *requests.ReqSender
	requestChan   chan *request
	retryChan     chan *request
	dedup         *deduplicator

	// For easier unit testing
	now        func() time.Time
	logUpdates bool

	retryDelay  time.Duration
	maxAttempts uint32

	TotalClientError4xxResponses int64
	TotalRetriedUpdates          int64
	TotalInvalidDimensions       int64
	dedupCleanupInterval         time.Duration
}

// Config defines configuration for correlation settings.
type Config struct {
	MaxRequests     uint          `mapstructure:"max_requests"`
	MaxBuffered     uint          `mapstructure:"max_buffered"`
	MaxRetries      uint          `mapstructure:"max_retries"`
	LogUpdates      bool          `mapstructure:"log_updates"`
	RetryDelay      time.Duration `mapstructure:"retry_delay"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

// ClientConfig for correlation client.
type ClientConfig struct {
	Config
	AccessToken string
	URL         *url.URL
}

// NewCorrelationClient returns a new Client
func NewCorrelationClient(ctx context.Context, log log.Logger, client *http.Client, conf ClientConfig) (CorrelationClient, error) {
	sender := requests.NewReqSender(ctx, client, conf.MaxRequests, "correlation")
	return &Client{
		log:                  log,
		ctx:                  ctx,
		Token:                conf.AccessToken,
		APIURL:               conf.URL,
		requestSender:        sender,
		client:               client,
		now:                  time.Now,
		logUpdates:           conf.LogUpdates,
		requestChan:          make(chan *request, conf.MaxBuffered),
		retryChan:            make(chan *request, conf.MaxBuffered),
		dedup:                newDeduplicator(int(conf.MaxBuffered)),
		retryDelay:           conf.RetryDelay,
		maxAttempts:          uint32(conf.MaxRetries) + 1,
		dedupCleanupInterval: conf.CleanupInterval,
	}, nil
}

func (cc *Client) putRequestOnChan(r *request) error {
	// prevent requests against empty dimension names and values
	if r.DimName == "" || r.DimValue == "" {
		// logging this as debug because this means there's no actual dimension to correlate with
		// and because this isn't being taken off on the request sender and subject to retries, this could
		// potentially spam the logs
		atomic.AddInt64(&cc.TotalInvalidDimensions, int64(1))
		r.Logger(cc.log).WithFields(log.Fields{"method": r.operation}).Debug("No dimension key or value to correlate to")
		return nil
	}

	r.ctx, r.cancel = context.WithCancel(requestcounter.ContextWithRequestCounter(context.Background()))

	var err error
	select {
	case cc.requestChan <- r:
	case <-cc.ctx.Done():
		err = context.DeadlineExceeded
	default:
		err = ErrChFull
	}
	return err
}

func (cc *Client) putRequestOnRetryChan(r *request) error {
	// handle request counter
	if requestcounter.GetRequestCount(r.ctx) >= cc.maxAttempts {
		return errMaxAttempts
	}
	requestcounter.IncrementRequestCount(r.ctx)

	// set the time to retry
	r.sendAt = cc.now().Add(cc.retryDelay)

	if r.ctx.Err() != nil {
		return errRequestCancelled
	}

	var err error
	select {
	case <-r.ctx.Done():
		err = errRequestCancelled
	case cc.retryChan <- r:
	case <-cc.ctx.Done():
		err = context.DeadlineExceeded
	default:
		err = errRetryChFull
	}

	return err
}

// CorrelateCB is a call back invoked with Correlate requests
// it is not invoked if the reqeust is deduplicated, cancelled, or the client context is cancelled
type CorrelateCB func(cor *Correlation, err error)

// Correlate
func (cc *Client) Correlate(cor *Correlation, cb CorrelateCB) {
	err := cc.putRequestOnChan(&request{
		Correlation: cor,
		operation:   http.MethodPut,
		callback: func(body []byte, statuscode int, err error) {
			switch statuscode {
			case http.StatusOK:
				if cc.logUpdates {
					cor.Logger(cc.log).WithFields(log.Fields{"method": http.MethodPut}).Info("Updated dimension")
				}
			case http.StatusTeapot:
				max := &ErrMaxEntries{}
				err = json.Unmarshal(body, max)
				if err == nil {
					err = max
				}
			}
			if err != nil {
				cor.Logger(cc.log).WithError(err).WithFields(log.Fields{"method": http.MethodPut}).Error("Unable to update dimension, not retrying")
			}
			cb(cor, err)
		}})
	if err != nil {
		cor.Logger(cc.log).WithError(err).WithFields(log.Fields{"method": http.MethodPut}).Debug("Unable to update dimension, not retrying")
	}
}

// SuccessfulDeleteCB is a call back that is only invoked on successful Deletion operations
type SuccessfulDeleteCB func(cor *Correlation)

// Delete removes a correlation
func (cc *Client) Delete(cor *Correlation, callback SuccessfulDeleteCB) {
	err := cc.putRequestOnChan(&request{
		Correlation: cor,
		operation:   http.MethodDelete,
		callback: func(_ []byte, statuscode int, err error) {
			switch statuscode {
			case http.StatusOK:
				callback(cor)
				if cc.logUpdates {
					cor.Logger(cc.log).WithFields(log.Fields{"method": http.MethodDelete}).Info("Updated dimension")
				}
			default:
				cc.log.WithError(err).Error("Unable to update dimension, not retrying")
			}
		}})
	if err != nil {
		cor.Logger(cc.log).WithError(err).WithFields(log.Fields{"method": http.MethodDelete}).Debug("Unable to update dimension, not retrying")
	}
}

// SuccessfulGetCB
type SuccessfulGetCB func(map[string][]string)

// Get
func (cc *Client) Get(dimName string, dimValue string, callback SuccessfulGetCB) {
	err := cc.putRequestOnChan(&request{
		Correlation: &Correlation{
			DimName:  dimName,
			DimValue: dimValue,
		},
		operation: http.MethodGet,
		callback: func(body []byte, statuscode int, err error) {
			switch statuscode {
			case http.StatusOK:
				var response = map[string][]string{}
				err = json.Unmarshal(body, &response)
				if err != nil {
					cc.log.WithError(err).WithFields(log.Fields{"dim": dimName, "value": dimValue}).Error("Unable to unmarshall correlations for dimension")
					return
				}
				callback(response)
			case http.StatusNotFound:
				// only log this as debug because we do a blanket fetch of correlations on the backend
				// and if the backend fails to find anything this isn't really an error for us
				cc.log.WithError(err).Debug("Unable to update dimension, not retrying")
			default:
				cc.log.WithError(err).Error("Unable to update dimension, not retrying")
			}
		},
	})
	if err != nil {
		cc.log.WithError(err).WithFields(log.Fields{"dimensionName": dimName, "dimensionValue": dimValue}).Debug("Unable to retrieve correlations for dimension, not retrying")
	}
}

func (cc *Client) makeRequest(r *request) {
	var (
		req *http.Request
		err error
	)

	// build endpoint url
	endpoint := fmt.Sprintf("%s/v2/apm/correlate/%s/%s", cc.APIURL, url.PathEscape(r.DimName), url.PathEscape(r.DimValue))

	switch r.operation {
	case http.MethodGet:
		req, err = http.NewRequest(r.operation, endpoint, nil)
	case http.MethodPut:
		// TODO: pool the reader
		endpoint = fmt.Sprintf("%s/%s", endpoint, r.Type)
		req, err = http.NewRequest(r.operation, endpoint, strings.NewReader(r.Value))
		req.Header.Add("Content-Type", "text/plain")
	case http.MethodDelete:
		endpoint = fmt.Sprintf("%s/%s/%s", endpoint, r.Type, url.PathEscape(r.Value))
		req, err = http.NewRequest(r.operation, endpoint, nil)
	default:
		err = fmt.Errorf("unknown operation")
	}

	if err != nil {
		// logging this as debug because this means there's something fundamentally wrong with the request
		// and because this isn't being taken off on the request sender and subject to retries, this could
		// potentially spam the logs long term.  This would be a really good candidate for a throttled error logger
		r.Correlation.Logger(cc.log).WithError(err).WithFields(log.Fields{"method": r.operation}).Debug("Unable to make request, not retrying")
		r.cancel()
		return
	}

	req.Header.Add("X-SF-TOKEN", cc.Token)

	req = req.WithContext(
		context.WithValue(req.Context(), requests.RequestFailedCallbackKey, requests.RequestFailedCallback(func(body []byte, statusCode int, err error) {
			// retry if the http status code is not 4XX. A 4xx or http client error implies
			// an error that is not going to be remedied by retrying.
			if statusCode < 400 || statusCode >= 500 {
				// The retry (for non 400 errors) is meant to provide some measure of robustness against
				// temporary API failures.  If the API is down for significant
				// periods of time, correlation updates will probably eventually back
				// up beyond conf.MaxBuffered and start dropping.
				retryErr := cc.putRequestOnRetryChan(r)
				if retryErr == nil {
					r.Correlation.Logger(cc.log).WithError(err).WithFields(log.Fields{"method": req.Method}).Debug("Unable to update dimension, retrying")
					return
				}
			} else {
				atomic.AddInt64(&cc.TotalClientError4xxResponses, int64(1))
			}

			// invoke the callback
			r.callback(body, statusCode, err)

			// cancel the request context
			r.cancel()
		})))

	req = req.WithContext(
		context.WithValue(req.Context(), requests.RequestSuccessCallbackKey, requests.RequestSuccessCallback(func(body []byte) {
			r.callback(body, http.StatusOK, nil)
			// close the request context
			r.cancel()
		})))

	// This will block if we don't have enough requests
	cc.requestSender.Send(req)
}

// routines
// processChan processes incoming requests, drops duplicates, and cancels conflicting requests
func (cc *Client) processChan() {
	defer cc.wg.Done()
	purgeDeduper := time.NewTimer(cc.dedupCleanupInterval)
	defer purgeDeduper.Stop()
	for {
		select {
		case <-cc.ctx.Done():
			return
		case <-purgeDeduper.C:
			cc.dedup.purge()
			purgeDeduper.Reset(cc.dedupCleanupInterval)
		case r := <-cc.requestChan:
			if cc.dedup.isDup(r) {
				r.cancel()
				continue
			}
			cc.makeRequest(r)
		}
	}
}

// processRetryChan is a routine that drains the retry channel and waits until the appropriate time to retry the request
func (cc *Client) processRetryChan() {
	defer cc.wg.Done()
	for {
		select {
		case <-cc.ctx.Done(): // client is shutdown
			return
		case r := <-cc.retryChan:
			if r.ctx.Err() != nil {
				continue
			}
			select {
			case <-time.After(time.Until(r.sendAt)): // wait and resend the request
				atomic.AddInt64(&cc.TotalRetriedUpdates, int64(1))
				cc.makeRequest(r)
			case <-r.ctx.Done(): // request is cancelled
				continue
			case <-cc.ctx.Done(): // client is shutdown
				return
			}
		}
	}
}

// Start the client's processing queue
func (cc *Client) Start() {
	cc.wg.Add(2)
	go cc.processChan()
	go cc.processRetryChan()
}
