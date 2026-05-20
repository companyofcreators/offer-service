package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/companyofcreators/offer-service/internal/domain/offer"
)

// OfferRepo implements offer.OfferRepository using PostgreSQL.
type OfferRepo struct {
	pool *sqlx.DB
}

// NewOfferRepo creates a new OfferRepo.
func NewOfferRepo(pool *sqlx.DB) *OfferRepo {
	return &OfferRepo{pool: pool}
}

func (r *OfferRepo) Create(ctx context.Context, o *offer.Offer) error {
	query := `
		INSERT INTO offers (id, order_id, master_id, price, message, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.pool.ExecContext(ctx, query,
		o.ID, o.OrderID, o.MasterID, o.Price, o.Message, string(o.Status), o.CreatedAt, o.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create offer: %w", err)
	}

	return nil
}

func (r *OfferRepo) FindByID(ctx context.Context, id uuid.UUID) (*offer.Offer, error) {
	query := `
		SELECT id, order_id, master_id, price, message, status, created_at, updated_at
		FROM offers
		WHERE id = $1
	`

	var o offer.Offer
	var status string
	err := r.pool.QueryRowContext(ctx, query, id).Scan(
		&o.ID, &o.OrderID, &o.MasterID, &o.Price, &o.Message, &status, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find offer by id: %w", err)
	}
	o.Status = offer.OfferStatus(status)
	return &o, nil
}

func (r *OfferRepo) ListByOrder(ctx context.Context, orderID uuid.UUID) ([]*offer.Offer, error) {
	query := `
		SELECT id, order_id, master_id, price, message, status, created_at, updated_at
		FROM offers
		WHERE order_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.pool.QueryContext(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to list offers by order: %w", err)
	}
	defer rows.Close()

	return scanOffers(rows)
}

func (r *OfferRepo) ListByMaster(ctx context.Context, masterID uuid.UUID, status *offer.OfferStatus, limit, offset int) ([]*offer.Offer, int, error) {
	args := []interface{}{masterID}
	argIdx := 2

	var countQuery, dataQuery string

	if status != nil {
		countQuery = `SELECT COUNT(*) FROM offers WHERE master_id = $1 AND status = $2`
		dataQuery = `
			SELECT id, order_id, master_id, price, message, status, created_at, updated_at
			FROM offers
			WHERE master_id = $1 AND status = $2
			ORDER BY created_at DESC
			LIMIT $3 OFFSET $4
		`
		args = append(args, string(*status))
		argIdx = 3
	} else {
		countQuery = `SELECT COUNT(*) FROM offers WHERE master_id = $1`
		dataQuery = `
			SELECT id, order_id, master_id, price, message, status, created_at, updated_at
			FROM offers
			WHERE master_id = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`
	}

	var total int
	if err := r.pool.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count offers by master: %w", err)
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	args = append(args, limit, offset)
	rows, err := r.pool.QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list offers by master: %w", err)
	}
	defer rows.Close()

	offers, err := scanOffers(rows)
	if err != nil {
		return nil, 0, err
	}
	_ = argIdx // suppress unused variable warning
	return offers, total, nil
}

