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

	// Prepare template data with separate uptime and hits values

	tplData := exec.NewContext(map[string]interface{}{
		"ctx": map[string]interface{}{
			"uptime": uptime.String(),
			"hits":   app.hitsProcessed.Load(),
			"routes": app.totalRoutes.Load(),
		},
	})

	// Render the template
	err = tpl.Execute(w, tplData)
	if err != nil {
		log.Printf("Error executing template: %v", err)
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
