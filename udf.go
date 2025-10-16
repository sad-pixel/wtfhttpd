package main

import (
	"database/sql/driver"
	"fmt"
	"regexp"
	"strings"

	"modernc.org/sqlite"
)

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
