DROP INDEX IF EXISTS idx_negotiation_events_type;
DROP INDEX IF EXISTS idx_negotiation_events_order_id;
DROP INDEX IF EXISTS idx_negotiation_events_offer_id;

DROP INDEX IF EXISTS idx_offers_master_order_status;
DROP INDEX IF EXISTS idx_offers_status;
DROP INDEX IF EXISTS idx_offers_master_id;
DROP INDEX IF EXISTS idx_offers_order_id;

DROP TABLE IF EXISTS negotiation_events;
DROP TABLE IF EXISTS offers;
