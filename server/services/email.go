package services

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/smtp"
	"time"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
)

const (
	codeLength     = 6
	codeExpiry     = 10 * time.Minute
	codeCooldown   = 60 * time.Second // minimum interval between sends to same email
	maxCodesPerDay = 10               // max codes per email per day
)

// GenerateCode creates a random 6-digit numeric code.
func GenerateCode() (string, error) {
	code := ""
	for i := 0; i < codeLength; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		code += fmt.Sprintf("%d", n.Int64())
	}
	return code, nil
}

// SendEmailCode generates a verification code and sends it via SMTP.
// Returns error if rate limited or send fails.
func SendEmailCode(email string) error {
	// Rate limit: check cooldown (no more than 1 per minute)
	var recent models.EmailCode
	cooldownAfter := time.Now().Add(-codeCooldown)
	if err := database.DB.Where("email = ? AND created_at > ?", email, cooldownAfter).
		First(&recent).Error; err == nil {
		return fmt.Errorf("please wait before requesting another code")
	}

	// Rate limit: max codes per day
	var dailyCount int64
	dayStart := time.Now().Truncate(24 * time.Hour)
	database.DB.Model(&models.EmailCode{}).
		Where("email = ? AND created_at > ?", email, dayStart).
		Count(&dailyCount)
	if dailyCount >= maxCodesPerDay {
		return fmt.Errorf("too many codes requested today")
	}

	// Generate code
	code, err := GenerateCode()
	if err != nil {
		return fmt.Errorf("failed to generate code: %w", err)
	}

	// Save to database
	emailCode := models.EmailCode{
		Email:     email,
		Code:      code,
		ExpiresAt: time.Now().Add(codeExpiry),
	}
	if err := database.DB.Create(&emailCode).Error; err != nil {
		return fmt.Errorf("failed to save code: %w", err)
	}

	// Send email via SMTP
	if err := sendSMTP(email, code); err != nil {
		util.Log.Error("[email] Failed to send code to %s: %v", email, err)
		return fmt.Errorf("failed to send email")
	}

	util.Log.Info("[email] Verification code sent to %s", email)
	return nil
}

// VerifyEmailCode checks if the code is valid for the email.
// Marks the code as used on success.
func VerifyEmailCode(email, code string) bool {
	var emailCode models.EmailCode
	err := database.DB.Where(
		"email = ? AND code = ? AND used = false AND expires_at > ?",
		email, code, time.Now(),
	).Order("created_at DESC").First(&emailCode).Error

	if err != nil {
		return false
	}

	// Mark as used
	database.DB.Model(&emailCode).Update("used", true)

	// Invalidate all other unused codes for this email
	database.DB.Model(&models.EmailCode{}).
		Where("email = ? AND used = false AND id != ?", email, emailCode.ID).
		Update("used", true)

	return true
}

// sendSMTP sends the verification code email via Proton SMTP.
func sendSMTP(to, code string) error {
	cfg := config.Cfg
	if cfg.SMTPUser == "" || cfg.SMTPPassword == "" {
		// In development, just log to console
		util.Log.Info("[email] DEV MODE — code for %s: %s", to, code)
		return nil
	}

	subject := "Your Ensoul verification code"
	body := fmt.Sprintf(`Your verification code is:

    %s

This code expires in 10 minutes.

If you didn't request this, please ignore this email.

— Ensoul`, code)

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		cfg.SMTPFrom, to, subject, body)

	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)
	addr := fmt.Sprintf("%s:%s", cfg.SMTPHost, cfg.SMTPPort)

	return smtp.SendMail(addr, auth, cfg.SMTPUser, []string{to}, []byte(msg))
}
