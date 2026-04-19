package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gkgkgkgk/ThereExists/server/internal/db"
	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
	"github.com/gkgkgkgk/ThereExists/server/internal/factory/flight"
	"github.com/gkgkgkgk/ThereExists/server/internal/handlers"
	"github.com/rs/cors"
)

func main() {
	database, err := db.Connect(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	// Wire the factory's manufacturer picker into the flight dispatcher.
	// Done here (not in factory/init) to keep the factory → flight edge
	// one-way and avoid an import cycle.
	flight.SetManufacturerPicker(factory.PickManufacturer)

	ph := handlers.NewPlayerHandler(database)
	sh := handlers.NewShipHandler(database)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/player", ph.GetPlayer)
	mux.HandleFunc("POST /api/ships/generate", sh.Generate)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "http://localhost:5173"
	}

	c := cors.New(cors.Options{
		AllowedOrigins:   strings.Split(allowedOrigins, ","),
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: false,
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, c.Handler(mux)))
}
