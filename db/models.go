package db

import (
	"github.com/arnnvv/peeple-api/pkg/enums"
	"gorm.io/gorm"
)

// Prompt represents a user's prompt response
type Prompt struct {
	gorm.Model
	UserID   uint                 `json:"user_id" gorm:"not null"`
	Category enums.PromptCategory `json:"category" gorm:"type:text;not null"`
	Question string               `json:"question" gorm:"type:text;not null"`
	Answer   string               `json:"answer" gorm:"type:text;not null"`
}

// TableName specifies the table name for Prompt
func (Prompt) TableName() string {
	return "prompts"
}

// AudioPromptModel represents a user's audio prompt
type AudioPromptModel struct {
	gorm.Model
	UserID   uint              `json:"user_id" gorm:"not null;uniqueIndex"`
	Prompt   enums.AudioPrompt `json:"prompt" gorm:"type:text;not null"`
	AudioURL string            `json:"audio_url" gorm:"type:text;not null"`
}

// TableName specifies the table name for AudioPromptModel
func (AudioPromptModel) TableName() string {
	return "audio_prompts"
}

// BeforeSave hook for Prompt
func (p *Prompt) BeforeSave(tx *gorm.DB) error {
	if p.Category == "" {
		return tx.AddError(gorm.ErrInvalidField)
	}
	if p.Question == "" {
		return tx.AddError(gorm.ErrInvalidField)
	}
	if p.Answer == "" {
		return tx.AddError(gorm.ErrInvalidField)
	}
	return nil
}

// BeforeSave hook for AudioPromptModel
func (a *AudioPromptModel) BeforeSave(tx *gorm.DB) error {
	if a.Prompt == "" {
		return tx.AddError(gorm.ErrInvalidField)
	}
	if a.AudioURL == "" {
		return tx.AddError(gorm.ErrInvalidField)
	}
	return nil
}
