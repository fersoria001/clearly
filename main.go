package main

import (
	"clearly-not-a-secret-project/data_mapper_generator"
	"log"
)

func main() {
	log.Println("validating the configuration, it can take a while...")
	g, err := data_mapper_generator.New()
	if err != nil {
		log.Printf("error: %v", err)
	}
	log.Println("generating data mappers...")
	err = g.GenerateAll()
	if err != nil {
		log.Printf("error generating data mappers: %v", err)
	}
	log.Println("data mappers generated")
	if g.WithTests() {
		log.Println("found test config, generating tests...")
		err = g.GenerateAllTests()
		if err != nil {
			log.Printf("error generating tests: %v", err)
		}
		log.Println("all test generated")
	}
	log.Println("code generated")
}
