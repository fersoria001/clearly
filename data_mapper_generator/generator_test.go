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
	g := new(DataMapperGenerator)
	err = g.readConfig(caller)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.config.Objects) < 1 {
		t.Fatalf("expected to have at least one config object %v", *g.config)
	}
}

func TestDataMapperGenerator_generateRegistry(t *testing.T) {
	os.Setenv("ENVIRONMENT", "DEV")
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	err = g.generateDataMapperRegistry()
	if err != nil {
		t.Fatal(err)
	}
}

func TestDataMapperGenerator_generateDatasource(t *testing.T) {
	os.Setenv("ENVIRONMENT", "DEV")
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	err = g.generateDataMapperRegistry()
	if err != nil {
		t.Fatal(err)
	}
	err = g.generateDataSource()
	if err != nil {
		t.Fatal(err)
	}
}

func TestDataMapperGenerator_generateObjectMethods(t *testing.T) {
	os.Setenv("ENVIRONMENT", "DEV")
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	err = g.generateDataMapperRegistry()
	if err != nil {
		t.Fatal(err)
	}
	err = g.generateDataSource()
	if err != nil {
		t.Fatal(err)
	}
	err = g.generateObjectMethods(g.config.Objects[0])
	if err != nil {
		t.Fatal(err)
	}
}

func TestDataMapperGenerator_generateDataMapper(t *testing.T) {
	os.Setenv("ENVIRONMENT", "DEV")
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	err = g.generateDataMapperRegistry()
	if err != nil {
		t.Fatal(err)
	}
	err = g.generateDataSource()
	if err != nil {
		t.Fatal(err)
	}
	err = g.generateObjectMethods(g.config.Objects[0])
	if err != nil {
		t.Fatal(err)
	}
	err = g.generateDataMapper(g.config.Objects[0])
	if err != nil {
		t.Fatal(err)
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
	err = g.generateDataMapperRegistry()
	if err != nil {
		t.Fatal(err)
	}
	err = g.generateDataSource()
	if err != nil {
		t.Fatal(err)
	}
	err = g.generateObjectMethods(g.config.Objects[0])
	if err != nil {
		t.Fatal(err)
	}
	err = g.generateTest(g.config.Objects[0])
	if err != nil {
		t.Fatal(err)
	}
}
