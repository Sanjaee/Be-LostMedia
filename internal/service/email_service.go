package service

import (
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"yourapp/internal/config"
)

// EmailService mendefinisikan antarmuka untuk layanan pengiriman email.
type EmailService interface {
	SendOTPEmail(to, otpCode string) error
	SendResetPasswordEmail(to, resetLink string) error
	SendVerificationEmail(to, token string) error
	SendWelcomeEmail(to, name string) error
}

type emailService struct {
	config *config.Config
}

// NewEmailService membuat instance baru dari EmailService.
func NewEmailService(cfg *config.Config) EmailService {
	return &emailService{
		config: cfg,
	}
}

// sendEmail adalah helper untuk mengirim email tanpa HTML (text-only fallback).
func (s *emailService) sendEmail(to, subject, body string) error {
	return s.sendEmailHTML(to, subject, body, body)
}

// sendEmailHTML mengirim email multipart dengan versi HTML dan plain text.
func (s *emailService) sendEmailHTML(to, subject, htmlBody, textBody string) error {
	if s.config.SMTPUsername == "" || s.config.SMTPPassword == "" {
		// In development, just log the email
		fmt.Printf("[EMAIL] To: %s, Subject: %s\nBody: %s\n", to, subject, textBody)
		return nil
	}

	auth := smtp.PlainAuth("", s.config.SMTPUsername, s.config.SMTPPassword, s.config.SMTPHost)
	from := s.config.EmailFrom
	if from == "" {
		from = s.config.SMTPUsername
	}

	// Use custom email name if available
	fromHeader := from
	if s.config.EmailName != "" {
		fromHeader = fmt.Sprintf("%s <%s>", s.config.EmailName, from)
	}

	// Create multipart message with HTML and plain text
	boundary := "----=_NextPart_" + fmt.Sprintf("%d", time.Now().UnixNano())
	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n",
		fromHeader, to, subject, boundary)

	// Plain text part
	textPart := fmt.Sprintf("--%s\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s\r\n",
		boundary, textBody)

	// HTML part - use 8bit encoding (UTF-8 compatible)
	htmlPart := fmt.Sprintf("--%s\r\nContent-Type: text/html; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s\r\n",
		boundary, htmlBody)

	// End boundary
	endBoundary := fmt.Sprintf("--%s--\r\n", boundary)

	msg := []byte(headers + textPart + htmlPart + endBoundary)
	addr := fmt.Sprintf("%s:%s", s.config.SMTPHost, s.config.SMTPPort)

	err := smtp.SendMail(addr, auth, from, []string{to}, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (s *emailService) SendOTPEmail(to, otpCode string) error {
	subject := "Email Verification - Kode OTP Anda"
	emailName := s.config.EmailName
	if emailName == "" {
		emailName = "Zacode"
	}
	supportEmail := fmt.Sprintf("support@%s", s.config.EmailName)
	if s.config.EmailFrom != "" {
		// Extract domain from email if available
		if idx := strings.Index(s.config.EmailFrom, "@"); idx != -1 {
			supportEmail = fmt.Sprintf("support@%s", s.config.EmailFrom[idx+1:])
		}
	}

	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Email Verification</title>
</head>
<body style="margin: 0; padding: 0; font-family: Arial, sans-serif; line-height: 1.6; background-color: #f4f4f4;">
    <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="background-color: #f4f4f4; padding: 20px;">
        <tr>
            <td align="center">
                <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="600" style="max-width: 600px; width: 100%%; background-color: #ffffff; border-radius: 10px; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">
                    <!-- Header -->
                    <tr>
                        <td style="padding: 40px 40px 30px 40px; text-align: center;">
                            <h1 style="color: #333333; margin: 0; font-size: 24px; font-weight: bold;">Welcome to %s!</h1>
                        </td>
                    </tr>
                    
                    <!-- Content -->
                    <tr>
                        <td style="padding: 0 40px 30px 40px;">
                            <p style="margin: 0 0 15px 0; color: #333333; font-size: 16px;">Hello,</p>
                            <p style="margin: 0 0 15px 0; color: #333333; font-size: 16px;">Thank you for signing up with %s! We're thrilled to have you on board.</p>
                            <p style="margin: 0 0 25px 0; color: #333333; font-size: 16px;">To ensure the security of your account and access all the features, please use the following OTP to verify your email address:</p>
                            
                            <!-- OTP Box -->
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%">
                                <tr>
                                    <td align="center" style="padding: 25px 0;">
                                        <table role="presentation" cellpadding="0" cellspacing="0" border="0" style="background-color: #f8f9fa; border-radius: 5px; padding: 20px;">
                                            <tr>
                                                <td align="center" style="font-size: 24px; font-weight: bold; letter-spacing: 5px; color: #007bff; font-family: Arial, sans-serif;">
                                                    %s
                                                </td>
                                            </tr>
                                        </table>
                                    </td>
                                </tr>
                            </table>
                            
                            <p style="margin: 25px 0 15px 0; color: #333333; font-size: 16px;">Once your email is verified, you'll be ready to dive into %s's exciting features.</p>
                            <p style="margin: 0 0 15px 0; color: #333333; font-size: 16px;">If you did not register with us, please ignore this email or contact our support team at <a href="mailto:%s" style="color: #007bff; text-decoration: none;">%s</a>.</p>
                        </td>
                    </tr>
                    
                    <!-- Footer -->
                    <tr>
                        <td style="padding: 30px 40px; border-top: 1px solid #eeeeee; text-align: center;">
                            <p style="margin: 0 0 10px 0; color: #333333; font-size: 16px;">Thank you for choosing %s!</p>
                            <p style="margin: 0; color: #666666; font-size: 16px;">Best regards,<br>%s Team</p>
                        </td>
                    </tr>
                </table>
                
                <!-- Disclaimer -->
                <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="600" style="max-width: 600px; width: 100%%;">
                    <tr>
                        <td align="center" style="padding: 20px 0 0 0; color: #999999; font-size: 12px;">
                            <p style="margin: 0;">This is an automated message, please do not reply to this email.</p>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>
`, emailName, emailName, otpCode, emailName, supportEmail, supportEmail, emailName, emailName)

	textBody := fmt.Sprintf(`
Hello,

Thank you for signing up with %s! We're thrilled to have you on board.

To ensure the security of your account and access all the features, please use the following OTP to verify your email address:

OTP Code: %s

Once your email is verified, you'll be ready to dive into %s's exciting features.

If you did not register with us, please ignore this email or contact our support team at %s.

Thank you for choosing %s!

Best regards,
%s Team

---
This is an automated message, please do not reply to this email.
`, emailName, otpCode, emailName, supportEmail, emailName, emailName)

	return s.sendEmailHTML(to, subject, htmlBody, textBody)
}

func (s *emailService) SendResetPasswordEmail(to, otpCode string) error {
	subject := "Permintaan Reset Password - Kode OTP"

	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="id">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin: 0; padding: 0; font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background-color: #f4f6f8;">
    <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="background-color: #f4f6f8; padding: 40px 20px;">
        <tr>
            <td align="center">
                <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="600" style="max-width: 600px; width: 100%%; background-color: #ffffff; border: 1px solid #e5e7eb; border-radius: 4px; box-shadow: 0 2px 4px rgba(0, 0, 0, 0.05);">
                    <!-- Header -->
                    <tr>
                        <td style="background-color: #1e3a8a; padding: 30px 40px; border-bottom: 3px solid #1e40af;">
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%">
                                <tr>
                                    <td>
                                        <h1 style="margin: 0; color: #ffffff; font-size: 24px; font-weight: 600; letter-spacing: 0.5px;">%s</h1>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                    
                    <!-- Content -->
                    <tr>
                        <td style="padding: 40px;">
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%">
                                <tr>
                                    <td>
                                        <p style="margin: 0 0 20px; color: #1f2937; font-size: 16px; line-height: 1.6; font-weight: 500;">
                                            Yth. Pelanggan Terhormat,
                                        </p>
                                        <p style="margin: 0 0 24px; color: #374151; font-size: 15px; line-height: 1.7;">
                                            Kami menerima permintaan untuk mereset kata sandi akun <strong>%s</strong> Anda. Untuk melanjutkan proses reset password, gunakan kode OTP berikut:
                                        </p>
                                        
                                        <!-- OTP Code Box -->
                                        <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="margin: 0 0 32px;">
                                            <tr>
                                                <td style="background-color: #f8fafc; border: 2px solid #e5e7eb; border-radius: 6px; padding: 30px; text-align: center;">
                                                    <p style="margin: 0 0 12px; color: #6b7280; font-size: 12px; text-transform: uppercase; letter-spacing: 1px; font-weight: 600;">
                                                        Kode OTP Reset Password
                                                    </p>
                                                    <div style="font-size: 36px; font-weight: 700; color: #1e3a8a; letter-spacing: 6px; font-family: 'Courier New', 'Consolas', monospace; padding: 16px 0; background-color: #ffffff; border: 1px solid #d1d5db; border-radius: 4px;">
                                                        %s
                                                    </div>
                                                    <p style="margin: 16px 0 0; color: #6b7280; font-size: 13px;">
                                                        Berlaku selama <strong style="color: #dc2626;">10 menit</strong>
                                                    </p>
                                                </td>
                                            </tr>
                                        </table>
                                        
                                        <!-- Steps Box -->
                                        <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="margin: 0 0 24px;">
                                            <tr>
                                                <td style="background-color: #eff6ff; border: 1px solid #bfdbfe; border-radius: 6px; padding: 20px;">
                                                    <p style="margin: 0 0 16px; color: #1e40af; font-size: 15px; font-weight: 600;">
                                                        Langkah-langkah Reset Password:
                                                    </p>
                                                    <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%">
                                                        <tr>
                                                            <td style="padding: 8px 0; color: #1e3a8a; font-size: 14px; line-height: 1.8;">
                                                                <strong>1.</strong> Masukkan kode OTP di atas pada halaman reset password
                                                            </td>
                                                        </tr>
                                                        <tr>
                                                            <td style="padding: 8px 0; color: #1e3a8a; font-size: 14px; line-height: 1.8;">
                                                                <strong>2.</strong> Buat kata sandi baru yang kuat (minimal 8 karakter, kombinasi huruf, angka, dan simbol)
                                                            </td>
                                                        </tr>
                                                        <tr>
                                                            <td style="padding: 8px 0; color: #1e3a8a; font-size: 14px; line-height: 1.8;">
                                                                <strong>3.</strong> Login kembali menggunakan kata sandi baru Anda
                                                            </td>
                                                        </tr>
                                                    </table>
                                                </td>
                                            </tr>
                                        </table>
                                        
                                        <!-- Security Notice -->
                                        <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="margin: 0 0 24px;">
                                            <tr>
                                                <td style="background-color: #fef3c7; border-left: 4px solid #f59e0b; padding: 16px 20px; border-radius: 4px;">
                                                    <p style="margin: 0; color: #92400e; font-size: 14px; line-height: 1.6;">
                                                        <strong style="color: #78350f;">PERINGATAN KEAMANAN:</strong><br>
                                                        • Jika Anda TIDAK melakukan permintaan reset password ini, segera hubungi Customer Service kami<br>
                                                        • Jangan pernah membagikan kode OTP kepada siapapun<br>
                                                        • Pastikan Anda menggunakan koneksi internet yang aman saat melakukan reset password
                                                    </p>
                                                </td>
                                            </tr>
                                        </table>
                                        
                                        <p style="margin: 0; color: #374151; font-size: 15px; line-height: 1.7;">
                                            Email ini dikirim secara otomatis oleh sistem keamanan kami. Mohon untuk tidak membalas email ini.
                                        </p>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                    
                    <!-- Footer -->
                    <tr>
                        <td style="background-color: #f9fafb; border-top: 1px solid #e5e7eb; padding: 30px 40px;">
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%">
                                <tr>
                                    <td style="padding-bottom: 16px; border-bottom: 1px solid #e5e7eb;">
                                        <p style="margin: 0 0 12px; color: #1f2937; font-size: 14px; line-height: 1.6;">
                                            Hormat kami,<br>
                                            <strong style="color: #1e3a8a;">Tim Layanan Pelanggan<br>%s</strong>
                                        </p>
                                    </td>
                                </tr>
                                <tr>
                                    <td style="padding-top: 20px;">
                                        <p style="margin: 0 0 8px; color: #6b7280; font-size: 12px; line-height: 1.6;">
                                            <strong>Informasi Kontak:</strong><br>
                                            Email: support@%s<br>
                                            Jam Layanan: Senin - Jumat, 08:00 - 17:00 WIB
                                        </p>
                                        <p style="margin: 16px 0 0; color: #9ca3af; font-size: 11px; line-height: 1.6; border-top: 1px solid #e5e7eb; padding-top: 16px;">
                                            © %d %s. Hak Cipta Dilindungi.<br>
                                            Email ini bersifat rahasia dan ditujukan hanya untuk penerima yang dimaksud. Jika Anda menerima email ini secara tidak sengaja, mohon untuk menghapusnya dan tidak menyebarkannya.
                                        </p>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>
`, s.config.EmailName, s.config.EmailName, otpCode, s.config.EmailName, s.config.EmailName, time.Now().Year(), s.config.EmailName)

	textBody := fmt.Sprintf(`
Halo,

Kami menerima permintaan untuk mereset password akun %s Anda.

Kode OTP Reset Password: %s

Langkah selanjutnya:
1. Masukkan kode OTP di atas
2. Buat password baru yang kuat
3. Login dengan password baru Anda

Kode ini berlaku selama 10 menit. Jangan bagikan kode ini kepada siapapun.

Jika Anda tidak meminta reset password ini, silakan abaikan email ini.

Terima kasih,
Tim %s
`, s.config.EmailName, otpCode, s.config.EmailName)

	return s.sendEmailHTML(to, subject, htmlBody, textBody)
}

