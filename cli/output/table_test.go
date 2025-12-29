package output

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

type writerFlusherStub struct {
	bytes.Buffer
}

func (s writerFlusherStub) Flush() error {
	return nil
}

type testFieldsStruct struct {
	Name   string
	Number int
}

func TestTableOutput(t *testing.T) {
	var wfs writerFlusherStub
	to := NewTable(os.Stdout)
	to.w = &wfs

	t.Run("AddAllowedFields", func(t *testing.T) {
		_, _ = to.AddAllowedFields(testFieldsStruct{})
		if _, ok := to.allowedFields["name"]; !ok {
			t.Error("name should be a allowed field")
		}
	})
	t.Run("AddFieldAlias", func(t *testing.T) {
		to.AddFieldAlias("CrowCI", "crow")
		if alias, ok := to.fieldAlias["crow"]; !ok || alias != "CrowCI" {
			t.Errorf("'crow' alias should resolve to 'CrowCI', is: %v", alias)
		}
	})
	t.Run("AddFieldOutputFn", func(t *testing.T) {
		to.AddFieldFn("CrowCI", FieldFn(func(_ any) string {
			return "CROW CI!!!"
		}))
		if _, ok := to.fieldMapping["crowci"]; !ok {
			t.Errorf("'CrowCI' field output fn should be set")
		}
	})
	t.Run("ValidateColumns", func(t *testing.T) {
		err := to.ValidateColumns([]string{"non-existent", "NAME"})
		if err == nil ||
			strings.Contains(err.Error(), "name") ||
			!strings.Contains(err.Error(), "non-existent") {
			t.Errorf("error should contain 'non-existent' but not 'name': %v", err)
		}
	})
	t.Run("WriteHeader", func(t *testing.T) {
		to.WriteHeader([]string{"crow", "name"})
		if wfs.String() != "CROW\tNAME\n" {
			t.Errorf("written header should be 'CROW\\tNAME\\n', is: %q", wfs.String())
		}
		wfs.Reset()
	})
	t.Run("WriteLine", func(t *testing.T) {
		_ = to.Write([]string{"crow", "name", "number"}, &testFieldsStruct{"test123", 1000000000})
		if wfs.String() != "CROW CI!!!\ttest123\t1000000000\n" {
			t.Errorf("written line should be 'CROW CI!!!\\ttest123\\t1000000000\\n', is: %q", wfs.String())
		}
		wfs.Reset()
	})
	t.Run("Columns", func(t *testing.T) {
		if len(to.Columns()) != 3 {
			t.Errorf("unexpected number of columns: %v", to.Columns())
		}
	})
}
