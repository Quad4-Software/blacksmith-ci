package migration

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBaselineSchema_ForeignKeyTypeConsistency parses each baseline schema SQL
// file and verifies that every FOREIGN KEY column type matches the referenced
// table's primary key type.  MySQL requires exact type matches for FK
// constraints; this test catches mismatches (e.g. INTEGER referencing BIGINT)
// at development time, before they reach CI or production.
func TestBaselineSchema_ForeignKeyTypeConsistency(t *testing.T) {
	schemas := []struct {
		file   string
		driver string
	}{
		{"baseline_schema_mysql.sql", "mysql"},
		{"baseline_schema_postgres.sql", "postgres"},
	}

	for _, s := range schemas {
		t.Run(s.driver, func(t *testing.T) {
			data, err := os.ReadFile(s.file)
			require.NoError(t, err)

			tables := parseCreateTables(string(data))
			pkTypes := extractPKTypes(tables)

			for tableName, body := range tables {
				checkForeignKeys(t, s.driver, tableName, body, pkTypes)
			}
		})
	}
}

// TestBaselineSchema_MigrationDDL_ForeignKeyTypes verifies that the
// driver-specific DDL in migration functions uses the correct FK column types.
// This catches the exact bug from issue #945: the migration used INTEGER for
// secret_id but secrets.id is BIGINT in MySQL.
func TestBaselineSchema_MigrationDDL_ForeignKeyTypes(t *testing.T) {
	data, err := os.ReadFile("migration.go")
	require.NoError(t, err)
	src := string(data)

	// Load PK types from each baseline schema so we can cross-reference.
	mysqlSchema, err := os.ReadFile("baseline_schema_mysql.sql")
	require.NoError(t, err)
	mysqlPKs := extractPKTypes(parseCreateTables(string(mysqlSchema)))

	pgSchema, err := os.ReadFile("baseline_schema_postgres.sql")
	require.NoError(t, err)
	pgPKs := extractPKTypes(parseCreateTables(string(pgSchema)))

	// Find all inline DDL strings in migration.go and check FK references.
	// Match backtick-delimited DDL strings (CREATE TABLE ...).
	ddlRe := regexp.MustCompile("(?s)`(CREATE TABLE[^`]+)`")
	matches := ddlRe.FindAllStringSubmatch(src, -1)

	for _, m := range matches {
		ddl := m[1]
		tables := parseCreateTables(ddl)

		for tableName, body := range tables {
			// Determine which driver this DDL is for by looking at syntax.
			switch {
			case strings.Contains(body, "AUTO_INCREMENT"):
				t.Run("mysql/migration/"+tableName, func(t *testing.T) {
					checkForeignKeys(t, "mysql", tableName, body, mysqlPKs)
				})
			case strings.Contains(body, "BIGSERIAL") || strings.Contains(body, "bigserial"):
				t.Run("postgres/migration/"+tableName, func(t *testing.T) {
					checkForeignKeys(t, "postgres", tableName, body, pgPKs)
				})
			}
		}
	}
}

// parseCreateTables extracts table name → body from CREATE TABLE statements.
func parseCreateTables(sql string) map[string]string {
	re := regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)\s*\((.+?)\);`)
	matches := re.FindAllStringSubmatch(sql, -1)
	result := make(map[string]string, len(matches))
	for _, m := range matches {
		result[strings.ToLower(m[1])] = m[2]
	}
	return result
}

// extractPKTypes returns a map of table name → normalized PK type.
func extractPKTypes(tables map[string]string) map[string]string {
	result := make(map[string]string, len(tables))
	pkRe := regexp.MustCompile(`(?i)id\s+(BIGINT|BIGSERIAL|INTEGER|INT)\b`)
	for name, body := range tables {
		if m := pkRe.FindStringSubmatch(body); m != nil {
			result[name] = normalizeSQLType(m[1])
		}
	}
	return result
}

// normalizeSQLType maps SQL types to a canonical form for comparison.
func normalizeSQLType(t string) string {
	switch strings.ToUpper(t) {
	case "BIGINT", "BIGSERIAL":
		return "BIGINT"
	case "INTEGER", "INT":
		return "INTEGER"
	default:
		return strings.ToUpper(t)
	}
}

// checkForeignKeys finds FK constraints in a table body and verifies the
// column type matches the referenced table's PK type.
func checkForeignKeys(t *testing.T, driver, tableName, body string, pkTypes map[string]string) {
	t.Helper()

	// Match explicit FOREIGN KEY (col) REFERENCES table(id) constraints.
	explicitFK := regexp.MustCompile(`(?i)FOREIGN\s+KEY\s*\((\w+)\)\s*REFERENCES\s+(\w+)\s*\((\w+)\)`)
	for _, m := range explicitFK.FindAllStringSubmatch(body, -1) {
		col, refTable := m[1], strings.ToLower(m[2])
		colType := findColumnType(body, col)
		refPKType, ok := pkTypes[refTable]
		if !ok {
			continue // referenced table not in this schema file
		}
		assert.Equalf(t, refPKType, normalizeSQLType(colType),
			"%s: %s.%s (%s) references %s.%s (%s) — FK column type must match PK type",
			driver, tableName, col, colType, refTable, m[3], refPKType)
	}

	// Match inline REFERENCES table(id) constraints (PostgreSQL/SQLite style).
	inlineRef := regexp.MustCompile(`(?i)(\w+)\s+(BIGINT|BIGSERIAL|INTEGER|INT)\b[^,]*REFERENCES\s+(\w+)\s*\((\w+)\)`)
	for _, m := range inlineRef.FindAllStringSubmatch(body, -1) {
		col, colType, refTable := m[1], m[2], strings.ToLower(m[3])
		refPKType, ok := pkTypes[refTable]
		if !ok {
			continue
		}
		assert.Equalf(t, refPKType, normalizeSQLType(colType),
			"%s: %s.%s (%s) references %s.%s (%s) — FK column type must match PK type",
			driver, tableName, col, colType, refTable, m[4], refPKType)
	}
}

// findColumnType extracts the SQL type for a given column name from a table body.
func findColumnType(body, colName string) string {
	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(colName) + `\s+(BIGINT|BIGSERIAL|INTEGER|INT)\b`)
	if m := re.FindStringSubmatch(body); m != nil {
		return m[1]
	}
	return ""
}
