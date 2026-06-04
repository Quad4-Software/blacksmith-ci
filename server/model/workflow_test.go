package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPipelineStatusPrecedence(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   []*Workflow
		want StatusValue
	}{
		{"all success", []*Workflow{{ID: 1, State: StatusSuccess}, {ID: 2, State: StatusSuccess}}, StatusSuccess},
		{"declined beats success", []*Workflow{{ID: 1, State: StatusSuccess}, {ID: 2, State: StatusDeclined}}, StatusDeclined},
		{"declined beats failure", []*Workflow{{ID: 1, State: StatusFailure}, {ID: 2, State: StatusDeclined}}, StatusDeclined},
		{"killed beats declined", []*Workflow{{ID: 1, State: StatusKilled}, {ID: 2, State: StatusDeclined}}, StatusKilled},
		{"failure when no decline or kill", []*Workflow{{ID: 1, State: StatusFailure}, {ID: 2, State: StatusSuccess}}, StatusFailure},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.want, PipelineStatus(c.in))
		})
	}
}
