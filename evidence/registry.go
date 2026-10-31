package evidence

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Tag is a tag of a repository of Docker Hub.
type Tag struct {
	Name       string     `json:"name"`
	LastPushed time.Time  `json:"tag_last_pushed"`
	Digest     string     `json:"digest"`
	Images     []TagImage `json:"images"`
}

// TagImage is the image of a platform of a tag.
type TagImage struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Variant      string `json:"variant"`
}

// Platforms returns the platforms of the tag, such as linux/amd64, without the attestations of
// the build, whose platform is unknown/unknown.
func (t Tag) Platforms() []string {
	var platforms []string
	for _, image := range t.Images {
		if image.OS == "" || image.OS == "unknown" {
			continue
		}
		p := image.OS + "/" + image.Architecture
		if image.Variant != "" {
			p += "/" + image.Variant
		}
		platforms = append(platforms, p)
	}
	return platforms
}

// DockerHub reads the tags of the public repositories of Docker Hub.
type DockerHub struct {
	// URL is the address of the API, https://hub.docker.com by default.
	URL    string
	Client *http.Client
}

// Tags returns the tags of the repository, such as "namespace/app", or false if it does not
// exist.
func (h DockerHub) Tags(repository string) ([]Tag, bool, error) {
	base := h.URL
	if base == "" {
		base = "https://hub.docker.com"
	}
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	next := fmt.Sprintf("%s/v2/repositories/%s/tags?page_size=100", strings.TrimRight(base, "/"), url.PathEscape(repository))
	next = strings.Replace(next, "%2F", "/", 1)
	var tags []Tag
	for next != "" {
		page, found, err := h.page(client, next)
		if err != nil || !found {
			return nil, found, err
		}
		tags = append(tags, page.Results...)
		next = page.Next
	}
	return tags, true, nil
}

type tagPage struct {
	Next    string `json:"next"`
	Results []Tag  `json:"results"`
}

func (h DockerHub) page(client *http.Client, address string) (tagPage, bool, error) {
	var page tagPage
	response, err := client.Get(address)
	if err != nil {
		return page, false, err
	}
	defer func() { _ = response.Body.Close() }() // The body is read: an error to close it changes nothing.
	if response.StatusCode == http.StatusNotFound {
		return page, false, nil
	}
	if response.StatusCode != http.StatusOK {
		return page, false, fmt.Errorf("GET %s: %s", address, response.Status)
	}
	if err := json.NewDecoder(response.Body).Decode(&page); err != nil {
		return page, false, fmt.Errorf("GET %s: %w", address, err)
	}
	return page, true, nil
}
