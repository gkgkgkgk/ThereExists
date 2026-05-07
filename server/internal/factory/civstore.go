package factory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrCivNotFound is returned by LoadCivilization when no row matches the
// provided id. Callers use errors.Is to distinguish this from
// infrastructure errors.
var ErrCivNotFound = errors.New("civilization not found")

// SaveCivilization writes a generated civilization (and its homeworld
// planet) to the civilizations table. The TechProfile and Planet are
// stored as JSONB so they can evolve without schema migrations.
//
// Caller must supply the FactoryVersion string from the assembly
// package; civstore takes it as a parameter rather than importing
// assembly to keep the dependency direction one-way.
func SaveCivilization(ctx context.Context, db *sql.DB, civ *Civilization, planet *Planet, factoryVersion string) error {
	if civ == nil {
		return errors.New("SaveCivilization: civ is nil")
	}
	if planet == nil {
		return errors.New("SaveCivilization: planet is nil")
	}
	profileJSON, err := json.Marshal(civ.TechProfile)
	if err != nil {
		return fmt.Errorf("marshal TechProfile: %w", err)
	}
	planetJSON, err := json.Marshal(planet)
	if err != nil {
		return fmt.Errorf("marshal Planet: %w", err)
	}
	const q = `INSERT INTO civilizations
		(id, name, description, homeworld_desc, age_years, tech_tier, flavor, profile, planet, factory_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
	if _, err := db.ExecContext(ctx, q,
		civ.ID,
		civ.Name,
		civ.Description,
		civ.HomeworldDescription,
		civ.AgeYears,
		civ.TechTier,
		civ.Flavor,
		profileJSON,
		planetJSON,
		factoryVersion,
	); err != nil {
		return fmt.Errorf("insert civilization: %w", err)
	}
	return nil
}

// LoadCivilization reads a previously-saved civilization by id. Returns
// ErrCivNotFound (wrapped) when no row matches.
func LoadCivilization(ctx context.Context, db *sql.DB, id string) (*Civilization, *Planet, error) {
	const q = `SELECT id, name, description, homeworld_desc, age_years, tech_tier, flavor, profile, planet
		FROM civilizations WHERE id = $1`
	var (
		civ                                                Civilization
		profileJSON, planetJSON                            []byte
	)
	err := db.QueryRowContext(ctx, q, id).Scan(
		&civ.ID,
		&civ.Name,
		&civ.Description,
		&civ.HomeworldDescription,
		&civ.AgeYears,
		&civ.TechTier,
		&civ.Flavor,
		&profileJSON,
		&planetJSON,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, fmt.Errorf("%w: %s", ErrCivNotFound, id)
		}
		return nil, nil, fmt.Errorf("select civilization: %w", err)
	}
	if err := json.Unmarshal(profileJSON, &civ.TechProfile); err != nil {
		return nil, nil, fmt.Errorf("unmarshal TechProfile: %w", err)
	}
	var planet Planet
	if err := json.Unmarshal(planetJSON, &planet); err != nil {
		return nil, nil, fmt.Errorf("unmarshal Planet: %w", err)
	}
	return &civ, &planet, nil
}
