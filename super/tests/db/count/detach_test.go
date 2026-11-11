package count

import (
	"context"
	"testing"
)

// Tests for BelongsToMany.Detach on a pivot table without an id column.

// pivotTags are the tag ids author id has on author_tag, in order.
func pivotTags(t *testing.T, id int64) []int64 {
	t.Helper()
	a := Author{}.DB(context.Background()).Where("id", id).With("Tags").First()
	var out []int64
	for _, tag := range a.Tags().Items() {
		out = append(out, tag.Id)
	}
	return out
}

func TestDetachSeveralIDs(t *testing.T) {
	setup(t)
	ada := Author{}.DB(context.Background()).Where("id", 1).First()
	if err := ada.Tags().Detach(int64(1), int64(3)); err != nil {
		t.Fatal(err)
	}
	if got := pivotTags(t, 1); len(got) != 1 || got[0] != 2 {
		t.Fatalf("Ada's tags after Detach(1, 3): %v, want [2]", got)
	}
	if got := pivotTags(t, 2); len(got) != 1 || got[0] != 1 {
		t.Fatalf("Bo's tags must be untouched: %v", got)
	}
}

func TestDetachOneAndAll(t *testing.T) {
	setup(t)
	ada := Author{}.DB(context.Background()).Where("id", 1).First()
	if err := ada.Tags().Detach(int64(2)); err != nil {
		t.Fatal(err)
	}
	if got := pivotTags(t, 1); len(got) != 2 {
		t.Fatalf("Ada's tags after Detach(2): %v, want [1 3]", got)
	}
	if err := ada.Tags().Detach(); err != nil {
		t.Fatal(err)
	}
	if got := pivotTags(t, 1); len(got) != 0 {
		t.Fatalf("Ada's tags after Detach(): %v, want none", got)
	}
	if got := pivotTags(t, 2); len(got) != 1 {
		t.Fatalf("Bo's tags must be untouched: %v", got)
	}
}
