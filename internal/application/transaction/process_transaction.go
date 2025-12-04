package transaction 

import (
	"time"
	"fmt"
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"golang.org/x/sync/errgroup"

	"fraud-detecction-system/internal/application/dto"
	"fraud-detecction-system/internal/domain/fraud"
	"fraud-detecction-system/internal/domain/transaction"
	)

// ProcessTransactionUseCase orchestrates transaction processing with fraud detection
// This is the main entry point for all incoming transactions 
type ProcessTransactionUseCase struct {
	// Domain services 
	txService *transaction.Service
	fraudService *fraud.Service

	// Configs
	fraudCheckTimeout time.Duration
	enableAsync bool
}

// NewProcessTransctionUseCase creates a new use case instance
func NewProcessTransctionUseCase(
	txService *transaction.Service,
	fraudeService *fraud.Service,
) *ProcessTransactionUseCase{
	return &ProcessTransactionUseCase{
		txService: txService,
		fraudService: fraudeService,
		fraudCheckTimeout: 200 * time.Millisecond, //p99 target
		enableAsync: false,  //Synchronous by default for correctness 
	}
}