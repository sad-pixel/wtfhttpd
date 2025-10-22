package main

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gernest/front"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

func (app *App) indexContent() error {
	log.Println("Indexing content...")
	startTime := time.Now()
	fileCount := 0

	tx, err := app.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Drop existing tables if they exist
	_, err = tx.Exec(`DROP TABLE IF EXISTS posts_fts`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`DROP TABLE IF EXISTS wtf_content`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS wtf_content (
			id INTEGER PRIMARY KEY,
			title TEXT NOT NULL,
			path TEXT UNIQUE NOT NULL,
			slug TEXT UNIQUE NOT NULL,
			metadata TEXT DEFAULT '{}',
			raw_body TEXT DEFAULT '',
			html_body TEXT DEFAULT ''
		)`)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS posts_fts USING fts5(
			title,
			content,
			tokenize = 'porter unicode61'
		)`)
	if err != nil {
		return err
	}

	// Prepare statements for inserting into wtf_content and posts_fts tables
	stmtContent, err := tx.Prepare(`
		INSERT INTO wtf_content (title, path, slug, metadata, raw_body, html_body)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			title = excluded.title,
			slug = excluded.slug,
			metadata = excluded.metadata,
			raw_body = excluded.raw_body,
			html_body = excluded.html_body
	`)
	if err != nil {
		log.Printf("Error preparing content insert statement: %v", err)
		return err
	}
	defer stmtContent.Close()

	stmtFTS, err := tx.Prepare(`
		INSERT INTO posts_fts (title, content)
		VALUES (?, ?)
	`)
	if err != nil {
		log.Printf("Error preparing FTS insert statement: %v", err)
		return err
	}
	defer stmtFTS.Close()

	m := front.NewMatter()
	m.Handle("---", front.YAMLHandler)
	walkErr := filepath.Walk(
		filepath.Join(app.Config.WebRoot, "content"),
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			fileName := filepath.Base(path)
			relativePath := strings.TrimPrefix(path, filepath.Join(app.Config.WebRoot, "content")+"/")
			ext := filepath.Ext(path)

			if ext == ".md" || ext == ".markdown" {
				log.Println("Found markdown file: ", relativePath, fileName)
				fileHandle, err := os.Open(path)
				if err != nil {
					log.Println("Error opening file: ", err)
					return err
				}
				defer fileHandle.Close()

				frontMatter, body, err := m.Parse(fileHandle)
				if err != nil {
					log.Printf("Error parsing markdown file %s: %v", relativePath, err)
					return nil
				}

				// Convert front matter to JSON for storage
				metadataJSON, err := json.Marshal(frontMatter)
				if err != nil {
					log.Printf("Error marshaling metadata for %s: %v", relativePath, err)
					return nil
				}

				// Generate slug from filename
				slug := strings.TrimSuffix(fileName, ext)

				// Get title from front matter or use filename
				title := fileName
				if t, ok := frontMatter["title"].(string); ok {
					title = t
				}

				// Render markdown to HTML using goldmark
				var buf bytes.Buffer
				markdown := goldmark.New(
					goldmark.WithExtensions(extension.GFM),
					goldmark.WithParserOptions(
						parser.WithAutoHeadingID(),
					),
					goldmark.WithRendererOptions(
						html.WithHardWraps(),
						html.WithXHTML(),
					),
				)
				if err := markdown.Convert([]byte(body), &buf); err != nil {
					log.Printf("Error rendering markdown for %s: %v", relativePath, err)
					return nil
				}
				renderedHTML := buf.String()

				// Insert into content table
				_, err = stmtContent.Exec(title, relativePath, slug, string(metadataJSON), body, renderedHTML)
				if err != nil {
					log.Printf("Error inserting content for %s: %v", relativePath, err)
					return nil
				}

				// Insert into FTS table
				_, err = stmtFTS.Exec(title, body)
				if err != nil {
					log.Printf("Error inserting into FTS for %s: %v", relativePath, err)
					return nil
				}

				log.Printf("Inserted %s into database", relativePath)
				fileCount++
			}
			return nil
		})

	if walkErr != nil {
		log.Println("Error walking: ", walkErr)
		return walkErr
	}
	if err := tx.Commit(); err != nil {
		log.Println("Error committing transaction:", err)
		return err
	}

	elapsed := time.Since(startTime)
	log.Printf("Content indexing complete: %d files indexed in %v", fileCount, elapsed)

	return nil
}
