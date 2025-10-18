package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
)

func createHandler(app *App, path string, pathParams []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		app.hitsProcessed.Add(1)
		effectivePath := filepath.Join(app.Config.WebRoot, path)

		content, err := os.ReadFile(effectivePath)
		if err != nil {
			http.Error(w, "Error reading file: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Create a transaction to work with temporary tables
		tx, err := app.DB.Begin()
		if err != nil {
			http.Error(w, "Error starting transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer tx.Rollback() // Will be ignored if transaction is committed

		if err := setupTemporaryTables(tx); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := populateTemporaryTables(tx, r, pathParams, app.Config); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		varsMap := make(map[string]interface{})

		// First add path parameters (highest precedence)
		for _, param := range pathParams {
			varsMap[param] = r.PathValue(param)
		}

		// Parse form data (including multipart forms)
		if err := r.ParseForm(); err == nil {
			// Add form parameters (second precedence)
			for key, values := range r.Form {
				// Check if this is an array parameter
				if strings.HasSuffix(key, "[]") {
					// Remove the [] suffix and store as JSON string
					arrayKey := key[:len(key)-2]
					if _, exists := varsMap[arrayKey]; !exists {
						jsonData, err := json.Marshal(values)
						if err == nil {
							varsMap[arrayKey] = string(jsonData)
						}
					}
				} else if _, exists := varsMap[key]; !exists {
					// For non-array parameters, use the first value if not already set by path params
					if len(values) > 0 {
						varsMap[key] = values[0]
					}
				}
			}
		}

		// Add query parameters (lowest precedence)
		for key, values := range r.URL.Query() {
			// Check if this is an array parameter
			if strings.HasSuffix(key, "[]") {
				// Remove the [] suffix and store as JSON string
				arrayKey := key[:len(key)-2]
				if _, exists := varsMap[arrayKey]; !exists {
					jsonData, err := json.Marshal(values)
					if err == nil {
						varsMap[arrayKey] = string(jsonData)
					}
				}
			} else if _, exists := varsMap[key]; !exists {
				// For non-array parameters, use the first value if not already set by path or form params
				if len(values) > 0 {
					varsMap[key] = values[0]
				}
			}
		}

		parsedQueries := ParseQueries(string(content))
		results := make(map[string][]map[string]any)
		for _, query := range parsedQueries {
			// Log all directives and the query
			log.Printf("Executing query: %s\n", query.Query)
			if len(query.Directives) > 0 {
				log.Printf("Query directives: %+v\n", query.Directives)
			}

			result, err := executeQuery(tx, query.Query, varsMap)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Check for store directive
			storeDirectiveFound := false
			for _, directive := range query.Directives {
				if directive.name == "store" && len(directive.params) > 0 {
					// Store the result under the specified key
					storeKey := directive.params[0]
					results[storeKey] = result
					log.Printf("Stored query result in '%s'", storeKey)
					storeDirectiveFound = true
					break
				}
			}

			// If no store directive was found, store the result in the "ctx" key
			if !storeDirectiveFound {
				results["ctx"] = result
				log.Printf("No store directive found, stored query result in 'ctx'")
			}
		}

		statusCode := http.StatusOK
		responseHeaders := make(map[string]string)

		rows, err := tx.Query("SELECT name, value FROM response_meta")
		tplName := ""
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var name, value string
				if err := rows.Scan(&name, &value); err != nil {
					continue
				}

				if strings.ToLower(name) == "status" {
					if code, err := strconv.Atoi(value); err == nil && code >= 100 && code < 600 {
						statusCode = code
					}
				} else if strings.ToLower(name) == "wtf-tpl" {
					tplName = value
				} else {
					responseHeaders[name] = value
				}
			}
		}

		if err := cleanupTemporaryTables(tx); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			http.Error(w, "Error committing transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}

		for name, value := range responseHeaders {
			w.Header().Set(name, value)
		}

		w.WriteHeader(statusCode)

		if tplName != "" {
			w.Header().Set("Content-Type", "text/html")
			// Prevent path traversal by ensuring the template path is within webroot
			cleanPath := filepath.Clean(tplName)
			if strings.Contains(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") {
				http.Error(w, "Invalid template path", http.StatusBadRequest)
				return
			}

			templatePath := filepath.Join(app.Config.WebRoot, cleanPath)
			// Ensure the path is still within webroot after cleaning
			if !strings.HasPrefix(filepath.Clean(templatePath), filepath.Clean(app.Config.WebRoot)) {
				http.Error(w, "Invalid template path", http.StatusBadRequest)
				return
			}

			template, err := gonja.FromFile(templatePath)
			if err != nil {
				http.Error(w, "Error loading template: "+err.Error(), http.StatusInternalServerError)
				return
			}

			// Convert results to the expected type for exec.NewContext
			contextData := make(map[string]interface{})
			for key, value := range results {
				contextData[key] = value
			}

			data := exec.NewContext(contextData)

			if err = template.Execute(w, data); err != nil {
				http.Error(w, "Error rendering template: "+err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			// this should probably depend on the content type set into response meta?
			w.Header().Set("Content-Type", "application/json")

			// If results only contains ctx, output just that, otherwise output all results
			var outputData any
			outputData = results
			if len(results) == 1 {
				if ctx, exists := results["ctx"]; exists {
					outputData = ctx
				}
			} else {
				// Remove "ctx" from the results if it exists
				if _, exists := results["ctx"]; exists {
					// Create a copy of results without the "ctx" key
					filteredResults := make(map[string]interface{})
					for key, value := range results {
						if key != "ctx" {
							filteredResults[key] = value
						}
					}
					outputData = filteredResults
				}
			}

			if err := json.NewEncoder(w).Encode(outputData); err != nil {
				http.Error(w, "Error encoding JSON: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

	}
}

func namedParamsToArgs(varsMap map[string]interface{}) []interface{} {
	args := make([]interface{}, 0, len(varsMap))
	for name, value := range varsMap {
		args = append(args, sql.Named(name, value))
	}
	return args
}

// executeQuery runs the SQL query and returns the results
func executeQuery(tx *sql.Tx, query string, varsMap map[string]interface{}) ([]map[string]interface{}, error) {
	rows, err := tx.Query(query, namedParamsToArgs(varsMap)...)
	if err != nil {
		return nil, fmt.Errorf("Error executing SQL: %v", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("Error getting columns: %v", err)
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	var results []map[string]interface{}

	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("Error scanning row: %v", err)
		}

		// Create a map for this row
		row := make(map[string]interface{})
		for i, col := range columns {
			var v interface{}
			val := values[i]
			if b, ok := val.([]byte); ok {
				v = string(b)
			} else {
				v = val
			}
			row[col] = v
		}

		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("Error iterating rows: %v", err)
	}

	return results, nil
}
