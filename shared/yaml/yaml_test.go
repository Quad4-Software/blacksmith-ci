// Copyright 2025 Crow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package yaml

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnmarshal(t *testing.T) {
	t.Run("simple mapping", func(t *testing.T) {
		data := `
name: test
value: 123
`
		var result map[string]any
		err := Unmarshal([]byte(data), &result)
		assert.NoError(t, err)
		assert.Equal(t, "test", result["name"])
		assert.Equal(t, uint64(123), result["value"])
	})

	t.Run("simple sequence", func(t *testing.T) {
		data := `
items:
  - one
  - two
  - three
`
		var result map[string][]string
		err := Unmarshal([]byte(data), &result)
		assert.NoError(t, err)
		assert.Equal(t, []string{"one", "two", "three"}, result["items"])
	})
}

func TestSequenceMerge(t *testing.T) {
	t.Run("single sequence merge", func(t *testing.T) {
		data := `
base: &base
  - item1
  - item2

merged:
  - <<: *base
  - item3
`
		var result map[string][]string
		err := Unmarshal([]byte(data), &result)
		assert.NoError(t, err)
		assert.Equal(t, []string{"item1", "item2"}, result["base"])
		assert.Equal(t, []string{"item1", "item2", "item3"}, result["merged"])
	})

	t.Run("multiple sequence merges", func(t *testing.T) {
		data := `
first: &first
  - a
  - b

second: &second
  - x
  - y

combined:
  - <<: *first
  - middle
  - <<: *second
  - end
`
		var result map[string][]string
		err := Unmarshal([]byte(data), &result)
		assert.NoError(t, err)
		assert.Equal(t, []string{"a", "b", "middle", "x", "y", "end"}, result["combined"])
	})

	t.Run("nested sequence merge", func(t *testing.T) {
		data := `
commands: &cmds
  - go version
  - go mod download

steps:
  build:
    image: golang
    commands:
      - <<: *cmds
      - go build
`
		var result struct {
			Steps map[string]struct {
				Image    string   `yaml:"image"`
				Commands []string `yaml:"commands"`
			} `yaml:"steps"`
		}
		err := Unmarshal([]byte(data), &result)
		assert.NoError(t, err)
		assert.Equal(t, "golang", result.Steps["build"].Image)
		assert.Equal(t, []string{"go version", "go mod download", "go build"}, result.Steps["build"].Commands)
	})

	t.Run("sequence merge at beginning", func(t *testing.T) {
		data := `
base: &base
  - first
  - second

result:
  - <<: *base
`
		var result map[string][]string
		err := Unmarshal([]byte(data), &result)
		assert.NoError(t, err)
		assert.Equal(t, []string{"first", "second"}, result["result"])
	})
}

func TestMappingMerge(t *testing.T) {
	t.Run("standard mapping merge", func(t *testing.T) {
		data := `
defaults: &defaults
  image: golang
  pull: true

step:
  <<: *defaults
  commands:
    - go build
`
		var result struct {
			Step struct {
				Image    string   `yaml:"image"`
				Pull     bool     `yaml:"pull"`
				Commands []string `yaml:"commands"`
			} `yaml:"step"`
		}
		err := Unmarshal([]byte(data), &result)
		assert.NoError(t, err)
		assert.Equal(t, "golang", result.Step.Image)
		assert.True(t, result.Step.Pull)
		assert.Equal(t, []string{"go build"}, result.Step.Commands)
	})

	t.Run("mapping merge with override", func(t *testing.T) {
		data := `
defaults: &defaults
  image: golang:1.19
  pull: true

step:
  <<: *defaults
  image: golang:latest
`
		var result struct {
			Step struct {
				Image string `yaml:"image"`
				Pull  bool   `yaml:"pull"`
			} `yaml:"step"`
		}
		err := Unmarshal([]byte(data), &result)
		assert.NoError(t, err)
		assert.Equal(t, "golang:latest", result.Step.Image)
		assert.True(t, result.Step.Pull)
	})
}

