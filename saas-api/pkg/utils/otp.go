package utils

import (
	"crypto/rand"
	"fmt"
	"time"
)

const (
	OTPExpiryMinutes = 10 // OTP expires in 10 minutes
)

// GenerateOTP generates a 6-digit OTP
func GenerateOTP() (string, error) {
	otp := make([]byte, 3)
	if _, err := rand.Read(otp); err != nil {
		return "", err
	}
	// Generate 6-digit OTP (000000-999999)
	otpNum := int(otp[0])<<16 | int(otp[1])<<8 | int(otp[2])
	otpNum = otpNum % 1000000
	if otpNum < 0 {
		otpNum = -otpNum
	}
	return fmt.Sprintf("%06d", otpNum), nil
}

// GetOTPExpiryTime returns the expiry time for OTP
func GetOTPExpiryTime() time.Time {
	return time.Now().Add(OTPExpiryMinutes * time.Minute)
}

