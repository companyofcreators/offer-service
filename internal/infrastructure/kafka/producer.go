package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"

	"github.com/companyofcreators/offer-service/internal/domain/offer"
)

// Producer publishes offer-related events to Kafka.
type Producer struct {
	writer *kafka.Writer
	log    *slog.Logger
}

// NewProducer creates a new Kafka producer.
func NewProducer(brokers []string, log *slog.Logger) *Producer {
	writer := &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Balancer:               &kafka.LeastBytes{},
		BatchTimeout:           10 * time.Millisecond,
		BatchSize:              1,
		RequiredAcks:           kafka.RequireOne,
		Compression:            kafka.Snappy,
		AllowAutoTopicCreation: true,
	}

	return &Producer{
		writer: writer,
		log:    log,
	}
}

// Close closes the Kafka writer.
func (p *Producer) Close() error {
	return p.writer.Close()
}

// PublishOfferCreated publishes an offer.created event.
func (p *Producer) PublishOfferCreated(ctx context.Context, o *offer.Offer, masterEmail string) error {
	msg := map[string]interface{}{
		"offer_id":     o.ID.String(),
		"order_id":     o.OrderID.String(),
		"master_id":    o.MasterID.String(),
		"master_email": masterEmail,
		"price":        o.Price,
		"timestamp":    o.CreatedAt.Format(time.RFC3339),
	}

	return p.publish(ctx, "offer.created", o.ID, msg)
}

// PublishOfferAccepted publishes an offer.accepted event.
func (p *Producer) PublishOfferAccepted(ctx context.Context, o *offer.Offer, customerID uuid.UUID, customerEmail string) error {
	msg := map[string]interface{}{
		"offer_id":      o.ID.String(),
		"order_id":      o.OrderID.String(),
		"master_id":     o.MasterID.String(),
		"customer_id":   customerID.String(),
		"customer_email": customerEmail,
		"price":         o.Price,
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
	}

	return p.publish(ctx, "offer.accepted", o.ID, msg)
}

// PublishOfferRejected publishes an offer.rejected event.
func (p *Producer) PublishOfferRejected(ctx context.Context, offerID, orderID uuid.UUID) error {
	msg := map[string]interface{}{
		"offer_id":  offerID.String(),
		"order_id":  orderID.String(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	return p.publish(ctx, "offer.rejected", offerID, msg)
}

// PublishOfferWithdrawn publishes an offer.withdrawn event.
func (p *Producer) PublishOfferWithdrawn(ctx context.Context, o *offer.Offer) error {
	msg := map[string]interface{}{
		"offer_id":  o.ID.String(),
		"order_id":  o.OrderID.String(),
		"master_id": o.MasterID.String(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	return p.publish(ctx, "offer.withdrawn", o.ID, msg)
}

// PublishOfferCountered publishes an offer.countered event.
func (p *Producer) PublishOfferCountered(ctx context.Context, offerID, orderID, customerID uuid.UUID, proposedPrice float64) error {
	msg := map[string]interface{}{
		"offer_id":       offerID.String(),
		"order_id":       orderID.String(),
		"customer_id":    customerID.String(),
		"proposed_price": proposedPrice,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	}

	return p.publish(ctx, "offer.countered", offerID, msg)
}

// PublishOfferUpdated publishes a generic offer.updated event.
func (p *Producer) PublishOfferUpdated(ctx context.Context, o *offer.Offer) error {
	msg := map[string]interface{}{
		"offer_id":  o.ID.String(),
		"order_id":  o.OrderID.String(),
		"status":    string(o.Status),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	return p.publish(ctx, "offer.updated", o.ID, msg)
}

func (p *Producer) publish(ctx context.Context, topic string, key uuid.UUID, msg map[string]interface{}) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal kafka message: %w", err)
	}

	err = p.writer.WriteMessages(ctx, kafka.Message{
		Topic: topic,
		Key:   []byte(key.String()),
		Value: payload,
	})
	if err != nil {
		return fmt.Errorf("failed to publish message to topic %s: %w", topic, err)
	}

	p.log.DebugContext(ctx, "published kafka message",
		"topic", topic,
		"key", key.String(),
	)

	return nil
}
