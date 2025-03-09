package db

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"github.com/arnnvv/peeple-api/pkg/enums"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"strings"
	"time"
)

type CustomDate time.Time

func (cd CustomDate) IsZero() bool {
	return time.Time(cd).IsZero()
}

func (cd *CustomDate) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}
	*cd = CustomDate(t)
	return nil
}

func (cd CustomDate) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(cd).Format("2006-01-02"))
}

func (cd CustomDate) Value() (driver.Value, error) {
	return time.Time(cd), nil
}

func (cd *CustomDate) Scan(value any) error {
	if value == nil {
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		*cd = CustomDate(v)
		return nil
	default:
		return fmt.Errorf("unsupported type for CustomDate: %T", value)
	}
}

type UserModel struct {
	gorm.Model
	Name               *string                      `json:"name"`
	LastName           *string                      `json:"last_name"`
	PhoneNumber        *string                      `json:"phone_number" gorm:"unique"`
	DateOfBirth        CustomDate                   `json:"date_of_birth"`
	Latitude           *float64                     `json:"latitude"`
	Longitude          *float64                     `json:"longitude"`
	Gender             *enums.Gender                `json:"gender" gorm:"type:text"`
	DatingIntention    *enums.DatingIntention       `json:"dating_intention" gorm:"type:text"`
	Height             *string                      `json:"height"`
	Hometown           *string                      `json:"hometown"`
	JobTitle           *string                      `json:"job_title"`
	Education          *string                      `json:"education"`
	ReligiousBeliefs   *enums.Religion              `json:"religious_beliefs" gorm:"type:text"`
	DrinkingHabit      *enums.DrinkingSmokingHabits `json:"drinking_habit" gorm:"type:text"`
	SmokingHabit       *enums.DrinkingSmokingHabits `json:"smoking_habit" gorm:"type:text"`
	MediaURLs          pq.StringArray               `json:"media_urls" gorm:"type:text[]"`
	VerificationStatus *enums.VerificationStatus    `json:"verification_status" gorm:"type:text;default:'false'"`
	VerificationPic    *string                      `json:"verification_pic"`
	Role               *enums.UserRole              `json:"role" gorm:"type:text;default:'user'"`
	Prompts            []Prompt                     `json:"prompts" gorm:"foreignKey:UserID"`
	AudioPrompt        *AudioPromptModel            `json:"audio_prompt" gorm:"foreignKey:UserID"`
}

func (u UserModel) String() string {
	return fmt.Sprintf(
		"User[ID:%d Name:%v Phone:%s]",
		u.ID,
		u.Name,
		*u.PhoneNumber,
	)
}

func (UserModel) TableName() string {
	return "users"
}

func logDatabaseAction(tx *gorm.DB) {
	fmt.Printf("[DB Operation] SQL: %s\nParams: %+v\n",
		tx.Statement.SQL.String(),
		tx.Statement.Vars,
	)
}

func (u *UserModel) BeforeSave(tx *gorm.DB) error {
	fmt.Printf("[DB Hook] BeforeSave - PhoneNumber: %v\n", u.PhoneNumber)
	if u.PhoneNumber == nil || *u.PhoneNumber == "" {
		return tx.AddError(gorm.ErrInvalidField)
	}
	logDatabaseAction(tx)
	return nil
}

type StringArray []string

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

func (sa StringArray) Value() (driver.Value, error) {
	if sa == nil {
		return nil, nil
	}
	return json.Marshal(sa)
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&UserModel{},
		&Prompt{},
		&AudioPromptModel{},
	)
}
