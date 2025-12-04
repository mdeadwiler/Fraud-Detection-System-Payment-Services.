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

