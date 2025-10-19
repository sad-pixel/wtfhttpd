package udfs

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"modernc.org/sqlite"
)

func buildQuery(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("build_query supports 1 argument, got %d", len(args))
	}

	jsonStr, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("build_query argument must be a string, got %T", args[0])
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil, fmt.Errorf("invalid JSON: %v", err)
	}

	values := url.Values{}
	for key, val := range data {
		switch v := val.(type) {
		case string:
			values.Add(key, v)
		case float64:
			values.Add(key, fmt.Sprintf("%v", v))
		case bool:
			values.Add(key, fmt.Sprintf("%v", v))
		case []interface{}:
			for _, item := range v {
				values.Add(key+"[]", fmt.Sprintf("%v", item))
			}
		case nil:
			values.Add(key, "")
		case map[string]interface{}:
			// Skip nested objects or serialize them if needed
			jsonBytes, err := json.Marshal(v)
			if err == nil {
				values.Add(key, string(jsonBytes))
			}
		default:
			values.Add(key, fmt.Sprintf("%v", v))
		}
	}

	return values.Encode(), nil
}

func parseQuery(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("parse_query supports 1 argument, got %d", len(args))
	}

	queryString, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("parse_query argument must be a string, got %T", args[0])
	}

	// Remove leading ? if present
	queryString = strings.TrimPrefix(queryString, "?")

	values, err := url.ParseQuery(queryString)
	if err != nil {
		return nil, fmt.Errorf("invalid query string: %v", err)
	}

	result := make(map[string]interface{})

	for key, vals := range values {
		// Check if this is an array parameter (ends with [])
		if strings.HasSuffix(key, "[]") {
			baseKey := strings.TrimSuffix(key, "[]")
			result[baseKey] = vals
		} else if len(vals) == 1 {
			// Single value
			result[key] = vals[0]
		} else if len(vals) > 1 {
			// Multiple values for the same key
			result[key] = vals
		}
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("error converting to JSON: %v", err)
	}

	return string(jsonBytes), nil
}
