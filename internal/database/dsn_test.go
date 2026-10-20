package database

import (
	"net/url"
	"testing"
)

func setDBEnv(t *testing.T, sslMode string) {
	t.Setenv("DB_HOST", "db.example")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "aikb")
	t.Setenv("DB_PASSWORD", "p@ss word!")
	t.Setenv("DB_NAME", "ai_kb")
	t.Setenv("DB_SSLMODE", sslMode)
}

func TestPostgresDSNs(t *testing.T) {
	const gormBase = "host=db.example port=5432 user=aikb password='p@ss word!' dbname=ai_kb"
	cases := []struct {
		env, gorm, migrate string
	}{
		{
			env:     "",
			gorm:    gormBase + " sslmode=disable TimeZone=UTC",
			migrate: "postgres://aikb:p%40ss%20word%21@db.example:5432/ai_kb?sslmode=disable",
		},
		{
			env:     "require",
			gorm:    gormBase + " sslmode=require TimeZone=UTC",
			migrate: "postgres://aikb:p%40ss%20word%21@db.example:5432/ai_kb?sslmode=require",
		},
	}
	for _, c := range cases {
		t.Run("DB_SSLMODE="+c.env, func(t *testing.T) {
			setDBEnv(t, c.env)
			mode, err := SSLMode()
			if err != nil {
				t.Fatalf("SSLMode() error: %v", err)
			}
			if got := PostgresGormDSN(mode); got != c.gorm {
				t.Errorf("gorm DSN = %q, want %q", got, c.gorm)
			}
			if got := PostgresMigrateURL(mode); got != c.migrate {
				t.Errorf("migrate URL = %q, want %q", got, c.migrate)
			}
		})
	}
}

func TestSSLModeRejectsUnknownValues(t *testing.T) {
	for _, mode := range []string{"requre", "prefer", "allow"} {
		t.Setenv("DB_SSLMODE", mode)
		if _, err := SSLMode(); err == nil {
			t.Errorf("SSLMode() accepted %q", mode)
		}
	}
}

func TestPostgresMigrateURLRoundTripsThePassword(t *testing.T) {
	setDBEnv(t, "")
	u, err := url.Parse(PostgresMigrateURL("disable"))
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := u.User.Password(); got != "p@ss word!" {
		t.Errorf("password = %q", got)
	}
}
