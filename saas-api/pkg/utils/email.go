package utils

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
)

type EmailConfig struct {
	SMTPHost     string
	SMTPPort     string
	SMTPUsername string
	SMTPPassword string
	FromEmail    string
	FromName     string
}

func GetEmailConfig() *EmailConfig {
	return &EmailConfig{
		SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:     getEnv("SMTP_PORT", "587"),
		SMTPUsername: getEnv("SMTP_USERNAME", "abhinavkangale123@gmail.com"),
		SMTPPassword: getEnv("SMTP_PASSWORD", "tmnpmoiwcxiaajjx"),
		FromEmail:    getEnv("SMTP_FROM_EMAIL", "abhinavkangale123@gmail.com"),
		FromName:     getEnv("SMTP_FROM_NAME", "FIA - FYERS Intelligent Assistant"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// SendOTPEmail sends OTP via email
func SendOTPEmail(email, otp string) error {
	config := GetEmailConfig()

	if config.SMTPPassword == "" {
		// In development, just log the OTP
		log.Printf("⚠️  SMTP_PASSWORD not set! OTP for %s: %s (email not sent)", email, otp)
		return fmt.Errorf("SMTP_PASSWORD not configured")
	}

	log.Printf("Email: Sending OTP email to %s via %s:%s", email, config.SMTPHost, config.SMTPPort)
	log.Printf("Email: Using SMTP username: %s", config.SMTPUsername)

	// Email template
	emailBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<title>Your OTP Code</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
	<div style="background: linear-gradient(135deg, #4158D0 0%%, #5B6FD8 100%%); padding: 30px; text-align: center; border-radius: 10px 10px 0 0;">
		<h1 style="color: white; margin: 0;">FIA - FYERS Intelligent Assistant</h1>
	</div>
	<div style="background: #f9f9f9; padding: 30px; border-radius: 0 0 10px 10px;">
		<h2 style="color: #333; margin-top: 0;">Your Login OTP</h2>
		<p>Hello,</p>
		<p>Your One-Time Password (OTP) for login is:</p>
		<div style="background: white; border: 2px solid #4158D0; border-radius: 8px; padding: 20px; text-align: center; margin: 20px 0;">
			<h1 style="color: #4158D0; font-size: 36px; letter-spacing: 8px; margin: 0; font-family: 'Courier New', monospace;">%s</h1>
		</div>
		<p style="color: #666; font-size: 14px;">This OTP will expire in 10 minutes.</p>
		<p style="color: #666; font-size: 14px;">If you didn't request this OTP, please ignore this email.</p>
		<hr style="border: none; border-top: 1px solid #ddd; margin: 30px 0;">
		<p style="color: #999; font-size: 12px; margin: 0;">This is an automated message from FIA - FYERS Intelligent Assistant.</p>
	</div>
</body>
</html>
`, otp)

	// Plain text version
	textBody := fmt.Sprintf(`
FIA - FYERS Intelligent Assistant

Your Login OTP

Your One-Time Password (OTP) for login is: %s

This OTP will expire in 10 minutes.

If you didn't request this OTP, please ignore this email.

This is an automated message from FIA - FYERS Intelligent Assistant.
`, otp)

	// Create message with proper headers
	msg := []byte(
		fmt.Sprintf("From: %s <%s>\r\n", config.FromName, config.FromEmail) +
			fmt.Sprintf("To: %s\r\n", email) +
			"Subject: Your Login OTP - FIA\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: multipart/alternative; boundary=boundary123\r\n\r\n" +
			"--boundary123\r\n" +
			"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
			textBody + "\r\n\r\n" +
			"--boundary123\r\n" +
			"Content-Type: text/html; charset=UTF-8\r\n\r\n" +
			emailBody + "\r\n\r\n" +
			"--boundary123--\r\n")

	// SMTP authentication (using empty identity string as in the example)
	auth := smtp.PlainAuth("", config.SMTPUsername, config.SMTPPassword, config.SMTPHost)

	// Send email using Gmail SMTP
	addr := fmt.Sprintf("%s:%s", config.SMTPHost, config.SMTPPort)
	log.Printf("Email: Connecting to SMTP server: %s", addr)
	err := smtp.SendMail(addr, auth, config.FromEmail, []string{email}, msg)
	if err != nil {
		log.Printf("Email: ❌ SMTP SendMail error: %v", err)
		log.Printf("Email: Error details - Host: %s, Port: %s, Username: %s, FromEmail: %s", config.SMTPHost, config.SMTPPort, config.SMTPUsername, config.FromEmail)
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("Email: ✅ Email sent successfully to %s", email)
	return nil
}
