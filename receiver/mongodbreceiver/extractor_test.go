package mongodbreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func TestDigForValue(t *testing.T) {
	testCases := []struct {
		desc   string
		doc    bson.M
		path   []string
		expect interface{}
		nt     numberType
	}{
		{
			desc: "int-double",
			doc: bson.M{
				"value": 0.0,
			},
			path:   []string{"value"},
			expect: int64(0),
			nt:     integer,
		},
		{
			desc: "int-string",
			doc: bson.M{
				"value": "90",
			},
			path:   []string{"value"},
			expect: int64(90),
			nt:     integer,
		},
		{
			desc: "int-non-int64",
			doc: bson.M{
				"value": int(56),
			},
			path:   []string{"value"},
			expect: int64(56),
			nt:     integer,
		},
		{
			desc: "int-nonexistent-path",
			doc: bson.M{
				"value": int(56),
			},
			path:   []string{"not-a-path"},
			expect: 0,
			nt:     integer,
		},
		{
			desc: "double-string",
			doc: bson.M{
				"value": "65.2",
			},
			path:   []string{"value"},
			expect: float64(65.2),
			nt:     double,
		},
		{
			desc: "double-int",
			doc: bson.M{
				"value": 45,
			},
			path:   []string{"value"},
			expect: float64(45.0),
			nt:     double,
		},
		{
			desc: "double-float32",
			doc: bson.M{
				"value": float32(1000),
			},
			path:   []string{"value"},
			expect: float64(1000.0),
			nt:     double,
		},
		{
			desc: "double-nested",
			doc: bson.M{
				"value": bson.M{
					"nested": 1000.0,
				},
			},
			path:   []string{"value", "nested"},
			expect: float64(1000.0),
			nt:     double,
		},
		{
			desc: "double-nil",
			doc: bson.M{
				"value": float32(1000),
			},
			path:   []string{"what-path?"},
			expect: 0,
			nt:     double,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			var v interface{}
			var err error
			switch tc.nt {
			case integer:
				v, err = digForIntValue(tc.doc, tc.path)
				if tc.expect == 0 {
					require.Error(t, err)
				} else {
					require.IsType(t, v, int64(0))
					require.Equal(t, v, tc.expect)
				}
			case double:
				v, err = digForDoubleValue(tc.doc, tc.path)
				if tc.expect == 0 {
					require.Error(t, err)
				} else {
					require.IsType(t, v, float64(0))
					require.Equal(t, v, tc.expect)
				}
			}

		})
	}
}
