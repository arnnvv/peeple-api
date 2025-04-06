package bak

import (
	"time"
)

type OTPModel struct {
	gorm.Model
	PhoneNumber string    `json:"phone_number" gorm:"uniqueIndex;not null"`
	OTPCode     string    `json:"otp_code" gorm:"not null"`
	ExpiresAt   time.Time `json:"expires_at" gorm:"not null"`
}

func (OTPModel) TableName() string {
	return "otps"
}

func (o *OTPModel) IsExpired() bool {
	return time.Now().After(o.ExpiresAt)
}

func CreateOTP(phoneNumber, otpCode string, ttl time.Duration) error {
	if err := DB.Unscoped().Where("phone_number = ?", phoneNumber).Delete(&OTPModel{}).Error; err != nil {
		return err
	}

	otp := OTPModel{
		PhoneNumber: phoneNumber,
		OTPCode:     otpCode,
		ExpiresAt:   time.Now().Add(ttl),
	}

	return DB.Create(&otp).Error
}

func VerifyOTP(phoneNumber, otpCode string) (bool, error) {
	var otp OTPModel
	result := DB.Where("phone_number = ?", phoneNumber).First(&otp)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, result.Error
	}

	if otp.IsExpired() {
		DB.Unscoped().Delete(&otp)
		return false, nil
	}

	isValid := otp.OTPCode == otpCode

	if isValid {
		DB.Unscoped().Delete(&otp)
	}

	return isValid, nil
}

func DeleteExpiredOTPs() error {
	return DB.Unscoped().Where("expires_at < ?", time.Now()).Delete(&OTPModel{}).Error
}
