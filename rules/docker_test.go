package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mincong-classroom/mc/common"
)

func TestUsesTemurinLTS(t *testing.T) {
	tests := []struct {
		name       string
		dockerfile string
		want       bool
	}{
		{name: "LTS 25, JDK on Alpine", want: true, dockerfile: `
FROM eclipse-temurin:25-alpine

WORKDIR /app

COPY target/spring-petclinic-4.0.0-SNAPSHOT.jar app.jar

EXPOSE 8080

ENTRYPOINT ["java", "-jar", "app.jar"]
`},
		{name: "LTS 21, JRE on Alpine, full version", want: true, dockerfile: `
FROM eclipse-temurin:21.0.8_9-jre-alpine
`},
		{name: "LTS 17, the minimum", want: true, dockerfile: `
FROM eclipse-temurin:17-jre
`},
		{name: "a future LTS", want: true, dockerfile: `
FROM eclipse-temurin:29
`},
		{name: "Docker Hub, full name", want: true, dockerfile: `
FROM docker.io/library/eclipse-temurin:21
`},
		{name: "mirror of Google", want: true, dockerfile: `
FROM mirror.gcr.io/library/eclipse-temurin:25-jre
`},
		{name: "AWS Public ECR", want: true, dockerfile: `
FROM public.ecr.aws/docker/library/eclipse-temurin:21
`},
		{name: "with a platform", want: true, dockerfile: `
FROM --platform=linux/amd64 eclipse-temurin:25
`},
		{name: "instruction in lower case", want: true, dockerfile: `
from eclipse-temurin:21
`},
		{name: "multi-stage build", want: true, dockerfile: `
FROM maven:3.9-eclipse-temurin-21 AS builder
WORKDIR /src
COPY . .
RUN mvn package -DskipTests

FROM eclipse-temurin:21-jre
COPY --from=builder /src/target/*.jar app.jar
`},
		{name: "not an LTS", want: false, dockerfile: `
FROM eclipse-temurin:23
`},
		{name: "an LTS older than 17", want: false, dockerfile: `
FROM eclipse-temurin:11
`},
		{name: "no version", want: false, dockerfile: `
FROM eclipse-temurin:latest
`},
		{name: "no tag", want: false, dockerfile: `
FROM eclipse-temurin
`},
		{name: "another distribution", want: false, dockerfile: `
FROM amazoncorretto:21
`},
		{name: "Temurin only in the tag of another image", want: false, dockerfile: `
FROM maven:3.9-eclipse-temurin-21
`},
		{name: "commented out", want: false, dockerfile: `
# FROM eclipse-temurin:21
FROM openjdk:21
`},
	}
	for _, tt := range tests {
		if got := usesTemurinLTS(tt.dockerfile); got != tt.want {
			t.Errorf("usesTemurinLTS() for %q = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestDockerfileRuleRun(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	team := common.Team{Name: "red"}
	path := filepath.Join(team.GetRepoPath(), "apps/spring-petclinic/Dockerfile")

	result := DockerfileRule{}.Run(team, "")
	if result.Completeness != 0 || result.Reason != "The Dockerfile is missing" {
		t.Errorf("Run() without a Dockerfile = %v, %q", result.Completeness, result.Reason)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		dockerfile       string
		wantCompleteness float32
		wantReason       string
	}{
		{dockerfile: `
FROM eclipse-temurin:25-alpine
EXPOSE 8080
`, wantCompleteness: 1, wantReason: "The Dockerfile is correct"},
		{dockerfile: `
FROM openjdk:21
EXPOSE 8080
`, wantCompleteness: 0.2, wantReason: "does not use the correct Java version or distribution"},
		{dockerfile: `
FROM eclipse-temurin:17
`, wantCompleteness: 0.8, wantReason: "does not expose the port 8080"},
	}
	for _, tt := range tests {
		if err := os.WriteFile(path, []byte(tt.dockerfile), 0o644); err != nil {
			t.Fatal(err)
		}
		result := DockerfileRule{}.Run(team, "")
		if result.Completeness != tt.wantCompleteness || !strings.Contains(result.Reason, tt.wantReason) {
			t.Errorf("Run() for %q = %v, %q; want %v, %q",
				tt.dockerfile, result.Completeness, result.Reason, tt.wantCompleteness, tt.wantReason)
		}
	}
}