func (s *emailService) SendVerificationEmail(to, token string) error {
	subject := "Verifikasi Alamat Email Anda"
	verificationURL := fmt.Sprintf("%s/auth/verify-email?token=%s", s.config.ClientURL, token)

	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="id">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin: 0; padding: 0; font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background-color: #f4f6f8;">
    <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="background-color: #f4f6f8; padding: 40px 20px;">
        <tr>
            <td align="center">
                <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="600" style="max-width: 600px; width: 100%%; background-color: #ffffff; border: 1px solid #e5e7eb; border-radius: 4px; box-shadow: 0 2px 4px rgba(0, 0, 0, 0.05);">
                    <!-- Header -->
                    <tr>
                        <td style="background-color: #1e3a8a; padding: 30px 40px; border-bottom: 3px solid #1e40af;">
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%">
                                <tr>
                                    <td>
                                        <h1 style="margin: 0; color: #ffffff; font-size: 24px; font-weight: 600; letter-spacing: 0.5px;">%s</h1>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                    
                    <!-- Content -->
                    <tr>
                        <td style="padding: 40px;">
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%">
                                <tr>
                                    <td>
                                        <p style="margin: 0 0 20px; color: #1f2937; font-size: 16px; line-height: 1.6; font-weight: 500;">
                                            Yth. Pelanggan Terhormat,
                                        </p>
                                        <p style="margin: 0 0 24px; color: #374151; font-size: 15px; line-height: 1.7;">
                                            Terima kasih telah melakukan pendaftaran di <strong>%s</strong>. Untuk mengaktifkan akun Anda, silakan verifikasi alamat email dengan mengklik tombol di bawah ini:
                                        </p>
                                        
                                        <!-- CTA Button -->
                                        <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="margin: 0 0 32px;">
                                            <tr>
                                                <td align="center">
                                                    <a href="%s" style="display: inline-block; padding: 14px 36px; background-color: #1e3a8a; color: #ffffff; text-decoration: none; border-radius: 4px; font-weight: 600; font-size: 15px; letter-spacing: 0.3px; border: 2px solid #1e3a8a;">
                                                        Verifikasi Email Saya
                                                    </a>
                                                </td>
                                            </tr>
                                        </table>
                                        
                                        <!-- Alternative Link -->
                                        <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="margin: 0 0 24px;">
                                            <tr>
                                                <td style="background-color: #f8fafc; border: 1px solid #e5e7eb; border-radius: 6px; padding: 20px;">
                                                    <p style="margin: 0 0 12px; color: #6b7280; font-size: 13px; font-weight: 600;">
                                                        Atau salin dan tempel link berikut ke browser Anda:
                                                    </p>
                                                    <p style="margin: 0; color: #1e40af; font-size: 13px; word-break: break-all; line-height: 1.6; font-family: 'Courier New', monospace;">
                                                        %s
                                                    </p>
                                                </td>
                                            </tr>
                                        </table>
                                        
                                        <!-- Warning Box -->
                                        <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="margin: 0 0 24px;">
                                            <tr>
                                                <td style="background-color: #fef3c7; border-left: 4px solid #f59e0b; padding: 16px 20px; border-radius: 4px;">
                                                    <p style="margin: 0; color: #92400e; font-size: 14px; line-height: 1.6;">
                                                        <strong style="color: #78350f;">PENTING:</strong> Link verifikasi ini berlaku selama <strong>24 jam</strong>. Setelah waktu tersebut, Anda perlu meminta link verifikasi baru melalui halaman login.
                                                    </p>
                                                </td>
                                            </tr>
                                        </table>
                                        
                                        <p style="margin: 0; color: #374151; font-size: 15px; line-height: 1.7;">
                                            Jika Anda tidak melakukan pendaftaran untuk akun ini, silakan abaikan email ini. Akun tidak akan diaktifkan tanpa verifikasi email.
                                        </p>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                    
                    <!-- Footer -->
                    <tr>
                        <td style="background-color: #f9fafb; border-top: 1px solid #e5e7eb; padding: 30px 40px;">
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%">
                                <tr>
                                    <td style="padding-bottom: 16px; border-bottom: 1px solid #e5e7eb;">
                                        <p style="margin: 0 0 12px; color: #1f2937; font-size: 14px; line-height: 1.6;">
                                            Hormat kami,<br>
                                            <strong style="color: #1e3a8a;">Tim Layanan Pelanggan<br>%s</strong>
                                        </p>
                                    </td>
                                </tr>
                                <tr>
                                    <td style="padding-top: 20px;">
                                        <p style="margin: 0 0 8px; color: #6b7280; font-size: 12px; line-height: 1.6;">
                                            <strong>Informasi Kontak:</strong><br>
                                            Email: support@%s<br>
                                            Jam Layanan: Senin - Jumat, 08:00 - 17:00 WIB
                                        </p>
                                        <p style="margin: 16px 0 0; color: #9ca3af; font-size: 11px; line-height: 1.6; border-top: 1px solid #e5e7eb; padding-top: 16px;">
                                            © %d %s. Hak Cipta Dilindungi.<br>
                                            Email ini bersifat rahasia dan ditujukan hanya untuk penerima yang dimaksud. Jika Anda menerima email ini secara tidak sengaja, mohon untuk menghapusnya dan tidak menyebarkannya.
                                        </p>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>
`, s.config.EmailName, s.config.EmailName, verificationURL, verificationURL, s.config.EmailName, s.config.EmailName, time.Now().Year(), s.config.EmailName)

	textBody := fmt.Sprintf(`
Halo,

Terima kasih telah mendaftar di %s!

Klik link berikut untuk memverifikasi email Anda:
%s

Link ini akan kedaluwarsa dalam 24 jam.

Jika Anda tidak meminta verifikasi ini, abaikan email ini.

Terima kasih,
Tim %s
`, s.config.EmailName, verificationURL, s.config.EmailName)

	return s.sendEmailHTML(to, subject, htmlBody, textBody)
}

func (s *emailService) SendWelcomeEmail(to, name string) error {
	subject := "Selamat Datang di " + s.config.EmailName

	htmlBody := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="id">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="margin: 0; padding: 0; font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background-color: #f4f6f8;">
    <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="background-color: #f4f6f8; padding: 40px 20px;">
        <tr>
            <td align="center">
                <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="600" style="max-width: 600px; width: 100%%; background-color: #ffffff; border: 1px solid #e5e7eb; border-radius: 4px; box-shadow: 0 2px 4px rgba(0, 0, 0, 0.05);">
                    <!-- Header -->
                    <tr>
                        <td style="background-color: #1e3a8a; padding: 30px 40px; border-bottom: 3px solid #1e40af;">
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%">
                                <tr>
                                    <td>
                                        <h1 style="margin: 0; color: #ffffff; font-size: 24px; font-weight: 600; letter-spacing: 0.5px;">%s</h1>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                    
                    <!-- Content -->
                    <tr>
                        <td style="padding: 40px;">
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%">
                                <tr>
                                    <td>
                                        <p style="margin: 0 0 20px; color: #1f2937; font-size: 16px; line-height: 1.6; font-weight: 500;">
                                            Yth. %s,
                                        </p>
                                        <p style="margin: 0 0 24px; color: #374151; font-size: 15px; line-height: 1.7;">
                                            Selamat datang di <strong>%s</strong>! Kami sangat senang Anda telah bergabung dengan kami. Akun Anda telah berhasil dibuat dan siap digunakan.
                                        </p>
                                        
                                        <!-- Features Box -->
                                        <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="margin: 0 0 32px;">
                                            <tr>
                                                <td style="background-color: #f8fafc; border: 1px solid #e5e7eb; border-radius: 6px; padding: 24px;">
                                                    <p style="margin: 0 0 16px; color: #1e3a8a; font-size: 16px; font-weight: 600;">
                                                        Layanan yang Tersedia untuk Anda:
                                                    </p>
                                                    <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%">
                                                        <tr>
                                                            <td style="padding: 10px 0; border-bottom: 1px solid #e5e7eb;">
                                                                <table role="presentation" cellpadding="0" cellspacing="0" border="0">
                                                                    <tr>
                                                                        <td style="padding-right: 12px; vertical-align: top; width: 24px;">
                                                                            <span style="color: #1e3a8a; font-size: 18px;">✓</span>
                                                                        </td>
                                                                        <td style="color: #374151; font-size: 14px; line-height: 1.7;">
                                                                            <strong>Akses penuh</strong> ke semua fitur dan layanan platform
                                                                        </td>
                                                                    </tr>
                                                                </table>
                                                            </td>
                                                        </tr>
                                                        <tr>
                                                            <td style="padding: 10px 0; border-bottom: 1px solid #e5e7eb;">
                                                                <table role="presentation" cellpadding="0" cellspacing="0" border="0">
                                                                    <tr>
                                                                        <td style="padding-right: 12px; vertical-align: top; width: 24px;">
                                                                            <span style="color: #1e3a8a; font-size: 18px;">✓</span>
                                                                        </td>
                                                                        <td style="color: #374151; font-size: 14px; line-height: 1.7;">
                                                                            <strong>Keamanan data</strong> terjamin dengan enkripsi tingkat tinggi
                                                                        </td>
                                                                    </tr>
                                                                </table>
                                                            </td>
                                                        </tr>
                                                        <tr>
                                                            <td style="padding: 10px 0;">
                                                                <table role="presentation" cellpadding="0" cellspacing="0" border="0">
                                                                    <tr>
                                                                        <td style="padding-right: 12px; vertical-align: top; width: 24px;">
                                                                            <span style="color: #1e3a8a; font-size: 18px;">✓</span>
                                                                        </td>
                                                                        <td style="color: #374151; font-size: 14px; line-height: 1.7;">
                                                                            <strong>Dukungan pelanggan</strong> siap membantu Anda kapan saja
                                                                        </td>
                                                                    </tr>
                                                                </table>
                                                            </td>
                                                        </tr>
                                                    </table>
                                                </td>
                                            </tr>
                                        </table>
                                        
                                        <!-- Info Box -->
                                        <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%" style="margin: 0 0 24px;">
                                            <tr>
                                                <td style="background-color: #eff6ff; border-left: 4px solid #1e40af; padding: 16px 20px; border-radius: 4px;">
                                                    <p style="margin: 0; color: #1e40af; font-size: 14px; line-height: 1.6;">
                                                        <strong>Tips Keamanan:</strong> Pastikan Anda menggunakan kata sandi yang kuat dan tidak membagikan informasi akun kepada siapapun. Aktifkan autentikasi dua faktor jika tersedia untuk keamanan ekstra.
                                                    </p>
                                                </td>
                                            </tr>
                                        </table>
                                        
                                        <p style="margin: 0; color: #374151; font-size: 15px; line-height: 1.7;">
                                            Jika Anda memiliki pertanyaan atau memerlukan bantuan, jangan ragu untuk menghubungi tim layanan pelanggan kami. Kami selalu siap membantu Anda.
                                        </p>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                    
                    <!-- Footer -->
                    <tr>
                        <td style="background-color: #f9fafb; border-top: 1px solid #e5e7eb; padding: 30px 40px;">
                            <table role="presentation" cellpadding="0" cellspacing="0" border="0" width="100%%">
                                <tr>
                                    <td style="padding-bottom: 16px; border-bottom: 1px solid #e5e7eb;">
                                        <p style="margin: 0 0 12px; color: #1f2937; font-size: 14px; line-height: 1.6;">
                                            Hormat kami,<br>
                                            <strong style="color: #1e3a8a;">Tim Layanan Pelanggan<br>%s</strong>
                                        </p>
                                    </td>
                                </tr>
                                <tr>
                                    <td style="padding-top: 20px;">
                                        <p style="margin: 0 0 8px; color: #6b7280; font-size: 12px; line-height: 1.6;">
                                            <strong>Informasi Kontak:</strong><br>
                                            Email: support@%s<br>
                                            Jam Layanan: Senin - Jumat, 08:00 - 17:00 WIB
                                        </p>
                                        <p style="margin: 16px 0 0; color: #9ca3af; font-size: 11px; line-height: 1.6; border-top: 1px solid #e5e7eb; padding-top: 16px;">
                                            © %d %s. Hak Cipta Dilindungi.<br>
                                            Email ini bersifat rahasia dan ditujukan hanya untuk penerima yang dimaksud. Jika Anda menerima email ini secara tidak sengaja, mohon untuk menghapusnya dan tidak menyebarkannya.
                                        </p>
                                    </td>
                                </tr>
                            </table>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
</body>
</html>
`, s.config.EmailName, name, s.config.EmailName, s.config.EmailName, s.config.EmailName, time.Now().Year(), s.config.EmailName)

	textBody := fmt.Sprintf(`
Halo %s,

Selamat datang di %s!

Kami sangat senang Anda bergabung dengan komunitas kami.

Anda sekarang siap untuk mulai menjelajahi semua fitur yang kami tawarkan:
- Nikmati semua fitur yang tersedia
- Jelajahi pengalaman yang menyenangkan
- Hubungi tim support jika ada pertanyaan

Jika Anda memiliki pertanyaan atau memerlukan bantuan, jangan ragu untuk menghubungi tim dukungan kami.

Terima kasih,
Tim %s
`, name, s.config.EmailName, s.config.EmailName)

	return s.sendEmailHTML(to, subject, htmlBody, textBody)
}
