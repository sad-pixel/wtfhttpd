package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/nikolalohinski/gonja/v2"
	"github.com/sad-pixel/wtfhttpd/udfs"
)

func main() {
	gonja.DefaultConfig.AutoEscape = true
	udfs.RegisterUdfs()

	config := LoadConfig()

	db, err := sql.Open("sqlite", config.Db)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	app := &App{
		Config:    config,
		DB:        db,
		startedAt: time.Now(),
	}

	if err := app.reloadRoutes(); err != nil {
		return
	}

	if config.LiveReload {
		log.Println("Starting Live Reloader")
		go app.liveReloader()
	}

	listen := fmt.Sprintf("%s:%d", config.Host, config.Port)

	log.Println("Server starting on ", listen)
	http.ListenAndServe(listen, app)
}
