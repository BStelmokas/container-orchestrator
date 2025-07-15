package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"orchestrator/internal/registry"
)

var reg = registry.NewRegistry()

func main() {
	http.HandleFunc("/register", handleRegister)
	http.HandleFunc("/lookup", handleLookup)
	http.HandleFunc("/deregister", handleDeregister)
	http.HandleFunc("/lookup-all", handleLookupAll)

	log.Println("Service Registry running on :8000")
	if err := http.ListenAndServe(":8000", nil); err != nil {
		log.Fatalf("HTTP server error: %v", err)
	}
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	ip := r.URL.Query().Get("ip")
	portStr := r.URL.Query().Get("port")

	if name == "" || ip == "" || portStr == "" {
		http.Error(w, "Missing required query parameters", http.StatusBadRequest)
		return
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		http.Error(w, "Invalid port value", http.StatusBadRequest)
		return
	}

	reg.Register(name, ip, port)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Registered service"))
}

func handleLookup(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "Missing 'name' parameter", http.StatusBadRequest)
		return
	}

	entry, found := reg.Lookup(name)
	if !found {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entry)
}

func handleDeregister(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "Missing 'name' parameter", http.StatusBadRequest)
		return
	}

	// Remove service from registry
	reg.Delete(name)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Deregistered service"))
}

func handleLookupAll(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "Missing 'name' parameter", http.StatusBadRequest)
		return
	}

	// For now, pretend we only support 1 instance per name
	entry, found := reg.Lookup(name)
	if !found {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Return as a slice of one element
	entries := []registry.ServiceEntry{entry}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}
