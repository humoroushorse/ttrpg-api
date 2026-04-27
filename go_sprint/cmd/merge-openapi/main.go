package main

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

func main() {
	// Read workitems spec
	workitemsData, err := os.ReadFile("../../api/openapi/workitems.yaml")
	if err != nil {
		log.Fatal("Failed to read workitems.yaml:", err)
	}
	var workitems map[string]interface{}
	if err := yaml.Unmarshal(workitemsData, &workitems); err != nil {
		log.Fatal("Failed to parse workitems.yaml:", err)
	}

	// Read sprints spec
	sprintsData, err := os.ReadFile("../../api/openapi/sprints.yaml")
	if err != nil {
		log.Fatal("Failed to read sprints.yaml:", err)
	}
	var sprints map[string]interface{}
	if err := yaml.Unmarshal(sprintsData, &sprints); err != nil {
		log.Fatal("Failed to parse sprints.yaml:", err)
	}

	// Read auth spec
	authData, err := os.ReadFile("../../../go_auth/api/openapi/auth.yaml")
	if err != nil {
		log.Fatal("Failed to read auth.yaml:", err)
	}
	var auth map[string]interface{}
	if err := yaml.Unmarshal(authData, &auth); err != nil {
		log.Fatal("Failed to parse auth.yaml:", err)
	}

	// Create combined spec
	combined := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "Sprint Management API with Authentication",
			"version":     "1.0.0",
			"description": "Complete API for sprint management including authentication",
		},
		"servers": []interface{}{
			map[string]interface{}{
				"url":         "http://localhost:8003",
				"description": "Local development server",
			},
		},
		"paths": make(map[string]interface{}),
		"components": map[string]interface{}{
			"schemas":         make(map[string]interface{}),
			"securitySchemes": make(map[string]interface{}),
		},
	}

	// Merge paths
	paths := combined["paths"].(map[string]interface{})

	if workitemsPaths, ok := workitems["paths"].(map[string]interface{}); ok {
		for k, v := range workitemsPaths {
			paths[k] = v
		}
	}

	if sprintsPaths, ok := sprints["paths"].(map[string]interface{}); ok {
		for k, v := range sprintsPaths {
			paths[k] = v
		}
	}

	// Add auth paths with /api/v1 prefix
	if authPaths, ok := auth["paths"].(map[string]interface{}); ok {
		for k, v := range authPaths {
			paths["/api/v1"+k] = v
		}
	}

	// Merge components
	components := combined["components"].(map[string]interface{})
	schemas := components["schemas"].(map[string]interface{})
	securitySchemes := components["securitySchemes"].(map[string]interface{})

	for _, spec := range []map[string]interface{}{workitems, sprints, auth} {
		if comp, ok := spec["components"].(map[string]interface{}); ok {
			if s, ok := comp["schemas"].(map[string]interface{}); ok {
				for k, v := range s {
					schemas[k] = v
				}
			}
			if ss, ok := comp["securitySchemes"].(map[string]interface{}); ok {
				for k, v := range ss {
					securitySchemes[k] = v
				}
			}
		}
	}

	// Write combined spec
	output, err := yaml.Marshal(combined)
	if err != nil {
		log.Fatal("Failed to marshal combined spec:", err)
	}

	if err := os.WriteFile("../../api/openapi/combined-with-auth.yaml", output, 0644); err != nil {
		log.Fatal("Failed to write combined-with-auth.yaml:", err)
	}

	fmt.Println("✅ Combined spec generated successfully!")
	fmt.Printf("📊 Total paths: %d\n", len(paths))
	fmt.Println("\nEndpoints included:")
	for path := range paths {
		fmt.Printf("  - %s\n", path)
	}
}
