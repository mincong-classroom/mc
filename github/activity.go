package github

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"time"
)

// Push is a push to a branch of a repository, from its activity on GitHub.
type Push struct {
	Time   time.Time `json:"timestamp"`
	Before string    `json:"before"` // The commit of the branch before the push.
	After  string    `json:"after"`  // The commit of the branch after the push.
	// Type is "push", or "force_push" when the push rewrote the history, among others.
	Type string `json:"activity_type"`
}

// MainPushes returns the pushes to the branch main of the repository of the organization, oldest
// first. The activity of the repository has every push, unlike its events, which miss some.
func (CLI) MainPushes(repo string) ([]Push, error) {
	out, err := gh("api", "--paginate", fmt.Sprintf("repos/%s/%s/activity?ref=refs/heads/main&per_page=100", Org, repo))
	if err != nil {
		return nil, err
	}
	return decodePushes(out)
}

// decodePushes reads the pages of the activity: gh prints one JSON array per page.
func decodePushes(out []byte) ([]Push, error) {
	var pushes []Push
	decoder := json.NewDecoder(bytes.NewReader(out))
	for {
		var page []Push
		err := decoder.Decode(&page)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read the activity: %w", err)
		}
		pushes = append(pushes, page...)
	}
	sort.SliceStable(pushes, func(i, j int) bool { return pushes[i].Time.Before(pushes[j].Time) })
	return pushes, nil
}
