// Package envfile loads a small KEY=VALUE configuration file without
// overwriting process environment variables. Controller Node uses it so the
// same binary can be installed as a Windows task or a Linux service without a
// shell wrapper that would expose secrets in a command line.
package envfile

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// LoadIntoEnvironment reads path when it exists. Values already present in the
// process environment win, which keeps Docker/Kubernetes deployment behavior
// unchanged. A missing optional file is not an error.
func LoadIntoEnvironment(path string) error {
	values, err := Read(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for key, value := range values {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set %s: %w", key, err)
		}
	}
	return nil
}

func Read(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("%s:%d must be KEY=VALUE", path, lineNumber)
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), "\"")
	}
	return values, scanner.Err()
}