func (r *OfferRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status offer.OfferStatus) error {
	query := `
		UPDATE offers
		SET status = $1, updated_at = $2
		WHERE id = $3
	`

	result, err := r.pool.ExecContext(ctx, query, string(status), time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("failed to update offer status: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return offer.ErrOfferNotFound
	}

	return nil
}

func (r *OfferRepo) RejectAllExcept(ctx context.Context, orderID uuid.UUID, exceptOfferID uuid.UUID) error {
	query := `
		UPDATE offers
		SET status = $1, updated_at = $2
		WHERE order_id = $3
		  AND status = $4
		  AND id != $5
	`

	_, err := r.pool.ExecContext(ctx, query, string(offer.OfferRejected), time.Now().UTC(), orderID, string(offer.OfferPending), exceptOfferID)
	if err != nil {
		return fmt.Errorf("failed to reject all except offer: %w", err)
	}

	return nil
}

func (r *OfferRepo) CountPending(ctx context.Context, orderID uuid.UUID) (int, error) {
	query := `
		SELECT COUNT(*)
		FROM offers
		WHERE order_id = $1 AND status = $2
	`

	var count int
	err := r.pool.QueryRowContext(ctx, query, orderID, string(offer.OfferPending)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count pending offers: %w", err)
	}

	return count, nil
}

func (r *OfferRepo) FindPendingByMasterAndOrder(ctx context.Context, masterID, orderID uuid.UUID) (*offer.Offer, error) {
	query := `
		SELECT id, order_id, master_id, price, message, status, created_at, updated_at
		FROM offers
		WHERE master_id = $1 AND order_id = $2 AND status = $3
	`

	var o offer.Offer
	var status string
	err := r.pool.QueryRowContext(ctx, query, masterID, orderID, string(offer.OfferPending)).Scan(
		&o.ID, &o.OrderID, &o.MasterID, &o.Price, &o.Message, &status, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find pending offer by master and order: %w", err)
	}
	o.Status = offer.OfferStatus(status)
	return &o, nil
}

// NegotiationEventRepo implements offer.NegotiationEventRepository using PostgreSQL.
type NegotiationEventRepo struct {
	pool *sqlx.DB
}

// NewNegotiationEventRepo creates a new NegotiationEventRepo.
func NewNegotiationEventRepo(pool *sqlx.DB) *NegotiationEventRepo {
	return &NegotiationEventRepo{pool: pool}
}

func (r *NegotiationEventRepo) Create(ctx context.Context, e *offer.NegotiationEvent) error {
	query := `
		INSERT INTO negotiation_events (id, offer_id, order_id, type, actor_id, actor_role, price, message, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.pool.ExecContext(ctx, query,
		e.ID, e.OfferID, e.OrderID, string(e.Type), e.ActorID, e.ActorRole, e.Price, e.Message, e.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create negotiation event: %w", err)
	}

	return nil
}

func (r *NegotiationEventRepo) ListByOrder(ctx context.Context, orderID uuid.UUID) ([]*offer.NegotiationEvent, error) {
	query := `
		SELECT id, offer_id, order_id, type, actor_id, actor_role, price, message, created_at
		FROM negotiation_events
		WHERE order_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.pool.QueryContext(ctx, query, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to list events by order: %w", err)
	}
	defer rows.Close()

	return scanNegotiationEvents(rows)
}

func (r *NegotiationEventRepo) ListByOffer(ctx context.Context, offerID uuid.UUID) ([]*offer.NegotiationEvent, error) {
	query := `
		SELECT id, offer_id, order_id, type, actor_id, actor_role, price, message, created_at
		FROM negotiation_events
		WHERE offer_id = $1
		ORDER BY created_at ASC
	`

	rows, err := r.pool.QueryContext(ctx, query, offerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list events by offer: %w", err)
	}
	defer rows.Close()

	return scanNegotiationEvents(rows)
}

// scanOffers scans multiple rows into a slice of offers.
func scanOffers(rows *sql.Rows) ([]*offer.Offer, error) {
	var offers []*offer.Offer
	for rows.Next() {
		var o offer.Offer
		var status string
		if err := rows.Scan(&o.ID, &o.OrderID, &o.MasterID, &o.Price, &o.Message, &status, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan offer row: %w", err)
		}
		o.Status = offer.OfferStatus(status)
		offers = append(offers, &o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating offer rows: %w", err)
	}
	return offers, nil
}

// scanNegotiationEvents scans multiple rows into a slice of negotiation events.
func scanNegotiationEvents(rows *sql.Rows) ([]*offer.NegotiationEvent, error) {
	var events []*offer.NegotiationEvent
	for rows.Next() {
		var e offer.NegotiationEvent
		var eventType string
		if err := rows.Scan(&e.ID, &e.OfferID, &e.OrderID, &eventType, &e.ActorID, &e.ActorRole, &e.Price, &e.Message, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan negotiation event row: %w", err)
		}
		e.Type = offer.NegotiationEventType(eventType)
		events = append(events, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating negotiation event rows: %w", err)
	}
	return events, nil
}
