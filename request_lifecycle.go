package main

import (
	"database/sql"
	"fmt"
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
		`CREATE TEMPORARY TABLE path_params (name TEXT, value TEXT)`,
		`CREATE TEMPORARY TABLE response_meta (name TEXT PRIMARY KEY, value TEXT)`,
	}

	for _, table := range tables {
		if _, err := tx.Exec(table); err != nil {
			return fmt.Errorf("Error creating temporary table: %v", err)
		}
	}

	return nil
}

// populateTemporaryTables fills the temporary tables with request data
func populateTemporaryTables(tx *sql.Tx, r *http.Request, pathParams []string) error {
	stmts := make(map[string]*sql.Stmt)
	tables := []string{"query_params", "request_meta", "request_headers", "env_vars", "path_params"}

	for _, table := range tables {
		stmt, err := tx.Prepare(fmt.Sprintf("INSERT INTO %s (name, value) VALUES (?, ?)", table))
		if err != nil {
			return fmt.Errorf("Error preparing statement for %s: %v", table, err)
		}
		defer stmt.Close()
		stmts[table] = stmt
	}

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

	// maybe risky but idk
	for _, envVar := range os.Environ() {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) == 2 {
			if _, err := stmts["env_vars"].Exec(parts[0], parts[1]); err != nil {
				return fmt.Errorf("Error inserting environment variable: %v", err)
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
		"env_vars",
		"response_meta",
		"path_params",
	}

	for _, table := range tables {
		if _, err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table)); err != nil {
			return fmt.Errorf("Error dropping %s table: %v", table, err)
		}
	}

	return nil
}
