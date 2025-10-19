package main

import (
	"encoding/json"
	"strconv"
)

// JsonRow represents a single flattened row to be inserted into the request_json table.
type JsonRow struct {
	Path  string      // The JSONPath-like key, e.g., "$.user.name"
	Value interface{} // The primitive value
	Type  string      // The JSON data type as a string
	Json  string      // The raw JSON of the value if it's an object or array
}

func flattenJson(prefix string, data interface{}, rows *[]JsonRow) {
	switch v := data.(type) {
	case map[string]interface{}: // It's an object
		// Marshal this sub-tree back to JSON for the 'json' column.
		jsonBytes, _ := json.Marshal(v)
		*rows = append(*rows, JsonRow{Path: prefix, Value: "[object]", Type: "object", Json: string(jsonBytes)})

		// Recurse into its children.
		for key, val := range v {
			newPrefix := prefix + "." + key
			flattenJson(newPrefix, val, rows)
		}

	case []interface{}: // It's an array
		jsonBytes, _ := json.Marshal(v)
		*rows = append(*rows, JsonRow{Path: prefix, Value: "[array]", Type: "array", Json: string(jsonBytes)})

		// Recurse into its elements.
		for i, val := range v {
			newPrefix := prefix + "[" + strconv.Itoa(i) + "]"
			flattenJson(newPrefix, val, rows)
		}

	case string:
		*rows = append(*rows, JsonRow{Path: prefix, Value: v, Type: "string"})
	case float64:
		// JSON numbers are all float64 when unmarshaled into interface{}.
		*rows = append(*rows, JsonRow{Path: prefix, Value: v, Type: "number"})
	case bool:
		*rows = append(*rows, JsonRow{Path: prefix, Value: v, Type: "boolean"})
	case nil:
		*rows = append(*rows, JsonRow{Path: prefix, Value: nil, Type: "null"})
	}
}
