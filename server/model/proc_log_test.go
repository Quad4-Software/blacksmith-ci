package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcLogEntry_TableName(t *testing.T) {
	entry := ProcLogEntry{}
	assert.Equal(t, "proc_log_entries", entry.TableName())
}
