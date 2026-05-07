package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	_ "github.com/gkgkgkgk/ThereExists/server/api"
	"github.com/gkgkgkgk/ThereExists/server/internal/db"
	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
	"github.com/gkgkgkgk/ThereExists/server/internal/factory/flight"
	"github.com/gkgkgkgk/ThereExists/server/internal/handlers"
	"github.com/gkgkgkgk/ThereExists/server/internal/llm"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

// @title           ThereExists API
// @version         1.0
// @description     Backend API for the ThereExists game.
// @BasePath        /

func main() {
	// Load .env from the repo root (one level up from the server dir
	// when running `go run ./cmd/server`). Silent on missing file —
	// production uses real env vars, not a committed .env. Existing env
	// vars take precedence over .env contents (godotenv.Load default).
	for _, path := range []string{".env", "../.env"} {
		if err := godotenv.Load(path); err == nil {
			log.Printf("loaded env from %s", path)
			break
		}
	}

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
	flight.SetCivTechTierLookup(func(id string) (int, bool) {
		c, ok := factory.Civilizations[id]
		if !ok {
			return 0, false
		}
		return c.TechTier, true
	})

	// LLM client is optional — if OPENAI_API_KEY is missing, the civ
	// endpoint 503s but the rest of the server still boots.
	llmClient, err := llm.NewOpenAIClient()
	if err != nil {
		log.Printf("llm: %v; /api/civilizations/generate will return 503", err)
		llmClient = nil
	}

	sh := handlers.NewShipHandler(database, llmClient)
	ch := handlers.NewCivilizationHandler(llmClient)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/ships/generate", sh.Generate)
	mux.HandleFunc("POST /api/civilizations/generate", ch.Generate)
	mux.HandleFunc("GET /api/health", healthCheck)

	mux.Handle("GET /swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

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

// healthCheck godoc
// @Summary      Health check
// @Description  Returns "ok" if the server is running.
// @Tags         health
// @Produce      plain
// @Success      200  {string}  string  "ok"
// @Router       /api/health [get]
func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
