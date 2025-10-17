package main

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/nikolalohinski/gonja/v2"
	"github.com/sad-pixel/wtfhttpd/udfs"
)

func main() {
	gonja.DefaultConfig.AutoEscape = true
	udfs.RegisterUdfs()

	db, err := sql.Open("sqlite", "./wtf.db")
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	app := &App{
		DB:        db,
		startedAt: time.Now(),
	}

	if err := app.reloadRoutes(); err != nil {
		return
	}

	go app.liveReloader()

	log.Println("Server starting on localhost:8080")
	// // Add a handler for the _wtf endpoint to show server stats
	// http.HandleFunc("/_wtf", app.serveAdmin)
	http.ListenAndServe("localhost:8080", app)
}
