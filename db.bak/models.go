package bak

type Prompt struct {
	gorm.Model
	UserID   uint                 `json:"user_id" gorm:"not null"`
	Category enums.PromptCategory `json:"category" gorm:"type:text;not null"`
	Question string               `json:"question" gorm:"type:text;not null"`
	Answer   string               `json:"answer" gorm:"type:text;not null"`
}

func (Prompt) TableName() string {
	return "prompts"
}

type AudioPromptModel struct {
	gorm.Model
	UserID   uint              `json:"user_id" gorm:"not null;uniqueIndex"`
	Prompt   enums.AudioPrompt `json:"prompt" gorm:"type:text;not null"`
	AudioURL string            `json:"audio_url" gorm:"type:text;not null"`
}

func (AudioPromptModel) TableName() string {
	return "audio_prompts"
}

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

func (a *AudioPromptModel) BeforeSave(tx *gorm.DB) error {
	if a.Prompt == "" {
		return tx.AddError(gorm.ErrInvalidField)
	}
	if a.AudioURL == "" {
		return tx.AddError(gorm.ErrInvalidField)
	}
	return nil
}
