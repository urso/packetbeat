//+build linux,cgo

package journalfield

import (
	"testing"

	"github.com/coreos/go-systemd/v22/sdjournal"
)

func TestApplyMatchersOr(t *testing.T) {
	cases := map[string]struct {
		filters []string
		wantErr bool
	}{
		"correct filter expression": {
			filters: []string{"systemd.unit=nginx"},
			wantErr: false,
		},
		"custom field": {
			filters: []string{"_MY_CUSTOM_FIELD=value"},
			wantErr: false,
		},
		"mixed filters": {
			filters: []string{"systemd.unit=nginx", "_MY_CUSTOM_FIELD=value"},
			wantErr: false,
		},
		"same field filters": {
			filters: []string{"systemd.unit=nginx", "systemd.unit=mysql"},
			wantErr: false,
		},
		"incorrect separator": {
			filters: []string{"systemd.unit~nginx"},
			wantErr: true,
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			journal, err := sdjournal.NewJournal()
			if err != nil {
				t.Fatalf("error while creating test journal: %v", err)
			}
			defer journal.Close()

			matchers := make([]Matcher, len(test.filters))
			for i, str := range test.filters {
				m, err := BuildMatcher(str)
				if err != nil && !test.wantErr {
					t.Fatalf("unexpected error compiling the filters: %v", err)
				}
				matchers[i] = m
			}

			// double check if journald likes our filters
			err = ApplyMatchersOr(journal, matchers)
			fail := (test.wantErr && err == nil) || (!test.wantErr && err != nil)
			if fail {
				t.Errorf("unexpected outcome: error: '%v', expected error: %v", err, test.wantErr)
			}
		})
	}
}
