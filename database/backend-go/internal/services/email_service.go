package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/smtp"

	"converter-backend/internal/config"
	"converter-backend/internal/logger"
)

type EmailService struct {
	config *config.EmailConfig
}

func NewEmailService(cfg *config.EmailConfig) *EmailService {
	return &EmailService{config: cfg}
}

// GenerateToken generates a random token for email verification or password reset
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SendVerificationEmail sends an email verification link
func (s *EmailService) SendVerificationEmail(to, token string) error {
	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", s.config.AppURL, token)

	subject := "Verify Your Email"
	body := fmt.Sprintf(`
Hello,

Thank you for registering! Please verify your email address by clicking the link below:

%s

This link will expire in 24 hours.

If you didn't create this account, please ignore this email.

Best regards,
Your App Team
`, verifyURL)

	return s.sendEmail(to, subject, body)
}

// SendPasswordResetEmail sends a password reset link
func (s *EmailService) SendPasswordResetEmail(to, token string) error {
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.config.AppURL, token)

	subject := "Password Reset Request"
	body := fmt.Sprintf(`
Hello,

We received a request to reset your password. Click the link below to reset it:

%s

This link will expire in 1 hour.

If you didn't request a password reset, please ignore this email.

Best regards,
Your App Team
`, resetURL)

	return s.sendEmail(to, subject, body)
}

// sendEmail sends an email using SMTP
func (s *EmailService) sendEmail(to, subject, body string) error {
	// If SMTP not configured, just log (for development)
	if s.config.SMTPUser == "" || s.config.SMTPPassword == "" {
		logger.Info(fmt.Sprintf("Email (dev mode): To=%s, Subject=%s\nBody:\n%s", to, subject, body))
		return nil
	}

	from := s.config.FromAddress
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", from, to, subject, body)

	auth := smtp.PlainAuth("", s.config.SMTPUser, s.config.SMTPPassword, s.config.SMTPHost)
	addr := fmt.Sprintf("%s:%d", s.config.SMTPHost, s.config.SMTPPort)

	err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to send email to %s: %v", to, err))
		return err
	}

	logger.Info(fmt.Sprintf("Email sent to %s", to))
	return nil
}
