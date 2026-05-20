-- Enable UUID extension if not already enabled
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Offers table
CREATE TABLE offers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id UUID NOT NULL,
    master_id UUID NOT NULL,
    price DECIMAL(12,2) NOT NULL CHECK (price > 0),
    message TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'rejected', 'withdrawn')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Negotiation events table (immutable audit trail)
CREATE TABLE negotiation_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    offer_id UUID NOT NULL REFERENCES offers(id) ON DELETE CASCADE,
    order_id UUID NOT NULL,
    type VARCHAR(30) NOT NULL,
    actor_id UUID NOT NULL,
    actor_role VARCHAR(20) NOT NULL CHECK (actor_role IN ('master', 'customer')),
    price DECIMAL(12,2),
    message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for common queries
CREATE INDEX idx_offers_order_id ON offers(order_id);
CREATE INDEX idx_offers_master_id ON offers(master_id);
CREATE INDEX idx_offers_status ON offers(status);
CREATE INDEX idx_offers_master_order_status ON offers(master_id, order_id, status);

CREATE INDEX idx_negotiation_events_offer_id ON negotiation_events(offer_id);
CREATE INDEX idx_negotiation_events_order_id ON negotiation_events(order_id);
CREATE INDEX idx_negotiation_events_type ON negotiation_events(type);
