package common

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Settings are the settings of mc on this machine, in ~/.mc/settings.yaml. The file and each of
// its keys are optional.
type Settings struct {
	// Year is the year of the cohort, such as 2026, which selects the files of the year, e.g.
	// ~/.mc/teams-{year}.yaml. The current year by default.
	Year int `yaml:"year"`
	// Rules is the YAML of the evaluation rules of a year, where {year} is the year, such as
	// ~/github/my-org/course/{year}/evaluation-rules.yaml. Without it, the commands that read the
	// rules need the flag --rules-file.
	Rules string `yaml:"rules"`
}

// SettingsPath returns the file of the settings, ~/.mc/settings.yaml.
func SettingsPath() string {
	return filepath.Join(ConfigDir(), "settings.yaml")
}

// ReadSettings reads the settings, which are empty if the file doesn't exist. An unknown key is an
// error, so that a typo doesn't go unnoticed.
func ReadSettings() (Settings, error) {
	var s Settings
	data, err := os.ReadFile(SettingsPath())
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&s); err != nil && !errors.Is(err, io.EOF) {
		return s, fmt.Errorf("%s: %w", SettingsPath(), err)
	}
	if s.Year < 0 {
		return s, fmt.Errorf("%s: the year %d is not a year", SettingsPath(), s.Year)
	}
	return s, nil
}

// settings reads the settings once per run of mc. CheckSettings reports its error at the start.
var settings = sync.OnceValues(ReadSettings)

// CheckSettings returns the error of the settings, if any: mc checks them before any command, so
// that a command never runs on a year that a broken file failed to give.
func CheckSettings() error {
	_, err := settings()
	return err
}

// Year returns the year of the cohort, which selects the files of the year: the environment
// variable MC_YEAR, e.g. MC_YEAR=2025, then the key year of the settings, then the current year.
func Year() int {
	if year, err := strconv.Atoi(os.Getenv("MC_YEAR")); err == nil {
		return year
	}
	if s, err := settings(); err == nil && s.Year != 0 {
		return s.Year
	}
	return time.Now().Year()
}

// RulesPath returns the YAML of the evaluation rules: file when it is not empty, such as the
// value of the flag --rules-file, otherwise the key rules of the settings, for the year of Year.
func RulesPath(file string) (string, error) {
	if file != "" {
		return ExpandHome(file), nil
	}
	s, err := settings()
	if err != nil {
		return "", err
	}
	return s.RulesOf(Year())
}

// RulesOf returns the YAML of the evaluation rules of the year, from the key rules.
func (s Settings) RulesOf(year int) (string, error) {
	if s.Rules == "" {
		return "", fmt.Errorf("no evaluation rules: set the key rules in %s, such as rules: ~/course/{year}/evaluation-rules.yaml, or pass --rules-file", SettingsPath())
	}
	return ExpandHome(strings.ReplaceAll(s.Rules, "{year}", strconv.Itoa(year))), nil
}

// ExpandHome replaces the leading "~/" of a path by the home directory, which the shell doesn't
// do for a flag such as --team-file=~/... nor for a value of the settings.
func ExpandHome(path string) string {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		return filepath.Join(os.Getenv("HOME"), rest)
	}
	return path
}
