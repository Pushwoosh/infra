package redact

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	infrapostgres "github.com/pushwoosh/infra/postgres"
)

type sentryConfig struct {
	DSN string
}

type testConfig struct {
	Postgres  infrapostgres.ConnectionsConfig
	Sentry    sentryConfig
	TokenTTL  time.Duration
	Endpoints []string
	JWTSample string
	Opaque    string
	Brokers   string
	RequestID string
	Empty     struct{ Password string }
}

func render(t *testing.T, v any) string {
	t.Helper()

	out, err := json.Marshal(Value(v))
	if err != nil {
		t.Fatal(err)
	}

	return string(out)
}

func TestValue(t *testing.T) {
	cfg := testConfig{
		Postgres: infrapostgres.ConnectionsConfig{
			"main": {
				Address: "postgres-main:6432",
				Credentials: infrapostgres.Credentials{
					Database: "app", Username: "app", Password: "app",
				},
			},
			"messages": {
				Address:     "postgres-messages:6432",
				Credentials: infrapostgres.Credentials{Password: "pg-password-1234567"},
			},
		},
		Sentry:    sentryConfig{DSN: "https://abc123publickey@sentry.example.com/42"},
		TokenTTL:  time.Hour,
		Endpoints: []string{"postgres://u:inline-pass@db:5432/x", "http://plain.host:80/path?token=qwerty123"},
		JWTSample: "Bearer eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
		Opaque:    "q9fQ2xLmA7bZ4kR1tY8wE3pN6vC0jH5uDx_Zt",
		Brokers:   "kafka-0.kafka-headless.default.svc.cluster.local:9092,kafka-1.kafka-headless.default.svc.cluster.local:9092",
		RequestID: "3f2b8c1e-9a4d-4e7b-8c6f-1d2e3a4b5c6d",
	}

	got := render(t, cfg)

	for _, leak := range []string{
		"pg-password-1234567", "abc123publickey", "inline-pass", "qwerty123",
		"eyJhbGci", "q9fQ2xLmA7bZ4kR1tY8wE3pN6vC0jH5uDx_Zt",
	} {
		if strings.Contains(got, leak) {
			t.Errorf("secret %q leaked: %s", leak, got)
		}
	}

	for _, keep := range []string{
		`"Address":"postgres-main:6432"`, `"Address":"postgres-messages:6432"`,
		`"Database":"app"`, `"Username":"app"`, `"Password":"[redacted]"`, "sentry.example.com/42", `"TokenTTL":3600000000000`,
		"postgres://u:[redacted]@db:5432/x", "plain.host:80/path?token=[redacted]",
		cfg.Brokers, cfg.RequestID, `"Empty":{"Password":""}`,
	} {
		if !strings.Contains(got, keep) {
			t.Errorf("expected %q to survive: %s", keep, got)
		}
	}
}

func TestFingerprintIsStable(t *testing.T) {
	a := render(t, map[string]string{"password": "same-long-password-value"})
	b := render(t, map[string]string{"db_password": "same-long-password-value"})

	if !strings.Contains(a, "sha256:") || a[strings.Index(a, "sha256:"):] != b[strings.Index(b, "sha256:"):] {
		t.Fatalf("fingerprints differ: %s vs %s", a, b)
	}

	if short := render(t, map[string]string{"password": "short"}); short != `{"password":"[redacted]"}` {
		t.Fatalf("short secret must not be fingerprinted: %s", short)
	}
}

func TestUnmarshalable(t *testing.T) {
	if got := Value(map[string]any{"f": func() {}}); !strings.HasPrefix(got.(string), "unloggable") {
		t.Fatalf("got %v", got)
	}
}

func TestAuthCredentials(t *testing.T) {
	const apiToken = "5GjZOey3RVCiSoZCbrJ0NtqaTK5p78gQImfbw00AnR8PkDLz3AySmWZod3cLycWAuhU75wJ7iOP2lI7u"

	tests := map[string]struct {
		in   any
		keep []string
	}{
		"auth key in a map": {
			in:   map[string]any{"request": map[string]any{"application": "APP-1", "auth": "short-token"}},
			keep: []string{`"application":"APP-1"`, `"auth":"[redacted]"`},
		},
		"authorization header": {
			in:   map[string][]string{"Authorization": {"Token " + apiToken}, "Content-Type": {"application/json"}},
			keep: []string{"application/json"},
		},
		"auth scheme inside a string": {
			in:   "request failed, header Token " + apiToken,
			keep: []string{"request failed, header Token [redacted]"},
		},
		"json body logged as a string": {
			in:   `{"request":{"application":"APP-1","auth":"` + apiToken + `","content":"hi"}}`,
			keep: []string{`\"application\":\"APP-1\"`, `\"auth\":\"[redacted]\"`, `\"content\":\"hi\"`},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := render(t, tt.in)

			if strings.Contains(got, apiToken) || strings.Contains(got, "short-token") {
				t.Fatalf("secret leaked: %s", got)
			}

			for _, keep := range tt.keep {
				if !strings.Contains(got, keep) {
					t.Errorf("expected %q to survive: %s", keep, got)
				}
			}
		})
	}
}

func TestAuthLookalikesSurvive(t *testing.T) {
	got := render(t, map[string]string{
		"author":        "Jane",
		"oauthProvider": "google",
		"message":       "token expired, basic auth disabled",
		"body":          `{"password":"","user":"u"}`,
	})

	for _, keep := range []string{
		`"author":"Jane"`, `"oauthProvider":"google"`, "token expired, basic auth disabled", `\"password\":\"\"`,
	} {
		if !strings.Contains(got, keep) {
			t.Errorf("expected %q to survive: %s", keep, got)
		}
	}
}
