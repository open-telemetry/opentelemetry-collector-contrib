package postgresqlreceiver

import "fmt"

var _ client = (*fakeClient)(nil)

type fakeClient struct {
	database  string
	databases []string
}

func (c *fakeClient) Close() error {
	return nil
}

func (c *fakeClient) listDatabases() ([]string, error) {
	return c.databases, nil
}

func (c *fakeClient) getCommitsAndRollbacks(databases []string) ([]MetricStat, error) {
	metrics := []MetricStat{}
	for idx, db := range databases {
		metrics = append(metrics, MetricStat{
			database: db,
			stats: map[string]string{
				"xact_commit":   fmt.Sprintf("%d", idx+1),
				"xact_rollback": fmt.Sprintf("%d", idx+2),
			},
		})
	}

	return metrics, nil
}

func (c *fakeClient) getBackends(databases []string) ([]MetricStat, error) {
	metrics := []MetricStat{}
	for idx, db := range databases {
		metrics = append(metrics, MetricStat{
			database: db,
			stats:    map[string]string{"count": fmt.Sprintf("%d", idx+3)},
		})
	}

	return metrics, nil
}

func (c *fakeClient) getDatabaseSize(databases []string) ([]MetricStat, error) {
	metrics := []MetricStat{}
	for idx, db := range databases {
		metrics = append(metrics, MetricStat{
			database: db,
			stats:    map[string]string{"db_size": fmt.Sprintf("%d", idx+4)},
		})
	}

	return metrics, nil
}

func (c *fakeClient) getDatabaseTableMetrics() ([]MetricStat, error) {
	idx := 0
	for i, db := range c.databases {
		if db == c.database {
			idx = i
			break
		}
	}
	metrics := []MetricStat{}
	metrics = append(metrics, MetricStat{
		database: c.database,
		table:    "public.table1",
		stats: map[string]string{
			"live":    fmt.Sprintf("%d", idx+7),
			"dead":    fmt.Sprintf("%d", idx+8),
			"ins":     fmt.Sprintf("%d", idx+39),
			"upd":     fmt.Sprintf("%d", idx+40),
			"del":     fmt.Sprintf("%d", idx+41),
			"hot_upd": fmt.Sprintf("%d", idx+42),
		},
	})

	metrics = append(metrics, MetricStat{
		database: c.database,
		table:    "public.table2",
		stats: map[string]string{
			"live":    fmt.Sprintf("%d", idx+9),
			"dead":    fmt.Sprintf("%d", idx+10),
			"ins":     fmt.Sprintf("%d", idx+43),
			"upd":     fmt.Sprintf("%d", idx+44),
			"del":     fmt.Sprintf("%d", idx+45),
			"hot_upd": fmt.Sprintf("%d", idx+46),
		},
	})

	return metrics, nil
}

func (c *fakeClient) getBlocksReadByTable() ([]MetricStat, error) {
	idx := 0
	for i, db := range c.databases {
		if db == c.database {
			idx = i
			break
		}
	}
	metrics := []MetricStat{}
	metrics = append(metrics, MetricStat{
		database: c.database,
		table:    "public.table1",
		stats: map[string]string{
			"heap_read":  fmt.Sprintf("%d", idx+19),
			"heap_hit":   fmt.Sprintf("%d", idx+20),
			"idx_read":   fmt.Sprintf("%d", idx+21),
			"idx_hit":    fmt.Sprintf("%d", idx+22),
			"toast_read": fmt.Sprintf("%d", idx+23),
			"toast_hit":  fmt.Sprintf("%d", idx+24),
			"tidx_read":  fmt.Sprintf("%d", idx+25),
			"tidx_hit":   fmt.Sprintf("%d", idx+26),
		},
	})

	metrics = append(metrics, MetricStat{
		database: c.database,
		table:    "public.table2",
		stats: map[string]string{
			"heap_read":  fmt.Sprintf("%d", idx+27),
			"heap_hit":   fmt.Sprintf("%d", idx+28),
			"idx_read":   fmt.Sprintf("%d", idx+29),
			"idx_hit":    fmt.Sprintf("%d", idx+30),
			"toast_read": fmt.Sprintf("%d", idx+31),
			"toast_hit":  fmt.Sprintf("%d", idx+32),
			"tidx_read":  fmt.Sprintf("%d", idx+33),
			"tidx_hit":   fmt.Sprintf("%d", idx+34),
		},
	})

	return metrics, nil
}
