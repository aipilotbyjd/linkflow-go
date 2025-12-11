package email

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"
)

// EmailService defines the interface for sending emails
type EmailService interface {
	SendVerificationEmail(to, name, token string) error
	SendPasswordResetEmail(to, name, token string) error
	SendLoginAlert(to, name, ipAddress, userAgent string) error
	SendWelcomeEmail(to, name string) error
}

// Config holds email service configuration
type Config struct {
	SMTPHost     string `mapstructure:"smtp_host"`
	SMTPPort     int    `mapstructure:"smtp_port"`
	SMTPUsername string `mapstructure:"smtp_username"`
	SMTPPassword string `mapstructure:"smtp_password"`
	FromEmail    string `mapstructure:"from_email"`
	FromName     string `mapstructure:"from_name"`
	UseTLS       bool   `mapstructure:"use_tls"`
	BaseURL      string `mapstructure:"base_url"` // Frontend URL for links
}

// SMTPEmailService implements EmailService using SMTP
type SMTPEmailService struct {
	config Config
}

// NewSMTPEmailService creates a new SMTP email service
func NewSMTPEmailService(cfg Config) *SMTPEmailService {
	return &SMTPEmailService{config: cfg}
}

// SendVerificationEmail sends email verification link
func (s *SMTPEmailService) SendVerificationEmail(to, name, token string) error {
	verifyLink := fmt.Sprintf("%s/verify-email?token=%s", s.config.BaseURL, token)

	subject := "Verify Your Email Address"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .button { display: inline-block; padding: 12px 24px; background-color: #4F46E5; color: white; text-decoration: none; border-radius: 6px; }
        .footer { margin-top: 20px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <h2>Welcome to LinkFlow, %s!</h2>
        <p>Thank you for signing up. Please verify your email address by clicking the button below:</p>
        <p><a href="%s" class="button">Verify Email</a></p>
        <p>Or copy and paste this link into your browser:</p>
        <p><code>%s</code></p>
        <p>This link will expire in 24 hours.</p>
        <div class="footer">
            <p>If you didn't create an account, you can safely ignore this email.</p>
        </div>
    </div>
</body>
</html>
`, name, verifyLink, verifyLink)

	return s.sendEmail(to, subject, body)
}

// SendPasswordResetEmail sends password reset link
func (s *SMTPEmailService) SendPasswordResetEmail(to, name, token string) error {
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", s.config.BaseURL, token)

	subject := "Reset Your Password"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .button { display: inline-block; padding: 12px 24px; background-color: #4F46E5; color: white; text-decoration: none; border-radius: 6px; }
        .warning { background-color: #FEF3C7; border-left: 4px solid #F59E0B; padding: 12px; margin: 16px 0; }
        .footer { margin-top: 20px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <h2>Password Reset Request</h2>
        <p>Hi %s,</p>
        <p>We received a request to reset your password. Click the button below to create a new password:</p>
        <p><a href="%s" class="button">Reset Password</a></p>
        <p>Or copy and paste this link into your browser:</p>
        <p><code>%s</code></p>
        <div class="warning">
            <strong>Security Notice:</strong> This link will expire in 1 hour.
        </div>
        <div class="footer">
            <p>If you didn't request a password reset, please ignore this email or contact support if you have concerns.</p>
        </div>
    </div>
</body>
</html>
`, name, resetLink, resetLink)

	return s.sendEmail(to, subject, body)
}

// SendLoginAlert sends notification of new login
func (s *SMTPEmailService) SendLoginAlert(to, name, ipAddress, userAgent string) error {
	subject := "New Login to Your Account"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .info-box { background-color: #F3F4F6; padding: 16px; border-radius: 6px; margin: 16px 0; }
        .warning { background-color: #FEE2E2; border-left: 4px solid #EF4444; padding: 12px; margin: 16px 0; }
        .footer { margin-top: 20px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <h2>New Login Detected</h2>
        <p>Hi %s,</p>
        <p>We detected a new login to your LinkFlow account:</p>
        <div class="info-box">
            <p><strong>IP Address:</strong> %s</p>
            <p><strong>Device:</strong> %s</p>
        </div>
        <div class="warning">
            <strong>Not you?</strong> If you didn't log in, please secure your account immediately by resetting your password.
        </div>
        <div class="footer">
            <p>This is an automated security notification from LinkFlow.</p>
        </div>
    </div>
</body>
</html>
`, name, ipAddress, userAgent)

	return s.sendEmail(to, subject, body)
}

// SendWelcomeEmail sends welcome email after registration
func (s *SMTPEmailService) SendWelcomeEmail(to, name string) error {
	subject := "Welcome to LinkFlow!"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .button { display: inline-block; padding: 12px 24px; background-color: #4F46E5; color: white; text-decoration: none; border-radius: 6px; }
        .features { background-color: #F3F4F6; padding: 16px; border-radius: 6px; margin: 16px 0; }
        .footer { margin-top: 20px; font-size: 12px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <h2>Welcome to LinkFlow, %s! 🎉</h2>
        <p>Your account is now active and ready to use.</p>
        <div class="features">
            <h3>Get Started:</h3>
            <ul>
                <li>Create your first workflow</li>
                <li>Connect your apps and services</li>
                <li>Automate your tasks</li>
            </ul>
        </div>
        <p><a href="%s/app" class="button">Go to Dashboard</a></p>
        <div class="footer">
            <p>Need help? Check out our documentation or contact support.</p>
        </div>
    </div>
</body>
</html>
`, name, s.config.BaseURL)

	return s.sendEmail(to, subject, body)
}

// sendEmail sends an email using SMTP
func (s *SMTPEmailService) sendEmail(to, subject, htmlBody string) error {
	from := fmt.Sprintf("%s <%s>", s.config.FromName, s.config.FromEmail)

	// Build email headers
	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	// Build message
	var message bytes.Buffer
	for k, v := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")
	message.WriteString(htmlBody)

	// Connect to SMTP server
	addr := fmt.Sprintf("%s:%d", s.config.SMTPHost, s.config.SMTPPort)

	var auth smtp.Auth
	if s.config.SMTPUsername != "" {
		auth = smtp.PlainAuth("", s.config.SMTPUsername, s.config.SMTPPassword, s.config.SMTPHost)
	}

	if s.config.UseTLS {
		// TLS connection
		tlsConfig := &tls.Config{
			ServerName: s.config.SMTPHost,
		}

		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, s.config.SMTPHost)
		if err != nil {
			return fmt.Errorf("failed to create SMTP client: %w", err)
		}
		defer client.Close()

		if auth != nil {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("SMTP authentication failed: %w", err)
			}
		}

		if err := client.Mail(s.config.FromEmail); err != nil {
			return fmt.Errorf("failed to set sender: %w", err)
		}

		if err := client.Rcpt(to); err != nil {
			return fmt.Errorf("failed to set recipient: %w", err)
		}

		w, err := client.Data()
		if err != nil {
			return fmt.Errorf("failed to get data writer: %w", err)
		}

		if _, err := w.Write(message.Bytes()); err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}

		if err := w.Close(); err != nil {
			return fmt.Errorf("failed to close data writer: %w", err)
		}

		return client.Quit()
	}

	// Non-TLS connection
	return smtp.SendMail(addr, auth, s.config.FromEmail, []string{to}, message.Bytes())
}

