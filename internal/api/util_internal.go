package api

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"

	"github.com/narcooo/inkos/internal/model"
)

// getenv returns the value of an environment variable, or "".
func getenv(k string) string { return os.Getenv(k) }

// jsonRoundTrip round-trips a value through JSON. Used to normalize maps
// (e.g. when overlaying a partial patch).
func jsonRoundTrip(v interface{}) (interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// urlPathToRelative decodes the URL-encoded path and strips leading slashes.
func urlPathToRelative(raw string) (string, error) {
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", err
	}
	decoded = strings.TrimLeft(decoded, "/")
	return decoded, nil
}

// notifyFromMap converts a generic map into a NotifyChannel.
func notifyFromMap(m map[string]interface{}) model.NotifyChannel {
	out := model.NotifyChannel{}
	if v, ok := m["type"].(string); ok {
		out.Type = v
	}
	if v, ok := m["enabled"].(bool); ok {
		out.Enabled = v
	}
	if v, ok := m["config"].(map[string]interface{}); ok {
		out.Config = v
	}
	return out
}
