package rules

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/mincong-classroom/mc/common"
)

type TagResponse struct {
	Count   int `json:"count"`
	Results []struct {
		Name string `json:"name"`
	} `json:"results"`
}

type RegistryRule struct{}

func (r RegistryRule) Spec() common.RuleSpec {
	return common.RuleSpec{
		LabId:    "L2",
		Symbol:   "RGT",
		Name:     "Registry Test",
		Exercice: "1",
		Description: `
The team is expected to upload the Docker image to the registry (Dockerhub).
This is the key of the whole lab session. By completing this exercise, it means
that the students were able to define the Dockerfile correctly, build the
Docker image, connect to the Dockerhub, and push the image with the right tag.
Else, teacher (Mincong) should check the steps by breaking it down into
multiple steps. Two kinds of tags are published to the registry, the "latest"
kind and the "commit" kind.`,
	}
}

func (r RegistryRule) Run(team common.Team, _ string) common.RuleEvaluationResult {
	result := common.RuleEvaluationResult{
		Team:         team,
		RuleId:       r.Spec().Id(),
		Completeness: 0,
		Reason:       "",
		ExecError:    nil,
	}

	// Docker Hub API URL for tags
	var (
		repo          = team.GetContainerRepoForWeekendServer()
		url           = fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/tags/", repo)
		tagResponse   TagResponse
		hasLatestTag  bool
		hasCommitTags bool
	)

	// Make HTTP GET request
	fmt.Printf("Fetching tags from %s\n", url)
	resp, err := http.Get(url)
	if err != nil {
		result.Reason = fmt.Sprintf("Error fetching tags: %v\n", err)
		result.ExecError = err
		return result
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Reason = fmt.Sprintf("Error reading response: %v\n", err)
		result.ExecError = err
		return result
	}

	// Parse JSON response
	err = json.Unmarshal(body, &tagResponse)
	if err != nil {
		result.Reason = fmt.Sprintf("Error parsing JSON: %v\n", err)
		result.ExecError = err
		return result
	}

	if tagResponse.Count < 10 {
		result.Reason = fmt.Sprintf("The image has less than 10 tags (%d), manual check required", tagResponse.Count)
		return result
	}
	result.Completeness += 0.5

	commitTagRegex, _ := regexp.Compile(`^[a-f0-9]{40}$`)
	for _, tag := range tagResponse.Results {
		if !hasLatestTag && tag.Name == "latest" {
			hasLatestTag = true
			result.Completeness += 0.2
		}
		if !hasCommitTags {
			if commitTagRegex.MatchString(tag.Name) {
				hasCommitTags = true
				result.Completeness += 0.3
			}
		}
	}

	if result.Completeness == 1 {
		result.Reason = fmt.Sprintf("Found %d tags including latest tag and commit tags", tagResponse.Count)
	} else {
		if !hasLatestTag {
			result.Reason += "Missing latest tag. "
		}
		if !hasCommitTags {
			result.Reason += "Missing commit tags. "
		}
		result.Reason = strings.TrimSpace(result.Reason)
	}

	return result
}
