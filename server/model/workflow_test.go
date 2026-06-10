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
		// Existing precedence cases.
		{"all success", []*Workflow{{ID: 1, State: StatusSuccess}, {ID: 2, State: StatusSuccess}}, StatusSuccess},
		{"declined beats success", []*Workflow{{ID: 1, State: StatusSuccess}, {ID: 2, State: StatusDeclined}}, StatusDeclined},
		{"declined beats failure", []*Workflow{{ID: 1, State: StatusFailure}, {ID: 2, State: StatusDeclined}}, StatusDeclined},
		{"killed beats declined", []*Workflow{{ID: 1, State: StatusKilled}, {ID: 2, State: StatusDeclined}}, StatusKilled},
		{"failure when no decline or kill", []*Workflow{{ID: 1, State: StatusFailure}, {ID: 2, State: StatusSuccess}}, StatusFailure},

		// New precedence cases for blocked/running rollup. The bug being
		// fixed: a pipeline with one successful workflow and one workflow
		// still gated on approval (Blocked) was rolling up to Success,
		// hiding the fact that human action was still required.
		{"blocked beats success", []*Workflow{{ID: 1, State: StatusSuccess}, {ID: 2, State: StatusBlocked}}, StatusBlocked},
		{"running beats success", []*Workflow{{ID: 1, State: StatusSuccess}, {ID: 2, State: StatusRunning}}, StatusRunning},
		{"pending beats success", []*Workflow{{ID: 1, State: StatusSuccess}, {ID: 2, State: StatusPending}}, StatusRunning},
		// Blocked wins over running because it is more actionable — it
		// tells the user "approve something", whereas running just says
		// "wait". Both situations are simultaneously true when at least
		// one workflow is blocked.
		{"blocked beats running", []*Workflow{{ID: 1, State: StatusRunning}, {ID: 2, State: StatusBlocked}}, StatusBlocked},
		// Higher-precedence outcomes still override blocked.
		{"declined beats blocked", []*Workflow{{ID: 1, State: StatusBlocked}, {ID: 2, State: StatusDeclined}}, StatusDeclined},
		{"failure beats blocked", []*Workflow{{ID: 1, State: StatusBlocked}, {ID: 2, State: StatusFailure}}, StatusFailure},
		{"killed beats blocked", []*Workflow{{ID: 1, State: StatusBlocked}, {ID: 2, State: StatusKilled}}, StatusKilled},
		// Single-workflow sanity checks.
		{"single blocked", []*Workflow{{ID: 1, State: StatusBlocked}}, StatusBlocked},
		{"single running", []*Workflow{{ID: 1, State: StatusRunning}}, StatusRunning},
		{"single pending", []*Workflow{{ID: 1, State: StatusPending}}, StatusRunning},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.want, PipelineStatus(c.in))
		})
	}
}
