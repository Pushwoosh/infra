// Package redact masks secrets in values that are logged, guessing them by key names and value shapes.
package redact

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"

	"go.uber.org/zap"
)

const (
	mask = "[redacted]"

	// Fingerprints of short values could be brute-forced offline, so those get a bare mask.
	minFingerprintLen = 16

	minTokenLen     = 32
	minTokenEntropy = 4.3
	tokenPunct      = "-_+/="
)

var (
	secretKeyParts = []string{
		"password", "passwd", "passphrase", "secret", "token", "apikey", "accesskey",
		"privatekey", "signingkey", "salt", "cookie", "bearer",
	}

	pemBlock    = regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`)
	jwt         = regexp.MustCompile(`eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`)
	urlUserinfo = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.-]*://)([^/@\s:]*)(:[^/@\s]*)?@`)
	kvSecret    = regexp.MustCompile(`(?i)\b(password|passwd|pwd|secret|token|api[_-]?key|access[_-]?key)=([^&\s;]+)`)
)

// Field returns a zap field that logs v as JSON with secrets masked.
func Field(key string, v any) zap.Field {
	return zap.Any(key, Value(v))
}

// Value returns a JSON-shaped copy of v with secrets masked; it is meant for one-off startup logging.
func Value(v any) any {
	raw, err := json.Marshal(v)
	if err != nil {
		return "unloggable: " + err.Error()
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()

	var tree any
	if err := dec.Decode(&tree); err != nil {
		return "unloggable: " + err.Error()
	}

	return walk(tree, false)
}

func isSecretKey(key string) bool {
	k := strings.ToLower(key)
	k = strings.NewReplacer("_", "", "-", "", ".", "", " ", "").Replace(k)

	for _, part := range secretKeyParts {
		if strings.Contains(k, part) {
			return true
		}
	}

	return false
}

func walk(node any, secret bool) any {
	switch n := node.(type) {
	case map[string]any:
		out := make(map[string]any, len(n))
		for k, v := range n {
			out[k] = walk(v, secret || isSecretKey(k))
		}

		return out
	case []any:
		out := make([]any, len(n))
		for i, v := range n {
			out[i] = walk(v, secret)
		}

		return out
	case string:
		// An empty secret stays visible: a missing password is exactly what one looks for in the startup log.
		if secret && n != "" {
			return fingerprint(n)
		}

		return scrub(n)
	default:
		return node
	}
}

func scrub(s string) string {
	s = pemBlock.ReplaceAllString(s, mask)
	s = jwt.ReplaceAllString(s, mask)
	s = kvSecret.ReplaceAllString(s, "$1="+mask)
	s = urlUserinfo.ReplaceAllStringFunc(s, func(m string) string {
		p := urlUserinfo.FindStringSubmatch(m)
		if p[3] != "" {
			return p[1] + p[2] + ":" + mask + "@"
		}

		// Without a password the user part is the key itself, as in Sentry DSNs.
		return p[1] + mask + "@"
	})

	if looksLikeToken(s) {
		return fingerprint(s)
	}

	return s
}

func looksLikeToken(s string) bool {
	if len(s) < minTokenLen {
		return false
	}

	var letters, digits bool
	for _, r := range s {
		switch {
		case unicode.IsLetter(r):
			letters = true
		case unicode.IsDigit(r):
			digits = true
		case !strings.ContainsRune(tokenPunct, r):
			// Dots, colons or @ mean a URL or an address, not a token.
			return false
		}
	}

	return letters && digits && entropy(s) >= minTokenEntropy
}

func entropy(s string) float64 {
	counts := map[rune]int{}
	for _, r := range s {
		counts[r]++
	}

	var h float64
	n := float64(len([]rune(s)))
	for _, c := range counts {
		p := float64(c) / n
		h -= p * math.Log2(p)
	}

	return h
}

func fingerprint(s string) string {
	if len(s) < minFingerprintLen {
		return mask
	}

	sum := sha256.Sum256([]byte(s))

	return fmt.Sprintf("[redacted sha256:%x]", sum[:4])
}
