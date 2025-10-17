package main

import (
	"log"
	"net/http"
	"time"

	"github.com/nikolalohinski/gonja/v2"
	"github.com/nikolalohinski/gonja/v2/exec"
)

func (app *App) serveAdmin(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(app.startedAt).Round(time.Second)
	// Set content type
	w.Header().Set("Content-Type", "text/html")

	// Read the template file
	tpl, err := gonja.FromFile("./templates/index.html")
	if err != nil {
		log.Printf("Error loading template: %v", err)
		http.Error(w, "Error loading template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch routes from the database
	rows, err := app.DB.Query("SELECT method, path, file FROM wtf_routes ORDER BY LENGTH(path)")
	if err != nil {
		log.Printf("Error querying routes: %v", err)
		http.Error(w, "Error querying routes: "+err.Error(), http.StatusInternalServerError)
		return
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

	// Prepare template data with uptime, hits, routes count, and routes list
	tplData := exec.NewContext(map[string]interface{}{
		"ctx": map[string]interface{}{
			"uptime": uptime.String(),
			"hits":   app.hitsProcessed.Load(),
			"routes": app.totalRoutes.Load(),
		},
		"routes_list": routes,
	})

	// Render the template
	err = tpl.Execute(w, tplData)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
