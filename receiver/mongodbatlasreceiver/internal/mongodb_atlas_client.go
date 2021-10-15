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

package internal

import (
	"context"
	"fmt"

	"github.com/mongodb-forks/digest"
	"go.mongodb.org/atlas/mongodbatlas"
	"go.uber.org/zap"
)

// MongoDBAtlasClient wraps the official MongoDB Atlas client to manage pagination
// and mapping to OpenTelmetry metric and log structures.
type MongoDBAtlasClient struct {
	log    *zap.Logger
	client *mongodbatlas.Client
}

func NewMongoDBAtlasClient(
	publicKey string,
	privateKey string,
	log *zap.Logger,
) (*MongoDBAtlasClient, error) {
	t := digest.NewTransport(publicKey, privateKey)
	tc, err := t.Client()
	if err != nil {
		return nil, fmt.Errorf("could not create MongoDB Atlas transport HTTP client: %w", err)
	}
	client := mongodbatlas.NewClient(tc)
	return &MongoDBAtlasClient{
		log,
		client,
	}, nil
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
func (s *MongoDBAtlasClient) Organizations(ctx context.Context) []*mongodbatlas.Organization {
	allOrgs := make([]*mongodbatlas.Organization, 0)
	page := 1

	for {
		orgs, hasNext, err := s.getOrganizationsPage(ctx, page)
		page++
		if err != nil {
			// TODO: Add error to a metric
			s.log.Debug("Error retrieving organizations from MongoDB Atlas API", zap.Error(err))
			break // Stop, returning what we have (probably empty slice)
		}
		allOrgs = append(allOrgs, orgs...)
		if !hasNext {
			break
		}
	}
	return allOrgs
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
