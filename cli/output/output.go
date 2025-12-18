package output

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/goccy/go-yaml"
)

var ErrOutputOptionRequired = errors.New("output option required")

func ParseOutputOptions(out string) (string, []string) {
	out, opt, found := strings.Cut(out, "=")

	if !found {
		return out, nil
	}

	var optList []string

	if opt != "" {
		optList = strings.Split(opt, ",")
	}

	return out, optList
}

// WriteJSON writes the given data as pretty-printed JSON to the writer.
func WriteJSON(out io.Writer, data any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// WriteYAML writes the given data as YAML to the writer.
func WriteYAML(out io.Writer, data any) error {
	enc := yaml.NewEncoder(out, yaml.Indent(2)) //nolint:mnd
	return enc.Encode(data)
}
