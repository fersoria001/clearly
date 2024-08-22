package data_mapper_generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadConfig(t *testing.T) {
	projectDir, err := os.Getwd()
	if err != nil {
		t.Log(err)
	}
	caller := filepath.Dir(projectDir)
	config, err := readConfig(caller)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Objects) < 1 {
		t.Fatalf("expected to have at least one config object %v", *config)
	}
}

func TestDataMapperGenerator(t *testing.T) {
	os.Setenv("ENVIRONMENT", "DEV")
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	err = g.GenerateAll()
	if err != nil {
		t.Fatal(err)
	}
}
