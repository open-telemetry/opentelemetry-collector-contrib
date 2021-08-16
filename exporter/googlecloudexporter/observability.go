// Copyright 2020 OpenTelemetry Authors
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

package googlecloudexporter

import (
	"context"
	"strconv"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	pointCount = stats.Int64("googlecloudmonitoring/point_count", "Count of metric points written to Cloud Monitoring.", "1")
	statusKey  = tag.MustNewKey("status")
)

var viewPointCount = &view.View{
	Name:        pointCount.Name(),
	Description: pointCount.Description(),
	Measure:     pointCount,
	Aggregation: view.Sum(),
	TagKeys:     []tag.Key{statusKey},
}

func recordPointCount(ctx context.Context, success, dropped int, grpcErr error) {
	if success > 0 {
		recordPointCountDataPoint(ctx, success, "OK")
	}

	if dropped > 0 {
		st := "UNKNOWN"
		s, ok := status.FromError(grpcErr)
		if ok {
			st = statusCodeToString(s)
		}
		recordPointCountDataPoint(ctx, dropped, st)
	}
}

func recordPointCountDataPoint(ctx context.Context, points int, status string) {
	ctx, err := tag.New(ctx, tag.Insert(statusKey, status))
	if err != nil {
		return
	}

	stats.Record(ctx, pointCount.M(int64(points)))
}

func statusCodeToString(s *status.Status) string {
	// see https://github.com/grpc/grpc/blob/master/doc/statuscodes.md
	switch c := s.Code(); c {
	case codes.OK:
		return "OK"
	case codes.Canceled:
		return "CANCELLED"
	case codes.Unknown:
		return "UNKNOWN"
	case codes.InvalidArgument:
		return "INVALID_ARGUMENT"
	case codes.DeadlineExceeded:
		return "DEADLINE_EXCEEDED"
	case codes.NotFound:
		return "NOT_FOUND"
	case codes.AlreadyExists:
		return "ALREADY_EXISTS"
	case codes.PermissionDenied:
		return "PERMISSION_DENIED"
	case codes.ResourceExhausted:
		return "RESOURCE_EXHAUSTED"
	case codes.FailedPrecondition:
		return "FAILED_PRECONDITION"
	case codes.Aborted:
		return "ABORTED"
	case codes.OutOfRange:
		return "OUT_OF_RANGE"
	case codes.Unimplemented:
		return "UNIMPLEMENTED"
	case codes.Internal:
		return "INTERNAL"
	case codes.Unavailable:
		return "UNAVAILABLE"
	case codes.DataLoss:
		return "DATA_LOSS"
	case codes.Unauthenticated:
		return "UNAUTHENTICATED"
	default:
		return "CODE_" + strconv.FormatInt(int64(c), 10)
	}
}
