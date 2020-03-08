package main

import (
	"testing"
)

func TestConfigParsingYaml(t *testing.T) {
	fp := "../../testdata/linux.yaml"
	projects, err := projectsList(fp)
	if err != nil {
		t.Errorf("Unexpected error reading %s: %v", fp, err)
	}
	if len(projects) != 3 {
		t.Errorf("In %s expected 3 projects, got %d", fp, len(projects))
	}
}

func TestConfigParsingJson(t *testing.T) {
	fp := "../../testdata/linux.json"
	projects, err := projectsList(fp)
	if err != nil {
		t.Errorf("Unexpected error reading %s: %v", fp, err)
	}
	if len(projects) != 3 {
		t.Errorf("In %s expected 3 projects, got %d", fp, len(projects))
	}
}
