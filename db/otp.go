package db

import (
	"time"

	"gorm.io/gorm"
)

// OTPModel represents an OTP entry in the database
type OTPModel struct {
	gorm.Model
	PhoneNumber string    `json:"phone_number" gorm:"uniqueIndex;not null"`
	OTPCode     string    `json:"otp_code" gorm:"not null"`
	ExpiresAt   time.Time `json:"expires_at" gorm:"not null"`
}

// TableName specifies the table name for OTPModel
func (OTPModel) TableName() string {
	return "otps"
}

// IsExpired checks if the OTP has expired
func (o *OTPModel) IsExpired() bool {
	return time.Now().After(o.ExpiresAt)
}

// CreateOTP creates a new OTP entry in the database
func CreateOTP(phoneNumber, otpCode string, ttl time.Duration) error {
	// Delete any existing OTP for this phone number
	if err := DB.Where("phone_number = ?", phoneNumber).Delete(&OTPModel{}).Error; err != nil {
		return err
	}

	// Create new OTP
	otp := OTPModel{
		PhoneNumber: phoneNumber,
		OTPCode:     otpCode,
		ExpiresAt:   time.Now().Add(ttl),
	}

	return DB.Create(&otp).Error
}

// VerifyOTP checks if the provided OTP is valid
func VerifyOTP(phoneNumber, otpCode string) (bool, error) {
	var otp OTPModel
	result := DB.Where("phone_number = ?", phoneNumber).First(&otp)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, result.Error
	}

	// Check if OTP has expired
	if otp.IsExpired() {
		// Delete expired OTP
		DB.Delete(&otp)
		return false, nil
	}

	// Check if OTP matches
	isValid := otp.OTPCode == otpCode

	// If valid, delete the OTP
	if isValid {
		DB.Delete(&otp)
	}

	return isValid, nil
}

// DeleteExpiredOTPs removes all expired OTPs from the database
func DeleteExpiredOTPs() error {
	return DB.Where("expires_at < ?", time.Now()).Delete(&OTPModel{}).Error
}
