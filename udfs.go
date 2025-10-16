package main

import (
	"database/sql/driver"
	"fmt"
	"log"
	"regexp"
	"strings"

	"modernc.org/sqlite"
)

// extractPathParams extracts path parameters from a URL pattern
// For example, if the pattern is "/users/{id}/profile",
// it will return ["id"]
func extractPathParams(pattern string) []string {
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")

	var params []string

	for _, part := range patternParts {
		// Check if this part is a parameter (enclosed in {})
		if len(part) > 2 && part[0] == '{' && part[len(part)-1] == '}' {
			// Extract the parameter name without the braces
			paramName := part[1 : len(part)-1]
			params = append(params, paramName)
		}
	}

	return params
}

var nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("slugify supports 1 argument, got %d", len(args))
	}
	s, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("slugify supports arguments of string type, got %T", args[0])
	}

	// 1. Convert to lowercase
	result := strings.ToLower(s)
	// 2. Replace non-alphanumeric characters with a hyphen
	result = nonAlphanumericRegex.ReplaceAllString(result, "-")
	// 3. Trim leading/trailing hyphens
	result = strings.Trim(result, "-")
	return result, nil
}

func registerUdfs() {
	// Register the slugify function with SQLite
	err := sqlite.RegisterFunction(
		"slugify",
		&sqlite.FunctionImpl{
			NArgs:         1,
			Deterministic: true,
			Scalar:        slugify,
		},
	)
	if err != nil {
		log.Fatalf("Error registering slugify function: %v", err)
	}
}
