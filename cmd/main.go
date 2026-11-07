package main

import (
	"fmt"
	"net/http"

	_ "fras/docs"
	"fras/infrastructure/core"
	"fras/infrastructure/handler"

	httpSwagger "github.com/swaggo/http-swagger"
)

func main() {
	mux := http.NewServeMux()
	
	mux.HandleFunc("POST /api/v1/analyze-stock", handler.HandleAnalytics)
	mux.HandleFunc("/swagger/", httpSwagger.WrapHandler)

	handlerCors := core.CorsMiddleware(mux)

	fmt.Println("FRAS Server started successfully at port 8080...")
	fmt.Println("Swagger UI: http://localhost:8080/swagger/index.html")

	if err := http.ListenAndServe(":8080", handlerCors); err != nil {
		fmt.Printf("Server failed to start: %v\n", err)
	}
}