package db

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/arnnvv/peeple-api/pkg/enums"
	"gorm.io/gorm"
)

// UserModel represents a user in the system
type UserModel struct {
	gorm.Model
	Name             *string                      `json:"name"`
	LastName         *string                      `json:"last_name"`
	PhoneNumber      string                       `json:"phone_number" gorm:"unique;not null"`
	DateOfBirth      *time.Time                   `json:"date_of_birth"`
	Latitude         *float64                     `json:"latitude"`
	Longitude        *float64                     `json:"longitude"`
	Gender           *enums.Gender                `json:"gender" gorm:"type:text"`
	DatingIntention  *enums.DatingIntention       `json:"dating_intention" gorm:"type:text"`
	Height           *string                      `json:"height"`
	Hometown         *string                      `json:"hometown"`
	JobTitle         *string                      `json:"job_title"`
	Education        *string                      `json:"education"`
	ReligiousBeliefs *enums.Religion              `json:"religious_beliefs" gorm:"type:text"`
	DrinkingHabit    *enums.DrinkingSmokingHabits `json:"drinking_habit" gorm:"type:text"`
	SmokingHabit     *enums.DrinkingSmokingHabits `json:"smoking_habit" gorm:"type:text"`
	MediaURLs        []string                     `json:"media_urls" gorm:"type:text[]"`
	Prompts          []Prompt                     `json:"prompts" gorm:"foreignKey:UserID"`
	AudioPrompt      *AudioPromptModel            `json:"audio_prompt" gorm:"foreignKey:UserID"`
}

// TableName specifies the table name for UserModel
func (UserModel) TableName() string {
	return "users"
}

// BeforeSave hook for UserModel
func (u *UserModel) BeforeSave(tx *gorm.DB) error {
	if u.PhoneNumber == "" {
		return tx.AddError(gorm.ErrInvalidField)
	}
	return nil
}

// Scan implementations for enum types
// Custom GORM type for string array
type StringArray []string

// Scan implements the sql.Scanner interface for StringArray
func (sa *StringArray) Scan(value any) error {
	if value == nil {
		*sa = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, sa)
	case string:
		return json.Unmarshal([]byte(v), sa)
	default:
		return fmt.Errorf("unsupported type for StringArray: %T", value)
	}
}

// Value implements the driver.Valuer interface for StringArray
func (sa StringArray) Value() (driver.Value, error) {
	if sa == nil {
		return nil, nil
	}
	return json.Marshal(sa)
}

// AutoMigrate creates or updates the database schema
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&UserModel{},
		&Prompt{},
		&AudioPromptModel{},
	)
}
