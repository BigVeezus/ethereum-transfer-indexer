package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Transfer represents a normalized ERC-20 Transfer event
// Value is stored as Decimal128 for precise arithmetic operations and faster aggregations
// event_signature allows future expansion to other event types (Approval, etc.)
type Transfer struct {
	ID             primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	EventSignature string               `bson:"event_signature" json:"event_signature"` // "Transfer" for ERC-20 transfers
	Token          string               `bson:"token" json:"token"`
	From           string               `bson:"from" json:"from"`
	To             string               `bson:"to" json:"to"`
	Value          primitive.Decimal128 `bson:"value" json:"value"`                                   // Decimal128 for precision and performance
	ValueString    string               `bson:"value_string,omitempty" json:"value_string,omitempty"` // Legacy: kept for backward compatibility
	ValueDecimal   float64              `bson:"value_decimal" json:"value_decimal"`                   // Human-readable decimal representation
	BlockNumber    uint64               `bson:"block_number" json:"block_number"`
	TxHash         string               `bson:"tx_hash" json:"tx_hash"`
	TxIndex        uint                 `bson:"tx_index" json:"tx_index"`
	LogIndex       uint                 `bson:"log_index" json:"log_index"`
	Timestamp      time.Time            `bson:"timestamp" json:"timestamp"`
	CreatedAt      time.Time            `bson:"created_at" json:"created_at"`
}

// ProcessedBlock tracks the last processed block for idempotency
type ProcessedBlock struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	BlockNumber uint64             `bson:"block_number"`
	ProcessedAt time.Time          `bson:"processed_at"`
}

// TransferQueryParams represents query parameters for filtering transfers
type TransferQueryParams struct {
	Token      string
	From       string
	To         string
	StartBlock *uint64
	EndBlock   *uint64
	StartTime  *time.Time
	EndTime    *time.Time
	Limit      int
	Offset     int
}

// AggregateResponse represents aggregated statistics
type AggregateResponse struct {
	TotalTransfers    int64   `json:"total_transfers"`
	TotalValue        string  `json:"total_value"`
	TotalValueDecimal float64 `json:"total_value_decimal"`
	UniqueTokens      int64   `json:"unique_tokens"`
	UniqueAddresses   int64   `json:"unique_addresses"`
	TimeRange         struct {
		Start time.Time `json:"start"`
		End   time.Time `json:"end"`
	} `json:"time_range"`
}
