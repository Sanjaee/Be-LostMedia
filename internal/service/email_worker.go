package service

import (
	"encoding/json"
	"log"
	"yourapp/internal/util"

	amqp "github.com/rabbitmq/amqp091-go"
)

type EmailWorker struct {
	emailService EmailService
	rabbitMQ     *util.RabbitMQClient
}

func NewEmailWorker(emailService EmailService, rabbitMQ *util.RabbitMQClient) *EmailWorker {
	return &EmailWorker{
		emailService: emailService,
		rabbitMQ:     rabbitMQ,
	}
}

// Start starts the email worker to consume messages from RabbitMQ
func (w *EmailWorker) Start() error {
	channel := w.rabbitMQ.GetChannel()
	queueName := w.rabbitMQ.GetQueueName()

	msgs, err := channel.Consume(
		queueName, // queue
		"",        // consumer
		false,     // auto-ack (set to false for manual ack)
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		return err
	}

	log.Println("Email worker started, waiting for messages...")

	// Process messages in a goroutine
	go func() {
		for msg := range msgs {
			if err := w.processEmailMessage(msg); err != nil {
				log.Printf("Error processing email message: %v", err)
				// Reject and requeue on error
				msg.Nack(false, true)
			} else {
				// Acknowledge message on success
				msg.Ack(false)
			}
		}
	}()

	return nil
}

func (w *EmailWorker) processEmailMessage(msg amqp.Delivery) error {
	var emailMsg util.EmailMessage
	if err := json.Unmarshal(msg.Body, &emailMsg); err != nil {
		return err
	}

	log.Printf("Processing email: Type=%s, To=%s", emailMsg.Type, emailMsg.To)

	switch emailMsg.Type {
	case "otp":
		// Extract OTP from body (simplified - in production, use structured data)
		return w.emailService.SendOTPEmail(emailMsg.To, emailMsg.Body)
	case "reset_password":
		// Body contains the reset password link
		return w.emailService.SendResetPasswordEmail(emailMsg.To, emailMsg.Body)
	case "verification":
		return w.emailService.SendVerificationEmail(emailMsg.To, emailMsg.Body)
	case "welcome":
		return w.emailService.SendWelcomeEmail(emailMsg.To, emailMsg.Subject) // Using Subject as name
	default:
		// Generic email
		return w.emailService.SendOTPEmail(emailMsg.To, emailMsg.Body)
	}
}

// Stop stops the email worker
func (w *EmailWorker) Stop() {
	log.Println("Stopping email worker...")
	// Channel and connection will be closed by RabbitMQ client
}
