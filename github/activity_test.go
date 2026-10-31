package github

import (
	"testing"
	"time"
)

func TestDecodePushes(t *testing.T) {
	// 2 pages, newest first, as gh api --paginate prints them.
	out := []byte(`[{"timestamp":"2030-03-04T09:00:00Z","before":"b","after":"c","activity_type":"force_push"}]
[{"timestamp":"2030-03-04T07:00:00Z","before":"a","after":"b","activity_type":"push"}]
`)

	pushes, err := decodePushes(out)

	if err != nil {
		t.Fatalf("decodePushes(): %v", err)
	}
	want := []Push{
		{Time: time.Date(2030, 3, 4, 7, 0, 0, 0, time.UTC), Before: "a", After: "b", Type: "push"},
		{Time: time.Date(2030, 3, 4, 9, 0, 0, 0, time.UTC), Before: "b", After: "c", Type: "force_push"},
	}
	if len(pushes) != len(want) {
		t.Fatalf("decodePushes() = %+v, want %+v", pushes, want)
	}
	for i := range want {
		if !pushes[i].Time.Equal(want[i].Time) || pushes[i].After != want[i].After || pushes[i].Type != want[i].Type {
			t.Errorf("push %d = %+v, want %+v", i, pushes[i], want[i])
		}
	}
}
