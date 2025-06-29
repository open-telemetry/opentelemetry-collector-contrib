// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestObfuscateSQL(t *testing.T) {
	expected := `SELECT e.employee_id, e.first_name, e.last_name, e.department_id, s.salary, d.department_name FROM employees e INNER JOIN ( SELECT department_id FROM employees GROUP BY department_id HAVING COUNT ( employee_id ) > ? ) ON e.department_id = subquery.department_id INNER JOIN salaries s ON e.employee_id = s.employee_id INNER JOIN departments d ON e.department_id = d.department_id WHERE s.salary > ? AND d.department_name LIKE ? ORDER BY e.salary DESC`

	origin := `SELECT e.employee_id, e.first_name, e.last_name, e.department_id, s.salary, d.department_name
                 FROM employees e
                 INNER JOIN
                    ( SELECT department_id FROM employees GROUP BY department_id HAVING COUNT(employee_id) > 10) AS subquery
                      ON e.department_id = subquery.department_id
                 INNER JOIN
                     salaries s ON e.employee_id = s.employee_id
                 INNER JOIN
                    departments d ON e.department_id = d.department_id
                 WHERE s.salary > 50000
                    AND d.department_name LIKE 'IT%'
                 ORDER BY e.salary DESC;`
	result, err := ObfuscateSQL(origin)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}
