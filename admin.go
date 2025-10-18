package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
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

	username, password, ok := r.BasicAuth()
	if !ok {
		w.Header().Set("WWW-Authenticate", `Basic realm="Admin Access"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if username != app.Config.AdminUsername || password != app.Config.AdminPassword {
		time.Sleep(1 * time.Second) // Prevent brute force attacks
		w.Header().Set("WWW-Authenticate", `Basic realm="Admin Access"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "text/html")

	schemaParam := r.URL.Query().Get("schema")
	if schemaParam != "" {
		app.serveSchemaView(w, r, schemaParam)
		return
	}

	browseParam := r.URL.Query().Get("browse")
	if browseParam != "" {
		app.serveDataBrowser(w, r, browseParam)
		return
	}

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

	app.serveAdminIndex(w, r)
}

func (app *App) serveAdminIndex(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(app.startedAt).Round(time.Second)
	startTime := time.Now()

	tpl, err := gonja.FromFile("./templates/index.html")
	if err != nil {
		log.Printf("Error loading template: %v", err)
		http.Error(w, "Error loading template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	routes, err := app.fetchRoutes()
	if err != nil {
		log.Printf("Error fetching routes: %v", err)
		http.Error(w, "Error fetching routes: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tables, err := app.fetchTables()
	if err != nil {
		log.Printf("Error fetching tables: %v", err)
		http.Error(w, "Error fetching tables: "+err.Error(), http.StatusInternalServerError)
		return
	}

	renderTime := time.Since(startTime).Milliseconds()

	tplData := exec.NewContext(map[string]any{
		"ctx": map[string]any{
			"uptime": uptime.String(),
			"hits":   app.hitsProcessed.Load(),
			"routes": app.totalRoutes.Load(),
		},
		"routes_list": routes,
		"tables_list": tables,
		"render_ms":   renderTime,
	})

	err = tpl.Execute(w, tplData)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func (app *App) fetchRoutes() ([]map[string]any, error) {
	rows, err := app.DB.Query("SELECT method, path, file FROM wtf_routes ORDER BY LENGTH(path)")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routes []map[string]any
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

		routes = append(routes, map[string]any{
			"method": method,
			"path":   path,
			"file":   file,
		})
	}
	return routes, nil
}

func (app *App) fetchTables() ([]map[string]any, error) {
	tableRows, err := app.DB.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer tableRows.Close()

	var tables []map[string]any
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

		tables = append(tables, map[string]any{
			"name":  tableName,
			"count": count,
		})
	}
	return tables, nil
}

func (app *App) serveSchemaView(w http.ResponseWriter, r *http.Request, tableName string) {
	startTime := time.Now()

	tpl, err := gonja.FromFile("./templates/schema.html")
	if err != nil {
		log.Printf("Error loading schema template: %v", err)
		http.Error(w, "Error loading template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	validatedTableName, err := app.validateTableName(tableName)
	if err != nil {
		log.Printf("Table validation error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tableName = validatedTableName

	// unfortunately we can't parameterize the table name here as it gives a syntax error.
	schemaRows, err := app.DB.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		log.Printf("Error querying schema for table %s: %v", tableName, err)
		http.Error(w, "Error querying schema: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer schemaRows.Close()

	var columns []map[string]any
	for schemaRows.Next() {
		var cid int
		var name, dataType string
		var notNull, pk int
		var dfltValue any

		if err := schemaRows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk); err != nil {
			log.Printf("Error scanning schema row: %v", err)
			continue
		}

		columns = append(columns, map[string]any{
			"cid":         cid,
			"name":        name,
			"type":        dataType,
			"not_null":    notNull == 1,
			"default":     dfltValue,
			"primary_key": pk == 1,
		})
	}

	renderTime := time.Since(startTime).Milliseconds()

	tplData := exec.NewContext(map[string]any{
		"table_name": tableName,
		"columns":    columns,
		"render_ms":  renderTime,
	})

	err = tpl.Execute(w, tplData)
	if err != nil {
		log.Printf("Error executing schema template: %v", err)
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// Validate that the requested table exists
func (app *App) validateTableName(tableName string) (string, error) {
	tableRows, err := app.DB.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		return "", fmt.Errorf("error querying tables: %v", err)
	}
	defer tableRows.Close()

	var tables []string
	for tableRows.Next() {
		var name string
		if err := tableRows.Scan(&name); err != nil {
			log.Printf("Error scanning table name: %v", err)
			continue
		}
		tables = append(tables, name)
	}

	tableExists := false
	validTableName := tableName
	for _, name := range tables {
		if strings.EqualFold(name, tableName) {
			// Use the actual table name from the database for subsequent queries
			validTableName = name
			tableExists = true
			break
		}
	}

	if !tableExists {
		return "", fmt.Errorf("table '%s' not found", tableName)
	}

	// Validate that the table name contains only alphanumeric characters and underscores
	// This is required to prevent 2nd order SQL injection
	if matched, err := regexp.MatchString("^[a-zA-Z0-9_]+$", validTableName); err != nil {
		return "", fmt.Errorf("error validating table name: %v", err)
	} else if !matched {
		return "", fmt.Errorf("invalid table name: '%s' contains characters other than letters, numbers, and underscores", validTableName)
	}

	return validTableName, nil
}

func (app *App) serveDataBrowser(w http.ResponseWriter, r *http.Request, tableName string) {
	startTime := time.Now()

	validatedTableName, err := app.validateTableName(tableName)
	if err != nil {
		log.Printf("Table validation error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tableName = validatedTableName

	tpl, err := gonja.FromFile("./templates/data_browser.html")
	if err != nil {
		log.Printf("Error loading data browser template: %v", err)
		http.Error(w, "Error loading template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	page := 1
	if pageParam := r.URL.Query().Get("page"); pageParam != "" {
		fmt.Sscanf(pageParam, "%d", &page)
		if page < 1 {
			page = 1
		}
	}

	pageSize := 50
	offset := (page - 1) * pageSize

	columns, err := app.getTableColumns(tableName)
	if err != nil {
		log.Printf("Error getting columns for table %s: %v", tableName, err)
		http.Error(w, "Error getting table columns: "+err.Error(), http.StatusInternalServerError)
		return
	}

	query := fmt.Sprintf("SELECT * FROM %s LIMIT ? OFFSET ?", tableName)
	rows, err := app.DB.Query(query, pageSize, offset)
	if err != nil {
		log.Printf("Error querying data for table %s: %v", tableName, err)
		http.Error(w, "Error querying table data: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var totalRows int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	err = app.DB.QueryRow(countQuery).Scan(&totalRows)
	if err != nil {
		log.Printf("Error counting rows in table %s: %v", tableName, err)
		totalRows = 0
	}

	totalPages := (totalRows + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	var data []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))

		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}

		rowMap := make(map[string]any)
		for i, col := range columns {
			var v any
			val := values[i]

			if val == nil {
				v = nil
			} else {
				switch val := val.(type) {
				case []byte:
					v = string(val)
				default:
					v = val
				}
			}
			rowMap[col] = v
		}
		data = append(data, rowMap)
	}

	renderTime := time.Since(startTime).Milliseconds()

	tplData := exec.NewContext(map[string]any{
		"table_name":   tableName,
		"columns":      columns,
		"data":         data,
		"current_page": page,
		"total_pages":  totalPages,
		"total_rows":   totalRows,
		"render_ms":    renderTime,
	})

	err = tpl.Execute(w, tplData)
	if err != nil {
		log.Printf("Error executing data browser template: %v", err)
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func (app *App) getTableColumns(tableName string) ([]string, error) {
	// unfortunately we can't parameterize the table name here as it gives a syntax error.
	schemaRows, err := app.DB.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		log.Println("here")
		return nil, err
	}
	defer schemaRows.Close()

	var columns []string
	for schemaRows.Next() {
		var cid int
		var name, dataType string
		var notNull, pk int
		var dfltValue any

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

	tpl, err := gonja.FromFile("./templates/sql_console.html")
	if err != nil {
		log.Printf("Error loading SQL console template: %v", err)
		http.Error(w, "Error loading template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tables, err := app.fetchTables()
	if err != nil {
		log.Printf("Error fetching tables: %v", err)
		http.Error(w, "Error fetching tables: "+err.Error(), http.StatusInternalServerError)
		return
	}

	renderTime := time.Since(startTime).Milliseconds()

	tplData := exec.NewContext(map[string]any{
		"tables":    tables,
		"render_ms": renderTime,
	})

	err = tpl.Execute(w, tplData)
	if err != nil {
		log.Printf("Error executing SQL console template: %v", err)
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func (app *App) executeSQL(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error parsing form: "+err.Error(), http.StatusBadRequest)
		return
	}

	query := r.FormValue("query")
	if query == "" {
		http.Error(w, "No SQL query provided", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	isSelect := strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "SELECT")

	if isSelect {
		rows, err := app.DB.Query(query)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{
				"error": err.Error(),
			})
			return
		}
		defer rows.Close()

		columns, err := rows.Columns()
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{
				"error": "Error getting column names: " + err.Error(),
			})
			return
		}

		var results []map[string]any
		for rows.Next() {
			values := make([]any, len(columns))
			valuePtrs := make([]any, len(columns))

			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				json.NewEncoder(w).Encode(map[string]any{
					"error": "Error scanning row: " + err.Error(),
				})
				return
			}

			rowMap := make(map[string]any)
			for i, col := range columns {
				var v any
				val := values[i]

				if b, ok := val.([]byte); ok {
					v = string(b)
				} else {
					v = val
				}

				rowMap[col] = v
			}

			results = append(results, rowMap)
		}

		if err := rows.Err(); err != nil {
			json.NewEncoder(w).Encode(map[string]any{
				"error": "Error iterating rows: " + err.Error(),
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]any{
			"columns": columns,
			"rows":    results,
			"count":   len(results),
		})
	} else {
		result, err := app.DB.Exec(query)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{
				"error": err.Error(),
			})
			return
		}

		rowsAffected, err := result.RowsAffected()
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{
				"error": "Error getting affected rows: " + err.Error(),
			})
			return
		}

		lastInsertID, err := result.LastInsertId()

		json.NewEncoder(w).Encode(map[string]any{
			"rowsAffected": rowsAffected,
			"lastInsertId": lastInsertID,
		})
	}
}
