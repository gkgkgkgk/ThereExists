package factory_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/gkgkgkgk/ThereExists/server/internal/db"
	"github.com/gkgkgkgk/ThereExists/server/internal/factory"
)

// civstoreTestDB returns a connection backed by TEST_DATABASE_URL or
// skips the test if the env var is unset. Migrations run on every call —
// they're idempotent.
func civstoreTestDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping civstore round-trip test")
	}
	conn, err := db.Connect(url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := db.Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return conn
}

func TestSaveLoadRoundTrip(t *testing.T) {
	conn := civstoreTestDB(t)
	defer conn.Close()
	ctx := context.Background()

	planet, err := factory.GeneratePlanet(42)
	if err != nil {
		t.Fatalf("GeneratePlanet: %v", err)
	}
	civ := &factory.Civilization{
		ID:                   uuid.NewString(),
		Name:                 "Roundtrip Civ",
		Description:          "A civilization used to validate JSONB persistence.",
		HomeworldDescription: "Synthetic homeworld description.",
		HomeworldPlanetID:    nil,
		AgeYears:             12345,
		TechTier:             3,
		TechProfile: factory.TechProfile{
			DesignPhilosophy:        "utilitarian austerity",
			PreferredCoolingMethods: []factory.CoolingMethod{factory.Regenerative, factory.Ablative},
			PreferredIgnitionTypes:  []factory.IgnitionMethod{factory.Hypergolic},
			PreferredMixtureIDs:     []string{"LOX_LH2", "MMH_NTO"},
			AversionToCryogenics:    0.6,
			FarDriveFamily:          "fusion",
			RiskTolerance:           0.25,
			ThrustVsIspPreference:   0.4,
		},
		Flavor: "test fixture",
	}

	if err := factory.SaveCivilization(ctx, conn, civ, planet, "test-version"); err != nil {
		t.Fatalf("SaveCivilization: %v", err)
	}

	loadedCiv, loadedPlanet, err := factory.LoadCivilization(ctx, conn, civ.ID)
	if err != nil {
		t.Fatalf("LoadCivilization: %v", err)
	}
	if !reflect.DeepEqual(loadedCiv, civ) {
		t.Errorf("civ mismatch:\n got: %#v\nwant: %#v", loadedCiv, civ)
	}
	if !reflect.DeepEqual(loadedPlanet, planet) {
		t.Errorf("planet mismatch:\n got: %#v\nwant: %#v", loadedPlanet, planet)
	}
}

func TestLoadCivNotFound(t *testing.T) {
	conn := civstoreTestDB(t)
	defer conn.Close()
	_, _, err := factory.LoadCivilization(context.Background(), conn, uuid.NewString())
	if !errors.Is(err, factory.ErrCivNotFound) {
		t.Fatalf("expected ErrCivNotFound, got %v", err)
	}
}
