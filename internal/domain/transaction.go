package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// TransactionType represents the type of transaction
type TransactionType string

const (
	Debit  TransactionType = "DEBIT"
	Credit TransactionType = "CREDIT"
)

// Transaction represents a system transaction
type Transaction struct {
	ID              int             `json:"id" db:"id"`
	TrxID           string          `json:"trx_id" db:"trx_id"`
	Amount          decimal.Decimal `json:"amount" db:"amount"`
	Type            TransactionType `json:"type" db:"type"`
	TransactionTime time.Time       `json:"transaction_time" db:"transaction_time"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at" db:"updated_at"`
}

// BankStatement represents a bank statement entry
type BankStatement struct {
	TrxRefID string          `json:"trx_ref_id"`
	Amount   decimal.Decimal `json:"amount"`
	Date     time.Time       `json:"date"`
	Source   string          `json:"source"` // Bank identifier
}

// MatchStatus represents the reconciliation match status
type MatchStatus string

const (
	Matched          MatchStatus = "MATCHED"
	UnmatchedSystem  MatchStatus = "UNMATCHED_SYSTEM"
	UnmatchedBank    MatchStatus = "UNMATCHED_BANK"
	Discrepancy      MatchStatus = "DISCREPANCY"
)

// ReconciliationResult represents the result of matching
type ReconciliationResult struct {
	ID              int             `json:"id" db:"id"`
	JobID           string          `json:"job_id" db:"job_id"`
	TrxID           *string         `json:"trx_id,omitempty" db:"trx_id"`
	TrxRefID        *string         `json:"trx_ref_id,omitempty" db:"trx_ref_id"`
	SystemAmount    *decimal.Decimal `json:"system_amount,omitempty" db:"system_amount"`
	BankAmount      *decimal.Decimal `json:"bank_amount,omitempty" db:"bank_amount"`
	Discrepancy     *decimal.Decimal `json:"discrepancy,omitempty" db:"discrepancy"`
	MatchStatus     MatchStatus     `json:"match_status" db:"match_status"`
	BankSource      *string         `json:"bank_source,omitempty" db:"bank_source"`
	TransactionDate *time.Time      `json:"transaction_date,omitempty" db:"transaction_date"`
	CreatedAt       time.Time       `json:"created_at" db:"created_at"`
}

// JobStatus represents the status of a reconciliation job
type JobStatus string

const (
	Pending    JobStatus = "PENDING"
	Processing JobStatus = "PROCESSING"
	Completed  JobStatus = "COMPLETED"
	Failed     JobStatus = "FAILED"
)

// ReconciliationJob represents a reconciliation job
type ReconciliationJob struct {
	ID                  int             `json:"id" db:"id"`
	JobID               string          `json:"job_id" db:"job_id"`
	StartDate           time.Time       `json:"start_date" db:"start_date"`
	EndDate             time.Time       `json:"end_date" db:"end_date"`
	Status              JobStatus       `json:"status" db:"status"`
	TotalProcessed      int             `json:"total_processed" db:"total_processed"`
	TotalMatched        int             `json:"total_matched" db:"total_matched"`
	TotalUnmatched      int             `json:"total_unmatched" db:"total_unmatched"`
	TotalDiscrepancies  decimal.Decimal `json:"total_discrepancies" db:"total_discrepancies"`
	ErrorMessage        *string         `json:"error_message,omitempty" db:"error_message"`
	CreatedAt           time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at" db:"updated_at"`
}

// ReconciliationSummary represents the summary output
type ReconciliationSummary struct {
	JobID              string                     `json:"job_id"`
	TotalProcessed     int                        `json:"total_processed"`
	TotalMatched       int                        `json:"total_matched"`
	TotalUnmatched     int                        `json:"total_unmatched"`
	TotalDiscrepancies decimal.Decimal            `json:"total_discrepancies"`
	UnmatchedSystem    []ReconciliationResult     `json:"unmatched_system,omitempty"`
	UnmatchedBank      map[string][]ReconciliationResult `json:"unmatched_bank,omitempty"`
	Discrepancies      []ReconciliationResult     `json:"discrepancies,omitempty"`
}
