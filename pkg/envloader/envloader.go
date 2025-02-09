package envloader

import (
	"os"
	"strings"
)

func LoadEnv(path string) error {
	// Read the entire file in one call.
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Convert the file contents to a string and split into lines.
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		// Trim spaces and skip blank or comment lines.
		line = strings.TrimSpace(line)
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		// Split into key and value using the first '=' as delimiter.
		i := strings.IndexByte(line, '=')
		if i < 0 {
			continue
		}
		key := strings.TrimSpace(line[:i])
		value := strings.TrimSpace(line[i+1:])

		// If the value is quoted (with matching quotes), remove them.
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}

		// Set the environment variable.
		os.Setenv(key, value)
	}
	return nil
}
