package core

import (
	"context"
	"testing"

	"github.com/indes/flowerss-bot/internal/config"
	"github.com/indes/flowerss-bot/internal/model"
)

func TestSourceErrorCountSaturates(t *testing.T) {
	for _, tc := range []struct {
		name string
		before uint
		after uint
	}{
		{"below threshold", config.ErrorThreshold - 1, config.ErrorThreshold},
		{"at threshold", config.ErrorThreshold, config.ErrorThreshold},
		{"explicitly disabled", config.ErrorThreshold + 1, config.ErrorThreshold + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, storage := getTestCore(t)
			defer storage.Ctrl.Finish()
			ctx := context.Background()
			source := &model.Source{ID: 8, ErrorCount: tc.before}
			storage.Source.EXPECT().GetSource(ctx, uint(8)).Return(source, nil)
			if tc.before < config.ErrorThreshold {
				storage.Source.EXPECT().UpsertSource(ctx, uint(8), source).Return(nil)
			}
			if err := c.SourceErrorCountIncr(ctx, 8); err != nil {
				t.Fatal(err)
			}
			if source.ErrorCount != tc.after {
				t.Fatalf("error count = %d, want %d", source.ErrorCount, tc.after)
			}
		})
	}
}
