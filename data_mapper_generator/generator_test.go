package data_mapper_generator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDbConfig_valid(t *testing.T) {
	projectDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	caller := filepath.Dir(projectDir)
	config := DbConfig{
		Pkg:     "example",
		Dir:     "/example",
		Builder: "CreatePool",
	}
	err = config.valid(caller)
	if err != nil {
		t.Fatal(err)
	}
}

func TestObjectType_valid(t *testing.T) {
	projectDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	caller := filepath.Dir(projectDir)
	fields := []FieldType{{Name: "id", Column: "id"}, {Name: "name", Column: "name"}}
	object := ObjectType{
		Name:    "DomainAggregate",
		Type:    "aggregate",
		Table:   "aggregate",
		Fields:  fields,
		Pkg:     "example",
		Dir:     "/example",
		Builder: "NewDomainAggregate",
	}
	err = object.valid(caller)
	if err != nil {
		t.Fatal(err)
	}
}

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

func TestDataMapperGenerator_GenerateAll(t *testing.T) {
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

func TestDataMapperGenerator_GenerateAllTests(t *testing.T) {
	os.Setenv("ENVIRONMENT", "DEV")
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	err = g.GenerateAllTests()
	if err != nil {
		t.Fatal(err)
	}
}
