package api

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("OTEL_TRACES_EXPORTER", "none")
	os.Exit(m.Run())
}
