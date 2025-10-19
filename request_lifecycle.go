package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// setupTemporaryTables creates all necessary temporary tables for the request
func setupTemporaryTables(tx *sql.Tx) error {
	tables := []string{
		`CREATE TEMPORARY TABLE query_params (name TEXT, value TEXT)`,
		`CREATE TEMPORARY TABLE env_vars (name TEXT, value TEXT)`,
		`CREATE TEMPORARY TABLE request_meta (name TEXT, value TEXT)`,
		`CREATE TEMPORARY TABLE request_headers (name TEXT, value TEXT)`,
		`CREATE TEMPORARY TABLE request_form (name TEXT, value TEXT)`,
		`CREATE TEMPORARY TABLE path_params (name TEXT, value TEXT)`,
		`CREATE TEMPORARY TABLE response_meta (name TEXT PRIMARY KEY, value TEXT)`,
		`CREATE TEMPORARY TABLE request_json (path TEXT PRIMARY KEY NOT NULL, value ANY, type TEXT NOT NULL, json TEXT)`,
	}

	for _, table := range tables {
		if _, err := tx.Exec(table); err != nil {
			return fmt.Errorf("Error creating temporary table: %v", err)
		}
	}

	return nil
}

// populateTemporaryTables fills the temporary tables with request data
func populateTemporaryTables(tx *sql.Tx, r *http.Request, pathParams []string, cfg *Config) error {
	stmts := make(map[string]*sql.Stmt)
	tables := []string{"query_params", "request_meta", "request_form", "request_headers", "env_vars", "path_params"}

	for _, table := range tables {
		stmt, err := tx.Prepare(fmt.Sprintf("INSERT INTO %s (name, value) VALUES (?, ?)", table))
		if err != nil {
			return fmt.Errorf("Error preparing statement for %s: %v", table, err)
		}
		defer stmt.Close()
		stmts[table] = stmt
	}

	stmt, err := tx.Prepare("INSERT INTO request_json (path, value, type, json) VALUES (?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("Error preparing statement for request_json: %w", err)
	}
	defer stmt.Close()
	stmts["request_json"] = stmt

	for key, values := range r.URL.Query() {
		for _, value := range values {
			if _, err := stmts["query_params"].Exec(key, value); err != nil {
				return fmt.Errorf("Error inserting query parameter: %v", err)
			}
		}
	}

	for key, values := range r.Header {
		for _, value := range values {
			if _, err := stmts["request_headers"].Exec(key, value); err != nil {
				return fmt.Errorf("Error inserting header: %v", err)
			}
		}
	}

	for _, param := range pathParams {
		if _, err := stmts["path_params"].Exec(param, r.PathValue(param)); err != nil {
			return fmt.Errorf("Error inserting path params: %v", err)
		}
	}

	metaData := []struct {
		name  string
		value string
	}{
		{"method", r.Method},
		{"path", r.URL.Path},
		{"remote_addr", r.RemoteAddr},
		{"protocol", r.Proto},
		{"content_length", fmt.Sprintf("%d", r.ContentLength)},
		{"request_uri", r.RequestURI},
		{"wtf", "100%"},
	}

	for _, meta := range metaData {
		if _, err := stmts["request_meta"].Exec(meta.name, meta.value); err != nil {
			return fmt.Errorf("Error inserting request metadata: %v", err)
		}
	}

	// Only load environment variables with the configured prefix
	for _, envVar := range os.Environ() {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) == 2 && strings.HasPrefix(parts[0], cfg.EnvPrefix) {
			if _, err := stmts["env_vars"].Exec(parts[0], parts[1]); err != nil {
				return fmt.Errorf("Error inserting environment variable: %v", err)
			}
		}
	}

	// Parse form data if content type is application/x-www-form-urlencoded or multipart/form-data
	if strings.Contains(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") ||
		strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseForm(); err != nil {
			return fmt.Errorf("Error parsing form data: %v", err)
		}

		// Insert form values into request_form table
		for key, values := range r.Form {
			for _, value := range values {
				if _, err := stmts["request_form"].Exec(key, value); err != nil {
					return fmt.Errorf("Error inserting form data: %v", err)
				}
			}
		}
	}

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") &&
		r.ContentLength != 0 {
		var data any
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("Error reading request body: %w", err)
		}

		if err := json.Unmarshal(body, &data); err != nil {
			return fmt.Errorf("Error parsing JSON: %w", err)
		}

		rows := make([]JsonRow, 0)
		flattenJson("$", data, &rows)
		if len(rows) != 0 {
			for _, row := range rows {
				_, err := stmt.Exec(row.Path, row.Value, row.Type, row.Json)
				if err != nil {
					return fmt.Errorf("Error inserting row into request_json (%s): %w", row.Path, err)
				}
			}
		}
	}

	return nil
}

// cleanupTemporaryTables drops all temporary tables
func cleanupTemporaryTables(tx *sql.Tx) error {
	tables := []string{
		"query_params",
		"request_meta",
		"request_headers",
		"request_form",
		"env_vars",
		"response_meta",
		"path_params",
		"request_json",
	}

	for _, table := range tables {
		if _, err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table)); err != nil {
			return fmt.Errorf("Error dropping %s table: %v", table, err)
		}
	}

	return nil
}
