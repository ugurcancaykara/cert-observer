package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func main() {
	http.HandleFunc("/report", handleReport)
	http.HandleFunc("/health", handleHealth)

	log.Println("Test server starting on :8080")
	log.Println("Endpoints:")
	log.Println("  POST /report - Receives and displays cert-observer reports")
	log.Println("  GET  /health - Health check")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func handleReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Printf("Failed to close request body: %v", err)
		}
	}()

	// Pretty print JSON
	var report interface{}
	if err := json.Unmarshal(body, &report); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		log.Printf("Raw body: %s", string(body))
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	prettyJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Printf("Error formatting JSON: %v", err)
		http.Error(w, "Failed to format JSON", http.StatusInternalServerError)
		return
	}

	// Log the report
	log.Println("======================")
	log.Printf("Report received at %s", time.Now().Format(time.RFC3339))
	log.Println("Full Report:")
	fmt.Println(string(prettyJSON))
	log.Println("======================")

	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintf(w, "Report received successfully\n"); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprintf(w, "OK\n"); err != nil {
		log.Printf("Failed to write health response: %v", err)
	}
}
