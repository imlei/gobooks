// 遵循产品需求 v1.0
package models

import (
	"fmt"
	"time"
)

// EntityType is a strict enum for company type.
// Required by PROJECT_GUIDE: must not be a free-form string.
type EntityType string

const (
	EntityTypePersonal      EntityType = "Personal"
	EntityTypeIncorporated  EntityType = "Incorporated"
	EntityTypeLLP           EntityType = "LLP"
)

func (t EntityType) Valid() bool {
	switch t {
	case EntityTypePersonal, EntityTypeIncorporated, EntityTypeLLP:
		return true
	default:
		return false
	}
}

func ParseEntityType(s string) (EntityType, error) {
	t := EntityType(s)
	if !t.Valid() {
		return "", fmt.Errorf("invalid entity type: %q", s)
	}
	return t, nil
}

// BusinessType is a strict enum for high-level business type.
type BusinessType string

const (
	BusinessTypeRetail           BusinessType = "Retail"
	BusinessTypeProfessionalCorp BusinessType = "Professional Corp"
)

func (t BusinessType) Valid() bool {
	switch t {
	case BusinessTypeRetail, BusinessTypeProfessionalCorp:
		return true
	default:
		return false
	}
}

func ParseBusinessType(s string) (BusinessType, error) {
	t := BusinessType(s)
	if !t.Valid() {
		return "", fmt.Errorf("invalid business type: %q", s)
	}
	return t, nil
}

// Industry is a strict enum for a simple controlled industry list.
// This keeps the UI simple (dropdown) while keeping data clean.
type Industry string

const (
	IndustryRetail        Industry = "Retail"
	IndustryConsulting    Industry = "Consulting"
	IndustryServices      Industry = "Services"
	IndustryManufacturing Industry = "Manufacturing"
	IndustryConstruction  Industry = "Construction"
	IndustryOther         Industry = "Other"
)

func (i Industry) Valid() bool {
	switch i {
	case IndustryRetail,
		IndustryConsulting,
		IndustryServices,
		IndustryManufacturing,
		IndustryConstruction,
		IndustryOther:
		return true
	default:
		return false
	}
}

func ParseIndustry(s string) (Industry, error) {
	i := Industry(s)
	if !i.Valid() {
		return "", fmt.Errorf("invalid industry: %q", s)
	}
	return i, nil
}

// Company stores the company profile created during first-time setup.
// The setup wizard will create exactly one row for MVP.
type Company struct {
	ID uint `gorm:"primaryKey"`

	Name           string       `gorm:"not null"`
	EntityType     EntityType   `gorm:"type:text;not null"`
	BusinessType   BusinessType `gorm:"type:text;not null"`
	Industry       Industry     `gorm:"type:text;not null"`
	IncorporatedDate string     `gorm:"not null"`
	FiscalYearEnd  string       `gorm:"not null"` // keep as string for now; e.g. "12-31"
	BusinessNumber string       `gorm:"not null"`

	AddressLine string `gorm:"not null"`
	Province    string `gorm:"not null"`
	PostalCode  string `gorm:"not null"`
	Country     string `gorm:"not null"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

