package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSSLModeFrom(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
		want string
	}{
		{"keyword form", "host=db user=nimbus password=pw dbname=nimbus port=5432 sslmode=require", "require"},
		{"keyword form disable", "host=db user=nimbus password=pw sslmode=disable", "disable"},
		{"keyword form absent", "host=db user=nimbus password=pw dbname=nimbus", ""},
		{"keyword form quoted", "host=db sslmode='verify-full'", "verify-full"},
		{"url form", "postgres://nimbus:pw@db:5432/nimbus?sslmode=verify-ca", "verify-ca"},
		{"url form postgresql scheme", "postgresql://nimbus:pw@db:5432/nimbus?sslmode=require", "require"},
		{"url form absent", "postgres://nimbus:pw@db:5432/nimbus", ""},
		{"unparseable url", "postgres://nimbus:pw@db:invalid_port/nimbus?sslmode=require", ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, sslModeFrom(tc.dsn))
		})
	}
}

func TestCheckTLS_LocalDevExemptsEverything(t *testing.T) {
	// docker-compose runs Postgres with no certificate, so its DSN disables TLS.
	assert.NoError(t, checkTLS("host=postgres user=nimbus password=nimbus sslmode=disable", true))
	assert.NoError(t, checkTLS("host=postgres user=nimbus", true))
}

func TestCheckTLS_AcceptsEncryptingModes(t *testing.T) {
	for _, mode := range []string{"require", "verify-ca", "verify-full", "REQUIRE", "Verify-Full"} {
		t.Run(mode, func(t *testing.T) {
			assert.NoError(t, checkTLS("host=db sslmode="+mode, false))
		})
	}
}

func TestCheckTLS_RejectsPlaintextCapableModes(t *testing.T) {
	// "prefer" and "allow" are the dangerous pair: both connect happily without
	// encryption when the server does not insist on it.
	for _, mode := range []string{"disable", "prefer", "allow"} {
		t.Run(mode, func(t *testing.T) {
			err := checkTLS("host=db sslmode="+mode, false)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), mode)
		})
	}
}

func TestCheckTLS_RejectsMissingSSLMode(t *testing.T) {
	// The core guarantee: an unset sslmode outside LOCAL_DEV is refused rather
	// than left to libpq's "prefer" default.
	err := checkTLS("host=db user=nimbus password=pw dbname=nimbus", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no sslmode")
}

func TestCheckTLS_ErrorNeverLeaksTheDSN(t *testing.T) {
	// The DSN carries the master password; only the mode may appear in the error.
	const password = "sup3r-s3cret-pw"
	err := checkTLS("host=db user=nimbus password="+password+" sslmode=disable", false)
	assert.Error(t, err)
	assert.NotContains(t, err.Error(), password)
}
