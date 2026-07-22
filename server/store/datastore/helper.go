// Copyright 2021 Woodpecker Authors
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

package datastore

import (
	"database/sql"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/uptrace/bun"

	"codefloe.com/crowci/crow/v6/server/model"
	"codefloe.com/crowci/crow/v6/server/store/types"
)

// castAsText returns a SQL expression that casts expr to a text type,
// using the correct syntax for the current database dialect.
func (s storage) castAsText(expr string) string {
	if s.driver == DriverMysql {
		return "CAST(" + expr + " AS CHAR)"
	}
	return "CAST(" + expr + " AS TEXT)"
}

// castAsInteger returns a SQL expression that casts expr to an integer type,
// using the correct syntax for the current database dialect.
func (s storage) castAsInteger(expr string) string {
	if s.driver == DriverMysql {
		return "CAST(" + expr + " AS UNSIGNED)"
	}
	return "CAST(" + expr + " AS INTEGER)"
}

// bucketExpr returns a dialect-specific SQL expression that maps a unix-seconds
// column to a lexicographically-sortable bucket label (e.g. "2026-07-22",
// "2026-W29" or "2026-07"). Used by the metrics chart-builder time dimension.
// All bucketing is UTC. `col` is a trusted column reference, never user input.
func (s storage) bucketExpr(col, bucket string) string {
	switch s.driver {
	case DriverPostgres:
		switch bucket {
		case model.MetricBucketWeek:
			return `to_char(to_timestamp(` + col + `), 'IYYY-"W"IW')`
		case model.MetricBucketMonth:
			return `to_char(to_timestamp(` + col + `), 'YYYY-MM')`
		default:
			return `to_char(to_timestamp(` + col + `), 'YYYY-MM-DD')`
		}
	case DriverMysql:
		switch bucket {
		case model.MetricBucketWeek:
			return `DATE_FORMAT(FROM_UNIXTIME(` + col + `), '%x-W%v')`
		case model.MetricBucketMonth:
			return `DATE_FORMAT(FROM_UNIXTIME(` + col + `), '%Y-%m')`
		default:
			return `DATE_FORMAT(FROM_UNIXTIME(` + col + `), '%Y-%m-%d')`
		}
	default: // sqlite
		switch bucket {
		case model.MetricBucketWeek:
			return `strftime('%Y-W%W', ` + col + `, 'unixepoch')`
		case model.MetricBucketMonth:
			return `strftime('%Y-%m', ` + col + `, 'unixepoch')`
		default:
			return `strftime('%Y-%m-%d', ` + col + `, 'unixepoch')`
		}
	}
}

// wrapGet returns types.RecordNotExist if the error is sql.ErrNoRows,
// otherwise returns the error as-is.
func wrapGet(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return types.RecordNotExist
	}
	fnName := callerName(2)
	return fmt.Errorf("%s: %w", fnName, err)
}

// wrapDelete returns types.RecordNotExist if no rows were affected,
// otherwise returns any error.
func wrapDelete(res sql.Result, err error) error {
	if err != nil {
		fnName := callerName(2)
		return fmt.Errorf("%s: %w", fnName, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return types.RecordNotExist
	}
	return nil
}

// paginate applies limit/offset pagination to a Bun select query.
func (s storage) paginate(q *bun.SelectQuery, p *model.ListOptions) *bun.SelectQuery {
	if p == nil || p.All {
		return q
	}
	if p.PerPage < 1 {
		p.PerPage = 1
	}
	if p.Page < 1 {
		p.Page = 1
	}
	return q.Limit(p.PerPage).Offset(p.PerPage * (p.Page - 1))
}

func callerName(skip int) string {
	pc, _, _, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}
	fnName := runtime.FuncForPC(pc).Name()
	pIndex := strings.LastIndex(fnName, ".")
	if pIndex != -1 {
		fnName = fnName[pIndex+1:]
	}
	return fnName
}
