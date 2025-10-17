package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
)

func (app *App) serveAdmin(w http.ResponseWriter, r *http.Request) {
	if !app.Config.EnableAdmin {
		http.NotFound(w, r)
		return
	}

	// Check for HTTP Basic Authentication
	username, password, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="Admin Access"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate credentials
	if username != app.Config.AdminUsername || password != app.Config.AdminPassword {
		time.Sleep(1 * time.Second) // Prevent brute force attacks
		w.Header().Set("WWW-Authenticate", `Basic realm="Admin Access"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Set content type
	w.Header().Set("Content-Type", "text/html")

	// Check if schema query parameter is present
	schemaParam := r.URL.Query().Get("schema")
	if schemaParam != "" {
		// Render schema.html with table schema
		app.serveSchemaView(w, r, schemaParam)
		return
	}

	// Check if browse query parameter is present
	browseParam := r.URL.Query().Get("browse")
	if browseParam != "" {
		// Render data browser for the table
		app.serveDataBrowser(w, r, browseParam)
		return
	}

	// Check if SQL console is requested
	sqlParam := r.URL.Query().Get("console")
	if sqlParam == "show" {
		app.serveSqlConsole(w, r)
		return
	}

	// Check if it's an AJAX request to execute SQL
	if sqlParam == "execute" && r.Method == "POST" {
		app.executeSQL(w, r)
		return
	}

	// Serve the main admin page
	app.serveAdminIndex(w, r)
}

func (app *App) serveAdminIndex(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(app.startedAt).Round(time.Second)
	startTime := time.Now()

	// Read the template file
	tpl, err := gonja.FromFile("./templates/index.html")
	if err != nil {
		log.Printf("Error loading template: %v", err)
		http.Error(w, "Error loading template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch routes from the database
	routes, err := app.fetchRoutes()
	if err != nil {
		log.Printf("Error fetching routes: %v", err)
		http.Error(w, "Error fetching routes: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch all tables from the database
	tables, err := app.fetchTables()
	if err != nil {
		log.Printf("Error fetching tables: %v", err)
		http.Error(w, "Error fetching tables: "+err.Error(), http.StatusInternalServerError)
		return
	}

	renderTime := time.Since(startTime).Milliseconds()

	// Prepare template data with uptime, hits, routes count, and routes list
	tplData := exec.NewContext(map[string]interface{}{
		"ctx": map[string]interface{}{
			"uptime": uptime.String(),
			"hits":   app.hitsProcessed.Load(),
			"routes": app.totalRoutes.Load(),
		},
		"routes_list": routes,
		"tables_list": tables,
		"render_ms":   renderTime,
	})

	// Render the template
	err = tpl.Execute(w, tplData)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func (app *App) fetchRoutes() ([]map[string]interface{}, error) {
	rows, err := app.DB.Query("SELECT method, path, file FROM wtf_routes ORDER BY LENGTH(path)")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []map[string]interface{}
	for rows.Next() {
		var method, path, file string
		if err := rows.Scan(&method, &path, &file); err != nil {
			log.Printf("Error scanning route row: %v", err)
			continue
		}

		// If path ends with {$}, remove it for display
		if len(path) > 3 && path[len(path)-3:] == "{$}" {
			path = path[:len(path)-3]
		}

		routes = append(routes, map[string]interface{}{
			"method": method,
			"path":   path,
			"file":   file,
		})
	}
	return routes, nil
}

func (app *App) fetchTables() ([]map[string]interface{}, error) {
	tableRows, err := app.DB.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer tableRows.Close()

	var tables []map[string]interface{}
	for tableRows.Next() {
		var tableName string
		if err := tableRows.Scan(&tableName); err != nil {
			log.Printf("Error scanning table row: %v", err)
			continue
		}

		// Get row count for this table
		var count int
		countQuery := "SELECT COUNT(*) FROM " + tableName
		err := app.DB.QueryRow(countQuery).Scan(&count)
		if err != nil {
			log.Printf("Error counting rows in table %s: %v", tableName, err)
			count = -1 // Indicate error in count
		}

		tables = append(tables, map[string]interface{}{
			"name":  tableName,
			"count": count,
		})
	}
	return tables, nil
}

func (app *App) serveSchemaView(w http.ResponseWriter, r *http.Request, tableName string) {
	startTime := time.Now()

	// Read the schema template file
	tpl, err := gonja.FromFile("./templates/schema.html")
	if err != nil {
		log.Printf("Error loading schema template: %v", err)
		http.Error(w, "Error loading template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Query for table schema
	schemaRows, err := app.DB.Query("PRAGMA table_info(" + tableName + ")")
	if err != nil {
		log.Printf("Error querying schema for table %s: %v", tableName, err)
		http.Error(w, "Error querying schema: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer schemaRows.Close()

	var columns []map[string]interface{}
	for schemaRows.Next() {
		var cid int
		var name, dataType string
		var notNull, pk int
		var dfltValue interface{}

		if err := schemaRows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk); err != nil {
			log.Printf("Error scanning schema row: %v", err)
			continue
		}

		columns = append(columns, map[string]interface{}{
			"cid":         cid,
			"name":        name,
			"type":        dataType,
			"not_null":    notNull == 1,
			"default":     dfltValue,
			"primary_key": pk == 1,
		})
	}

	renderTime := time.Since(startTime).Milliseconds()

	// Prepare template data
	tplData := exec.NewContext(map[string]interface{}{
		"table_name": tableName,
		"columns":    columns,
		"render_ms":  renderTime,
	})

	// Render the template
	err = tpl.Execute(w, tplData)
	if err != nil {
		log.Printf("Error executing schema template: %v", err)
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func (app *App) serveDataBrowser(w http.ResponseWriter, r *http.Request, tableName string) {
	startTime := time.Now()

	// Read the data browser template file
	tpl, err := gonja.FromFile("./templates/data_browser.html")
	if err != nil {
		log.Printf("Error loading data browser template: %v", err)
		http.Error(w, "Error loading template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get page parameter for pagination
	page := 1
	if pageParam := r.URL.Query().Get("page"); pageParam != "" {
		fmt.Sscanf(pageParam, "%d", &page)
		if page < 1 {
			page = 1
		}
	}

	// Set page size
	pageSize := 50
	offset := (page - 1) * pageSize

	// Get column names
	columns, err := app.getTableColumns(tableName)
	if err != nil {
		log.Printf("Error getting columns for table %s: %v", tableName, err)
		http.Error(w, "Error getting table columns: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Query for table data with pagination
	query := fmt.Sprintf("SELECT * FROM %s LIMIT %d OFFSET %d", tableName, pageSize, offset)
	rows, err := app.DB.Query(query)
	if err != nil {
		log.Printf("Error querying data for table %s: %v", tableName, err)
		http.Error(w, "Error querying table data: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Get total row count for pagination
	var totalRows int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	err = app.DB.QueryRow(countQuery).Scan(&totalRows)
	if err != nil {
		log.Printf("Error counting rows in table %s: %v", tableName, err)
		totalRows = 0
	}

	// Calculate total pages
	totalPages := (totalRows + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	// Process the rows into a slice of maps
	var data []map[string]interface{}
	for rows.Next() {
		// Create a slice of interface{} to hold the values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))

		// Set up pointers to each interface{}
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		// Scan the row into the slice of interface{}
		if err := rows.Scan(valuePtrs...); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}

		// Create a map for this row
		rowMap := make(map[string]interface{})
		for i, col := range columns {
			var v interface{}
			val := values[i]

			// Handle null values
			if val == nil {
				v = nil
			} else {
				// Try to convert to string for display
				switch val.(type) {
				case []byte:
					v = string(val.([]byte))
				default:
					v = val
				}
			}
			rowMap[col] = v
		}
		data = append(data, rowMap)
	}

	renderTime := time.Since(startTime).Milliseconds()

	// Prepare template data
	tplData := exec.NewContext(map[string]interface{}{
		"table_name":   tableName,
		"columns":      columns,
		"data":         data,
		"current_page": page,
		"total_pages":  totalPages,
		"total_rows":   totalRows,
		"render_ms":    renderTime,
	})

	// Render the template
	err = tpl.Execute(w, tplData)
	if err != nil {
		log.Printf("Error executing data browser template: %v", err)
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func (app *App) getTableColumns(tableName string) ([]string, error) {
	// Query for table schema
	schemaRows, err := app.DB.Query("PRAGMA table_info(" + tableName + ")")
	if err != nil {
		return nil, err
	}
	defer schemaRows.Close()

	var columns []string
	for schemaRows.Next() {
		var cid int
		var name, dataType string
		var notNull, pk int
		var dfltValue interface{}

		if err := schemaRows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk); err != nil {
			log.Printf("Error scanning schema row: %v", err)
			continue
		}

		columns = append(columns, name)
	}
	return columns, nil
}

func (app *App) serveSqlConsole(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Read the SQL console template file
	tpl, err := gonja.FromFile("./templates/sql_console.html")
	if err != nil {
		log.Printf("Error loading SQL console template: %v", err)
		http.Error(w, "Error loading template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get list of tables for autocomplete
	tables, err := app.fetchTables()
	if err != nil {
		log.Printf("Error fetching tables: %v", err)
		http.Error(w, "Error fetching tables: "+err.Error(), http.StatusInternalServerError)
		return
	}

	renderTime := time.Since(startTime).Milliseconds()

	// Prepare template data
	tplData := exec.NewContext(map[string]interface{}{
		"tables":    tables,
		"render_ms": renderTime,
	})

	// Render the template
	err = tpl.Execute(w, tplData)
	if err != nil {
		log.Printf("Error executing SQL console template: %v", err)
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func (app *App) executeSQL(w http.ResponseWriter, r *http.Request) {
	// Parse the SQL query from the request
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error parsing form: "+err.Error(), http.StatusBadRequest)
		return
	}

	query := r.FormValue("query")
	if query == "" {
		http.Error(w, "No SQL query provided", http.StatusBadRequest)
		return
	}

	// Set response content type to JSON
	w.Header().Set("Content-Type", "application/json")

	// Determine if this is a SELECT query or a modification query
	isSelect := strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "SELECT")

	if isSelect {
		// Execute SELECT query
		rows, err := app.DB.Query(query)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
		defer rows.Close()

		// Get column names
		columns, err := rows.Columns()
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Error getting column names: " + err.Error(),
			})
			return
		}

		// Process the rows into a slice of maps
		var results []map[string]interface{}
		for rows.Next() {
			// Create a slice of interface{} to hold the values
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))

			// Set up pointers to each interface{}
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			// Scan the row into the slice of interface{}
			if err := rows.Scan(valuePtrs...); err != nil {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": "Error scanning row: " + err.Error(),
				})
				return
			}

			// Create a map for this row
			rowMap := make(map[string]interface{})
			for i, col := range columns {
				var v interface{}
				val := values[i]

				// Convert byte slices to strings for JSON compatibility
				if b, ok := val.([]byte); ok {
					v = string(b)
				} else {
					v = val
				}

				rowMap[col] = v
			}

			results = append(results, rowMap)
		}

		// Check for errors after iterating through rows
		if err := rows.Err(); err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Error iterating rows: " + err.Error(),
			})
			return
		}

		// Return the results as JSON
		json.NewEncoder(w).Encode(map[string]interface{}{
			"columns": columns,
			"rows":    results,
			"count":   len(results),
		})
	} else {
		// Execute modification query (INSERT, UPDATE, DELETE, etc.)
		result, err := app.DB.Exec(query)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		// Get affected rows
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": "Error getting affected rows: " + err.Error(),
			})
			return
		}

		// Get last insert ID (if applicable)
		lastInsertID, err := result.LastInsertId()

		// Return the result as JSON
		json.NewEncoder(w).Encode(map[string]interface{}{
			"rowsAffected": rowsAffected,
			"lastInsertId": lastInsertID,
		})
	}
}
