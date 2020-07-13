package query

import (
	"encoding/json"
)

// Maps aggregation names to a corresponding body
type aggregationsMap map[string]*aggregationResponse

// Maps keys to corresponding buckets
type bucketsMap map[interface{}]*bucketResponse

type HTTPResponse struct {
	Aggregations aggregationsMap `json:"aggregations"`
}

// Represents all kinds of supported aggregations. aggregationResponse type can
// be determined in the following manner -
// 1) single bucket aggregationInfo contains only "doc_count" and a map to sub aggregations
// 2) multi bucket aggregationInfo contains "buckets"and a map to sub aggregations
// 3) metric aggregationInfo contains "value" or "values"
// These structs are defined based on inputs from
// https://github.com/elastic/elasticsearch/tree/master/server/src/main/java/org/elasticsearch/search/aggregations
type aggregationResponse struct {
	// Number of documents backing the aggregationInfo
	DocCount *int64 `json:"doc_count,omitempty"`
	// Non nil for single value metric aggregations
	Value interface{} `json:"value,omitempty"`
	// Non nil for multi-value metrics aggregations
	Values interface{} `json:"values,omitempty"`
	// Map of sub aggregations with aggregationInfo names, includes both bucket
	// and metric aggregations
	SubAggregations aggregationsMap `json:"-"`
	// Non nil for multi-value bucket aggregations
	Buckets bucketsMap `json:"-"`
	// All other key-value pairs
	OtherValues map[string]interface{} `json:"-"`
}

// Required to handle certain fields specially
func (agg *aggregationResponse) UnmarshalJSON(b []byte) error {
	type aggregationResponseX aggregationResponse // prevent recursion
	var temp aggregationResponseX

	if err := json.Unmarshal(b, &temp); err != nil {
		return err
	}

	*agg = aggregationResponse(temp)
	agg.SubAggregations = aggregationsMap{}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	otherValues := map[string]interface{}{}
	for k, v := range m {

		switch k {
		// "value" is a special key in single value metric aggregations
		// and will be picked as a field on aggregationResponse
		case "value":
			continue
		// "values" is a special key in multi value metric aggregations
		// and will be picked as a field on aggregationResponse
		case "values":
			continue
		// Some aggregations return buckets as a map of "key" to the bucket
		// object whereas others return a list of buckets with the key embedded
		// inside the object. Handle these cases separately
		case "buckets":
			buckets := bucketsMap{}
			switch v := v.(type) {
			case map[string]interface{}:
				for bk, bv := range v {
					bucket, err := getBucketResponseFromInterface(bv)
					if err != nil {
						return err
					}

					bucket.Key = bk
					buckets[bk] = bucket
				}
			case []interface{}:
				for _, bv := range v {
					bucket, err := getBucketResponseFromInterface(bv)
					if err != nil {
						return err
					}

					buckets[bucket.Key] = bucket
				}
			}
			agg.Buckets = buckets
		default:
			// Absence of these "doc_count" and "buckets" fields is a good indicator that the aggregation
			// is not a bucket aggregation. If this metric aggregation does not happen to have the standard
			// "value" or "values" fields through which values are typically available put the fields into
			// OtherValues field of the struct
			if m["doc_count"] == nil && m["buckets"] == nil && m["value"] == nil && m["values"] == nil {
				otherValues[k] = v
				continue
			}

			_, ok := v.(map[string]interface{})

			if ok {
				subAgg, err := getAgrgeagtionResponseFromInterface(v)
				if err != nil {
					return err
				}

				agg.SubAggregations[k] = subAgg
			}
		}
	}

	agg.OtherValues = otherValues
	return nil
}

func getAgrgeagtionResponseFromInterface(aggregation interface{}) (*aggregationResponse, error) {
	b, err := json.Marshal(aggregation)
	if err != nil {
		return nil, err
	}

	var out aggregationResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// Represents a slice ("bucket") of ES documents
type bucketResponse struct {
	// Value uniquely identifying a bucket for a given bucket aggregationInfo
	Key interface{} `json:"key,omitempty"`
	// Number of documents in the bucket
	DocCount *int64 `json:"doc_count,omitempty"`
	// Map of sub aggregations with aggregationInfo names, includes both bucket
	// and metric aggregations
	SubAggregations aggregationsMap `json:"-"`
}

// Required to handle certain fields specially
func (bucket *bucketResponse) UnmarshalJSON(b []byte) error {
	type bucketResponseX bucketResponse // prevent recursion
	var temp bucketResponseX

	if err := json.Unmarshal(b, &temp); err != nil {
		return err
	}

	*bucket = bucketResponse(temp)
	bucket.SubAggregations = aggregationsMap{}

	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	for k, v := range m {
		_, ok := v.(map[string]interface{})
		if ok {
			subAgg, err := getAgrgeagtionResponseFromInterface(v)
			if err != nil {
				return err
			}

			bucket.SubAggregations[k] = subAgg
		}
	}

	return nil
}

func getBucketResponseFromInterface(bucket interface{}) (*bucketResponse, error) {
	b, err := json.Marshal(bucket)
	if err != nil {
		return nil, err
	}

	var out bucketResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}

	return &out, nil
}