// NoopEmailService is a no-op implementation for testing/development
type NoopEmailService struct{}

func NewNoopEmailService() *NoopEmailService {
	return &NoopEmailService{}
}

func (n *NoopEmailService) SendVerificationEmail(to, name, token string) error {
	fmt.Printf("[EMAIL] Verification email to: %s, token: %s\n", to, token)
	return nil
}

func (n *NoopEmailService) SendPasswordResetEmail(to, name, token string) error {
	fmt.Printf("[EMAIL] Password reset email to: %s, token: %s\n", to, token)
	return nil
}

func (n *NoopEmailService) SendLoginAlert(to, name, ipAddress, userAgent string) error {
	fmt.Printf("[EMAIL] Login alert to: %s, IP: %s\n", to, ipAddress)
	return nil
}

func (n *NoopEmailService) SendWelcomeEmail(to, name string) error {
	fmt.Printf("[EMAIL] Welcome email to: %s\n", to)
	return nil
}

// Template helpers (for future use with external templates)
func renderTemplate(tmpl string, data interface{}) (string, error) {
	t, err := template.New("email").Parse(tmpl)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// ParseUserAgent extracts browser/device info from user agent string
func ParseUserAgent(userAgent string) string {
	// Simple parsing - in production, use a proper UA parser library
	if strings.Contains(userAgent, "Chrome") {
		return "Chrome Browser"
	} else if strings.Contains(userAgent, "Firefox") {
		return "Firefox Browser"
	} else if strings.Contains(userAgent, "Safari") {
		return "Safari Browser"
	} else if strings.Contains(userAgent, "Edge") {
		return "Microsoft Edge"
	}
	return userAgent
}
