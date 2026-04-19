package factory

import "github.com/google/uuid"

// SystemBase is embedded by every concrete system instance. Quality is
// inherited through ManufacturerID → CivilizationID → TechTier, never
// stored on the system itself (see Phase3_Plan §2).
type SystemBase struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	ArchetypeName  string    `json:"archetype"`
	ManufacturerID string    `json:"manufacturer_id"`
	SerialNumber   string    `json:"serial_number"`
	Health         float64   `json:"health"`
}