func TestAnchorAlias(t *testing.T) {
	t.Run("scalar anchor", func(t *testing.T) {
		data := `
image: &img golang:1.21
step:
  image: *img
`
		var result struct {
			Step struct {
				Image string `yaml:"image"`
			} `yaml:"step"`
		}
		err := Unmarshal([]byte(data), &result)
		assert.NoError(t, err)
		assert.Equal(t, "golang:1.21", result.Step.Image)
	})

	t.Run("mapping anchor", func(t *testing.T) {
		data := `
template: &tmpl
  image: golang
  commands:
    - go test

step: *tmpl
`
		var result struct {
			Step struct {
				Image    string   `yaml:"image"`
				Commands []string `yaml:"commands"`
			} `yaml:"step"`
		}
		err := Unmarshal([]byte(data), &result)
		assert.NoError(t, err)
		assert.Equal(t, "golang", result.Step.Image)
		assert.Equal(t, []string{"go test"}, result.Step.Commands)
	})

	// Test for nested anchor bug fix - aliases to anchors defined within parent
	// sequences should be resolved correctly (goccy/go-yaml bug workaround)
	t.Run("nested anchor in sequence", func(t *testing.T) {
		data := `
variables:
  - &when
    - path: &paths
        - 'a.yaml'
        - 'b.yaml'
      event: push
    - path: *paths
      event: pull_request
`
		var result struct {
			Variables [][]struct {
				Path  []string `yaml:"path"`
				Event string   `yaml:"event"`
			} `yaml:"variables"`
		}
		err := Unmarshal([]byte(data), &result)
		assert.NoError(t, err)
		assert.Len(t, result.Variables, 1)
		assert.Len(t, result.Variables[0], 2)
		// Both should have the same paths
		assert.Equal(t, []string{"a.yaml", "b.yaml"}, result.Variables[0][0].Path)
		assert.Equal(t, []string{"a.yaml", "b.yaml"}, result.Variables[0][1].Path)
	})
}

func TestMarshal(t *testing.T) {
	t.Run("simple struct", func(t *testing.T) {
		input := struct {
			Name  string `yaml:"name"`
			Value int    `yaml:"value"`
		}{
			Name:  "test",
			Value: 42,
		}
		data, err := Marshal(input)
		assert.NoError(t, err)
		assert.Contains(t, string(data), "name: test")
		assert.Contains(t, string(data), "value: 42")
	})
}

// Benchmarks to measure the performance of different code paths.

var benchSimpleYAML = []byte(`
steps:
  build:
    image: golang:1.21
    commands:
      - go build
      - go test
  deploy:
    image: alpine
    commands:
      - echo deploying
`)

var benchWithAnchors = []byte(`
defaults: &defaults
  image: golang:1.21
  pull: true

steps:
  build:
    <<: *defaults
    commands:
      - go build
  test:
    <<: *defaults
    commands:
      - go test
`)

var benchWithSequenceMerge = []byte(`
common_commands: &common
  - go mod download
  - go generate

steps:
  build:
    image: golang
    commands:
      - <<: *common
      - go build
  test:
    image: golang
    commands:
      - <<: *common
      - go test
`)

func BenchmarkUnmarshal_FastPath(b *testing.B) {
	// No anchors or aliases - should use fast path
	var result any
	b.ResetTimer()
	for b.Loop() {
		_ = Unmarshal(benchSimpleYAML, &result)
	}
}

func BenchmarkUnmarshal_MediumPath(b *testing.B) {
	// Has anchors/aliases but no sequence merge - medium path
	var result any
	b.ResetTimer()
	for b.Loop() {
		_ = Unmarshal(benchWithAnchors, &result)
	}
}

func BenchmarkUnmarshal_SlowPath(b *testing.B) {
	// Has sequence merge - needs full AST processing
	var result any
	b.ResetTimer()
	for b.Loop() {
		_ = Unmarshal(benchWithSequenceMerge, &result)
	}
}
