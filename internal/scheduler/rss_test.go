package scheduler

import (
	"testing"

	"github.com/indes/flowerss-bot/internal/config"
	"github.com/indes/flowerss-bot/internal/model"
)

func TestSourcePollingDisabled(t *testing.T) {
	for _, tc := range []struct {
		name string
		errors uint
		disabled bool
	}{
		{"below threshold", config.ErrorThreshold-1, false},
		{"at threshold must retry", config.ErrorThreshold, false},
		{"explicitly disabled", config.ErrorThreshold+1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := sourcePollingDisabled(&model.Source{ErrorCount: tc.errors})
			if got != tc.disabled {
				t.Fatalf("errors=%d: disabled=%v, want %v", tc.errors, got, tc.disabled)
			}
		})
	}
}
