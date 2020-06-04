package memlog

import (
	"testing"

	"github.com/elastic/beats/v7/libbeat/logp"
	"github.com/elastic/beats/v7/libbeat/statestore/backend"
	"github.com/elastic/beats/v7/libbeat/statestore/backend/storetests"
)

func init() {
	logp.DevelopmentSetup()
}

func TestCompliance(t *testing.T) {
	storetests.TestBackendCompliance(t, func(testPath string) (backend.Registry, error) {
		return New(logp.NewLogger("test"), Settings{Root: testPath})
	})
}
