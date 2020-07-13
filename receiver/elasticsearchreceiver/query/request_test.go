package query

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestGetAggregationsMetaSimpleQuery(t *testing.T) {
	query := `{
				"query" : {
					"match_all" : {}
				},
				"aggs" : {
					"host" : {
						"terms" : {
							"field":"host"
						},
						"aggs": {
							"metric_agg_1": {
								"extended_stats": {
									"field": "cpu_utilization"
								}
							},
							"metric_agg_2": {
								"percentiles": {
									"field": "cpu_utilization"
								}
							},
							"metric_agg_3": {
								"value_count": {
									"field": "cpu_utilization"
								}
							}
						}
					}
				}
			}`

	var reqBody ElasticsearchQueryBody
	if err := json.Unmarshal([]byte(query), &reqBody); err != nil {
		t.Error(err)
	}

	actual, err := reqBody.AggregationsMeta()
	if err != nil {
		t.Error(err)
	}

	expected := map[string]*AggregationMeta{
		"host": {
			Type: "terms",
		},
		"metric_agg_1": {
			Type: "extended_stats",
		},
		"metric_agg_2": {
			Type: "percentiles",
		},
		"metric_agg_3": {
			Type: "value_count",
		},
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Expected: %v, Actual: %v", printAggregationsMeta(expected), printAggregationsMeta(actual))
	}
}

func TestGetAggregationsMetaComplexQuery_terms(t *testing.T) {
	query :=
		`{ 
   "query":{ 
      "match_all":{ 

      }
   },
   "aggs":{ 
      "servers":{ 
         "nested":{ 
            "path":"server"
         },
         "aggs":{ 
            "host":{ 
               "filters":{ 
                  "filters":{ 
                     "helsinki":{ 
                        "match":{ 
                           "server.host":"helsinki"
                        }
                     },
                     "nairobi":{ 
                        "match":{ 
                           "server.host":"nairobi"
                        }
                     }
                  }
               },
               "aggs":{ 
                  "temp_tp":{ 
                     "reverse_nested":{ 

                     },
                     "aggs":{ 
                        "cpus":{ 
                           "nested":{ 
                              "path":"cpu"
                           },
                           "aggs":{ 
                              "metric_agg_1":{ 
                                 "extended_stats":{ 
                                    "field":"cpu.utilization"
                                 }
                              },
                              "metric_agg_2":{ 
                                 "percentiles":{ 
                                    "field":"cpu.utilization"
                                 }
                              },
                              "metric_agg_3":{ 
                                 "stats":{ 
                                    "field":"cpu.utilization"
                                 }
                              }
                           }
                        }
                     }
                  }
               }
            }
         }
      }
   }
}`

	var reqBody ElasticsearchQueryBody
	if err := json.Unmarshal([]byte(query), &reqBody); err != nil {
		t.Error(err)
	}

	actual, err := reqBody.AggregationsMeta()
	if err != nil {
		t.Error(err)
	}

	expected := map[string]*AggregationMeta{
		"servers": {
			Type: "nested",
		},
		"host": {
			Type: "filters",
		},
		"temp_tp": {
			Type: "reverse_nested",
		},
		"cpus": {
			Type: "nested",
		},
		"metric_agg_1": {
			Type: "extended_stats",
		},
		"metric_agg_2": {
			Type: "percentiles",
		},
		"metric_agg_3": {
			Type: "stats",
		},
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Expected: %v, Actual: %v", printAggregationsMeta(expected), printAggregationsMeta(actual))
	}
}

func TestGetAggregationsMetaComplexQuery_filters(t *testing.T) {
	query :=
		`{ 
   "query":{ 
      "match_all":{ 

      }
   },
   "aggs":{ 
      "servers":{ 
         "nested":{ 
            "path":"server"
         },
         "aggs":{ 
            "host":{ 
      			"terms" : { 
        			"field":"host"
      			},
               "aggs":{ 
                  "temp_tp":{ 
                     "reverse_nested":{ 

                     },
                     "aggs":{ 
                        "cpus":{ 
                           "nested":{ 
                              "path":"cpu"
                           },
                           "aggs":{ 
                              "metric_agg_1":{ 
                                 "extended_stats":{ 
                                    "field":"cpu.utilization"
                                 }
                              },
                              "metric_agg_2":{ 
                                 "percentiles":{ 
                                    "field":"cpu.utilization"
                                 }
                              },
                              "metric_agg_3":{ 
                                 "stats":{ 
                                    "field":"cpu.utilization"
                                 }
                              }
                           }
                        }
                     }
                  }
               }
            }
         }
      }
   }
}`

	var reqBody ElasticsearchQueryBody
	if err := json.Unmarshal([]byte(query), &reqBody); err != nil {
		t.Error(err)
	}

	actual, err := reqBody.AggregationsMeta()
	if err != nil {
		t.Error(err)
	}

	expected := map[string]*AggregationMeta{
		"servers": {
			Type: "nested",
		},
		"host": {
			Type: "terms",
		},
		"temp_tp": {
			Type: "reverse_nested",
		},
		"cpus": {
			Type: "nested",
		},
		"metric_agg_1": {
			Type: "extended_stats",
		},
		"metric_agg_2": {
			Type: "percentiles",
		},
		"metric_agg_3": {
			Type: "stats",
		},
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("Expected: %v, Actual: %v", printAggregationsMeta(expected), printAggregationsMeta(actual))
	}
}

func printAggregationsMeta(am map[string]*AggregationMeta) string {
	var out []string
	for k := range am {
		temp := fmt.Sprintf("%s:%s", k, am[k].Type)
		out = append(out, temp)
	}
	return strings.Join(out, ",")
}
