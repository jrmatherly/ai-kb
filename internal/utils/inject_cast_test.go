package utils

import "testing"

func TestValidateSQLCasts(t *testing.T) {
	opts := []SQLValidationOption{
		WithAllowedTables("data"), WithSelectOnly(), WithSingleStatement(), WithNoDangerousFunctions(),
	}
	cases := map[string]bool{
		`SELECT SUM(CAST(b AS DOUBLE)) FROM data`:           true,
		`SELECT SUM(CAST(b AS INTEGER)) FROM data`:          true,
		`SELECT SUM(b::numeric) FROM data`:                  true,
		`SELECT CAST(b AS DOUBLE PRECISION) FROM data`:      true,
		`SELECT CAST(b AS REAL), CAST(c AS DATE) FROM data`: true,
		`SELECT CAST(b AS regclass) FROM data`:              false,
		`SELECT b::pg_catalog.regproc FROM data`:            false,
		`SELECT CAST(b AS pg_lsn) FROM data`:                false,
	}
	for sql, want := range cases {
		_, result := ValidateSQL(sql, opts...)
		if result.Valid != want {
			t.Errorf("ValidateSQL(%q).Valid = %v, want %v (errors: %v)", sql, result.Valid, want, result.Errors)
		}
	}
}
