package udfs

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

func wtfAbort(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	// Handle different argument counts
	if len(args) == 0 {
		// Default to 500 Internal Server Error if no args provided
		return nil, fmt.Errorf("HTTP_ERROR:500:Internal Server Error")
	} else if len(args) == 1 {
		// Use the first arg as HTTP status code
		code, ok := args[0].(int64)
		if !ok {
			return nil, fmt.Errorf("HTTP_ERROR:400:First argument must be a numeric status code")
		}
		return nil, fmt.Errorf("HTTP_ERROR:%d", code)
	} else if len(args) == 2 {
		// Use first arg as status code and second as message
		code, ok := args[0].(int64)
		if !ok {
			return nil, fmt.Errorf("HTTP_ERROR:400:First argument must be a numeric status code")
		}

		message, ok := args[1].(string)
		if !ok {
			message = fmt.Sprintf("%v", args[1])
		}

		return nil, fmt.Errorf("HTTP_ERROR:%d:%s", code, message)
	}
	return nil, fmt.Errorf("wtf_abort supports up to 2 arguments, got %d", len(args))
}
