package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
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
		effectivePath := filepath.Join("./webroot", path)

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

		if err := populateTemporaryTables(tx, r, pathParams); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		results, err := executeQuery(tx, string(content))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
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
			template, err := gonja.FromFile("./webroot/" + tplName)
			if err != nil {
				http.Error(w, "Error loading template: "+err.Error(), http.StatusInternalServerError)
				return
			}

			data := exec.NewContext(map[string]interface{}{
				"ctx": results,
			})

			if err = template.Execute(w, data); err != nil {
				http.Error(w, "Error rendering template: "+err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			// this should probably depend on the content type set into response meta?
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(results); err != nil {
				http.Error(w, "Error encoding JSON: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

	}
}

// executeQuery runs the SQL query and returns the results
func executeQuery(tx *sql.Tx, query string) ([]map[string]interface{}, error) {
	rows, err := tx.Query(query)
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
