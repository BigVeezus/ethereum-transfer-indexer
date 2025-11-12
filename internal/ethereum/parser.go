package ethereum

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"pagrin/internal/models"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ERC20TransferEventSignature is the keccak256 hash of Transfer(address,address,uint256)
var ERC20TransferEventSignature = common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")

// EventSignatureTransfer is the string identifier for ERC-20 Transfer events
const EventSignatureTransfer = "Transfer"

// ParseTransferLog parses a raw Ethereum log into a normalized Transfer event
// Converts wei value to Decimal128 for precise storage and efficient aggregations
func ParseTransferLog(log types.Log, blockTime time.Time) (*models.Transfer, error) {
	if len(log.Topics) != 3 {
		return nil, fmt.Errorf("invalid Transfer event: expected 3 topics, got %d", len(log.Topics))
	}

	if log.Topics[0] != ERC20TransferEventSignature {
		return nil, fmt.Errorf("not a Transfer event")
	}

	from := common.BytesToAddress(log.Topics[1].Bytes())
	to := common.BytesToAddress(log.Topics[2].Bytes())

	if len(log.Data) != 32 {
		return nil, fmt.Errorf("invalid Transfer event data: expected 32 bytes, got %d", len(log.Data))
	}

	value := new(big.Int).SetBytes(log.Data)
	valueStr := value.String()

	// Convert big.Int to Decimal128 for MongoDB storage
	// Decimal128 provides 34 decimal digits of precision, perfect for wei values
	decimalValue, err := primitive.ParseDecimal128(valueStr)
	if err != nil {
		// Fallback: if parsing fails, use zero and log the error
		// This should never happen with valid wei values, but defensive programming
		decimalValue = primitive.NewDecimal128(0, 0)
	}

	transfer := &models.Transfer{
		EventSignature: EventSignatureTransfer,
		Token:          strings.ToLower(log.Address.Hex()),
		From:           strings.ToLower(from.Hex()),
		To:             strings.ToLower(to.Hex()),
		Value:          decimalValue,
		ValueString:    valueStr, // Keep string for backward compatibility and JSON serialization
		ValueDecimal:   parseValueDecimal(value),
		BlockNumber:    log.BlockNumber,
		TxHash:         log.TxHash.Hex(),
		TxIndex:        log.TxIndex,
		LogIndex:       log.Index,
		Timestamp:      blockTime,
		CreatedAt:      time.Now(),
	}

	return transfer, nil
}

// parseValueDecimal converts wei to a decimal representation (assuming 18 decimals for ERC-20)
func parseValueDecimal(value *big.Int) float64 {
	divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	valueFloat := new(big.Float).SetInt(value)
	result, _ := new(big.Float).Quo(valueFloat, divisor).Float64()
	return result
}
