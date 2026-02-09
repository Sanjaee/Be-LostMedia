package service

import (
	"encoding/json"
	"log"

	"yourapp/internal/util"
	"yourapp/internal/websocket"

	amqp "github.com/rabbitmq/amqp091-go"
)

// NotificationWorker consumes notification messages from RabbitMQ and pushes to WebSocket
type NotificationWorker struct {
	notificationService NotificationService
	rabbitMQ           *util.RabbitMQClient
	wsHub              *websocket.Hub
	stopChan           chan bool
}

// NewNotificationWorker creates a new notification worker
func NewNotificationWorker(
	notificationService NotificationService,
	rabbitMQ *util.RabbitMQClient,
	wsHub *websocket.Hub,
) *NotificationWorker {
	return &NotificationWorker{
		notificationService: notificationService,
		rabbitMQ:            rabbitMQ,
		wsHub:               wsHub,
		stopChan:            make(chan bool),
	}
}

// Start starts consuming notification messages from RabbitMQ
func (w *NotificationWorker) Start() error {
	if w.rabbitMQ == nil {
		return nil // RabbitMQ not available, worker will not start
	}

	channel := w.rabbitMQ.GetChannel()
	if channel == nil {
		return nil
	}

	// Ensure notification exchange and queue exist
	// This is already done in notification service, but we ensure it here too
	notificationExchange := "notification_exchange"
	notificationQueue := "notification_queue"

	// Declare exchange
	if err := channel.ExchangeDeclare(
		notificationExchange,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}

	// Declare queue
	queue, err := channel.QueueDeclare(
		notificationQueue,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	// Bind queue to exchange
	if err := channel.QueueBind(
		notificationQueue,
		"notification",
		notificationExchange,
		false,
		nil,
	); err != nil {
		return err
	}

	// Consume messages
	msgs, err := channel.Consume(
		queue.Name,
		"notification_worker",
		false, // auto-ack
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	// Start consuming in a goroutine
	go func() {
		log.Println("Notification worker started, consuming messages...")
		for {
			select {
			case <-w.stopChan:
				log.Println("Notification worker stopped")
				return
			case msg, ok := <-msgs:
				if !ok {
					log.Println("Notification queue closed")
					return
				}
				if err := w.processNotificationMessage(msg); err != nil {
					log.Printf("Error processing notification message: %v", err)
					// Don't ack on error, let RabbitMQ requeue
					msg.Nack(false, true)
				} else {
					msg.Ack(false)
				}
			}
		}
	}()

	return nil
}

// processNotificationMessage processes a notification message from RabbitMQ
func (w *NotificationWorker) processNotificationMessage(msg amqp.Delivery) error {
	var notificationMsg NotificationMessage
	if err := json.Unmarshal(msg.Body, &notificationMsg); err != nil {
		return err
	}

	// Push to WebSocket hub for realtime delivery
	// BroadcastToUser already wraps as {type:"notification", payload:...}
	// so we pass the notification data directly — no extra wrapping
	if w.wsHub != nil {
		payload := map[string]interface{}{
			"type":    notificationMsg.Type,
			"user_id": notificationMsg.UserID,
		}
		// Copy all fields from notificationMsg into payload
		msgBytes, err := json.Marshal(notificationMsg)
		if err == nil {
			var msgMap map[string]interface{}
			if json.Unmarshal(msgBytes, &msgMap) == nil {
				for k, v := range msgMap {
					payload[k] = v
				}
			}
		}
		w.wsHub.BroadcastToUser(notificationMsg.UserID, payload)
		log.Printf("Notification pushed to WebSocket for user: %s, type: %s", notificationMsg.UserID, notificationMsg.Type)
	}

	return nil
}

// Stop stops the notification worker
func (w *NotificationWorker) Stop() {
	close(w.stopChan)
}
