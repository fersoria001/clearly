package main

import (
	"clearly-not-a-secret-project/data_mapper_generator"
	"log"
)

func main() {
	g, err := data_mapper_generator.New()
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	err = g.GenerateAll()
	if err != nil {
		log.Fatalf("error generating data mappers: %v", err)
	}
}
