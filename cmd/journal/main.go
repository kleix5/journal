package main

import (
	"log"

	"journal/internal/app"
)

func main() {
	server, err := app.NewServer("")
	if err != nil {
		log.Fatalf("init server: %v", err)
	}
	defer server.Close()

	log.Println("journal app listening on http://localhost:8080")
	if err := server.Start(":8080"); err != nil {
		log.Fatalf("start server: %v", err)
	}
}
