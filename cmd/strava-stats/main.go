package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"
)

func main() {
	port := ":8080"

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := template.ParseFiles("web/templates/index.html")
		if err != nil {
			http.Error(w, "Could not load template", http.StatusInternalServerError)
			log.Printf("Error parsing template: %v", err)
			return
		}
		if err := tmpl.Execute(w, nil); err != nil {
			log.Printf("Error executing template: %v", err)
		}
	})
	
	server := &http.Server{
		Addr:         port,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	fmt.Printf("Starting server on http://localhost%s\n", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}
