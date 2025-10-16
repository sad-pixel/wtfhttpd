package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
	_ "modernc.org/sqlite"
)

// App holds application-wide dependencies
type App struct {
	DB            *sql.DB
	startedAt     time.Time
	hitsProcessed atomic.Int64
}

// setupRoutes walks through the webroot directory and sets up HTTP routes
func setupRoutes(app *App) error {
	return filepath.Walk("./webroot", func(path string, info os.FileInfo, err error) error {
		return processFile(app, path, info, err)
	})
}

// processFile handles each file found during directory walk
func processFile(app *App, path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if info.IsDir() {
		return nil
	}

	// This code definitely has bugs, but it's ok for now
	relativePath := strings.TrimPrefix(path, "webroot")
	file := filepath.Base(relativePath)
	ext := filepath.Ext(file)

	if ext != ".sql" {
		return nil
	}

	fileName := strings.TrimSuffix(file, ext)
	dir := filepath.Dir(relativePath)

	secondLevelExt := filepath.Ext(fileName)
	methods := []string{".get", ".post", ".put", ".patch", ".delete", ".options"}

	if secondLevelExt != "" && slices.Contains(methods, secondLevelExt) {
		registerMethodSpecificRoute(app, secondLevelExt, fileName, dir, relativePath)
	} else {
		registerGenericRoute(app, fileName, dir, relativePath)
	}

	return nil
}

// registerMethodSpecificRoute registers a route for a specific HTTP method
func registerMethodSpecificRoute(app *App, methodExt, fileName, dir, relativePath string) {
	effectiveFileName := strings.TrimSuffix(fileName, methodExt)

	if effectiveFileName != "index" {
		dir = filepath.Join(dir, effectiveFileName)
	}

	method := strings.TrimPrefix(strings.ToUpper(methodExt), ".")
	fmt.Printf("%s %s -> %s\n", method, dir, relativePath)
	pathPatterns := extractPathParams(fmt.Sprintf("%s %s/{$}", method, dir))
	http.HandleFunc(fmt.Sprintf("%s %s/{$}", method, dir), createHandler(app, relativePath, pathPatterns))
}

// registerGenericRoute registers a route that responds to any HTTP method
func registerGenericRoute(app *App, fileName, dir, relativePath string) {
	if fileName != "index" {
		dir = filepath.Join(dir, fileName)
	}

	fmt.Printf("ANY %s -> %s\n", dir, relativePath)
	pathPatterns := extractPathParams(dir)
	http.HandleFunc(dir, createHandler(app, relativePath, pathPatterns))
}

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

func main() {
	db, err := sql.Open("sqlite", "./wtf.db")
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	app := &App{
		DB:        db,
		startedAt: time.Now(),
	}

	if err := setupRoutes(app); err != nil {
		log.Println(err)
	}

	log.Println("Server starting on localhost:8080")
	// Add a handler for the _wtf endpoint to show server stats
	http.HandleFunc("/_wtf", func(w http.ResponseWriter, r *http.Request) {
		log.Println("/_wtf endpoint accessed")

		// Calculate uptime
		log.Println("Calculating uptime")
		uptime := time.Since(app.startedAt).Round(time.Second)
		log.Printf("Uptime: %s", uptime)

		log.Println("Getting hit count")
		hits := app.hitsProcessed.Load()
		log.Printf("Hits processed: %d", hits)

		// Set content type
		log.Println("Setting content type header")
		w.Header().Set("Content-Type", "text/html")

		// Read the template file
		log.Println("Loading template file from ./templates/index.html")
		tpl, err := gonja.FromFile("./templates/index.html")
		if err != nil {
			log.Printf("Error loading template: %v", err)
			http.Error(w, "Error loading template: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Println("Template loaded successfully")

		// Prepare template data with separate uptime and hits values
		log.Println("Preparing template data")
		tplData := exec.NewContext(map[string]interface{}{
			"ctx": map[string]interface{}{
				"uptime": uptime.String(),
				"hits":   hits,
			},
		})
		log.Println("Template data prepared")

		// Render the template
		log.Println("Rendering template")
		err = tpl.Execute(w, tplData)
		if err != nil {
			log.Printf("Error executing template: %v", err)
			http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
			return
		}
		log.Println("Template rendered successfully")
	})
	http.ListenAndServe("localhost:8080", nil)
}

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
