package main

import (
	"fmt"
	"net/http"

	"backend/infrastructure"
)

func main() {
	http.HandleFunc("/analytics-emiten", infrastructure.HandleAnalytics)
	fmt.Println("Financial Analytics Server started successfully at port 8080...")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Printf("Server failed to start: %v\n", err)
	}
}
