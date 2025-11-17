package transaction

import "errors"

var (
	// ErrTransactionNotFound is returned when a transaction cannot be found
	ErrTransactionNotFound = errors.New("transaction not found")

	// ErrInvalidTransactionID is returned when the transaction ID is invalid
	ErrInvalidTransactionID = errors.New("invalid transaction ID")

	// ErrInvalidUserID is returned when the user ID is invalid
	ErrInvalidUserID = errors.New("invalid user ID")

	// ErrInvalidAccountID is returned when the account ID is invalid
	ErrInvalidAccountID = errors.New("invalid account ID")

	// ErrNegativeAmount is returned when transaction amount is negative
	ErrNegativeAmount = errors.New("transaction amount cannot be negative")

	// ErrZeroAmount is returned when transaction amount is zero
	ErrZeroAmount = errors.New("transaction amount cannot be zero")

	// ErrMissingCurrency is returned when currency is not specified
	ErrMissingCurrency = errors.New("transaction currency is required")

	// ErrInvalidStatusTransition is returned when an invalid status change is attempted
	ErrInvalidStatusTransition = errors.New("invalid transaction status transition")

	// ErrTransactionAlreadyProcessed is returned when trying to modify a processed transaction
	ErrTransactionAlreadyProcessed = errors.New("transaction has already been processed")

	// ErrInvalidAmount is returned when the amount format is invalid
	ErrInvalidAmount = errors.New("invalid transaction amount")

	// ErrDuplicateTransaction is returned when a duplicate transaction is detected
	ErrDuplicateTransaction = errors.New("duplicate transaction detected")
)
