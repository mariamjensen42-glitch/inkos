package llm

import "os"

// getenvDefault returns the value of an environment variable, or "".
func getenvDefault(key string) string {
	return os.Getenv(key)
}
