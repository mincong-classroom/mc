package rules

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/mincong-classroom/mc/common"
)

func TestJavaPodExtras(t *testing.T) {
	team := common.Team{Name: "red"}
	page := `<div class="footer">Team red</div>`
	tests := []struct {
		name        string
		content     string
		labels      map[string]string
		want        float32
		wantMissing []string
	}{
		{
			name:    "everything",
			content: page,
			labels:  map[string]string{"app": "spring-petclinic", "team": "red"},
			want:    0.2,
		},
		{
			name:        "no label",
			content:     page,
			labels:      nil,
			want:        0.1,
			wantMissing: []string{"the label app=spring-petclinic", "the label team=red"},
		},
		{
			name:        "the label of another team",
			content:     page,
			labels:      map[string]string{"app": "spring-petclinic", "team": "gray"},
			want:        0.15,
			wantMissing: []string{"the label team=red"},
		},
		{
			name:        "no team name on the page",
			content:     `<div class="footer">PetClinic</div>`,
			labels:      map[string]string{"app": "spring-petclinic", "team": "red"},
			want:        0.1,
			wantMissing: []string{"the team name on the home page"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, missing := javaPodExtras(team, tt.content, tt.labels)
			if diff := got - tt.want; diff > 1e-6 || diff < -1e-6 {
				t.Errorf("completeness = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(missing, tt.wantMissing) {
				t.Errorf("missing = %q, want %q", missing, tt.wantMissing)
			}
		})
	}
}

func TestPodInfo(t *testing.T) {
	// Trimmed from `kubectl get pod -o json`: an image built for linux/amd64 only, in kind on an
	// arm64 node
	podJSON := `{
  "apiVersion": "v1",
  "kind": "Pod",
  "metadata": {
    "labels": {
      "app": "spring-petclinic",
      "team": "red"
    },
    "name": "spring-petclinic"
  },
  "status": {
    "containerStatuses": [
      {
        "name": "main",
        "ready": false,
        "state": {
          "waiting": {
            "message": "failed to pull and unpack image \"docker.io/mincongclassroom/spring-petclinic-red:1.1.0\": no match for platform in manifest: not found",
            "reason": "ErrImagePull"
          }
        }
      }
    ],
    "phase": "Pending"
  }
}
`
	var pod podInfo
	if err := json.Unmarshal([]byte(podJSON), &pod); err != nil {
		t.Fatal(err)
	}

	wantLabels := map[string]string{"app": "spring-petclinic", "team": "red"}
	if !reflect.DeepEqual(pod.Metadata.Labels, wantLabels) {
		t.Errorf("labels = %v, want %v", pod.Metadata.Labels, wantLabels)
	}
	want := `main: ErrImagePull: failed to pull and unpack image "docker.io/mincongclassroom/spring-petclinic-red:1.1.0": no match for platform in manifest: not found`
	if got := pod.waitingReasons(); got != want {
		t.Errorf("waitingReasons() = %q, want %q", got, want)
	}

	var running podInfo
	if err := json.Unmarshal([]byte(`{"status": {"containerStatuses": [{"name": "main", "state": {"running": {}}}]}}`), &running); err != nil {
		t.Fatal(err)
	}
	if got := running.waitingReasons(); got != "" {
		t.Errorf("waitingReasons() of a running Pod = %q, want empty", got)
	}
}
