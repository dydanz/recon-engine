package matcher

import (
	"fmt"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"recon-engine/internal/domain"
	"recon-engine/pkg/logger"
)

// MatchingStrategy defines the interface for different matching strategies
type MatchingStrategy interface {
	Match(systemTx domain.Transaction, bankStmt domain.BankStatement) bool
}

// ExactMatchStrategy matches by exact ID
type ExactMatchStrategy struct{}

func (s *ExactMatchStrategy) Match(systemTx domain.Transaction, bankStmt domain.BankStatement) bool {
	return systemTx.TrxID == bankStmt.TrxRefID
}

// ReconciliationEngine performs the reconciliation using hash-based matching
type ReconciliationEngine struct {
	strategy MatchingStrategy
	mu       sync.RWMutex
}

func NewReconciliationEngine(strategy MatchingStrategy) *ReconciliationEngine {
	if strategy == nil {
		strategy = &ExactMatchStrategy{}
	}
	return &ReconciliationEngine{
		strategy: strategy,
	}
}

// ReconciliationInput contains all input data for reconciliation
type ReconciliationInput struct {
	SystemTransactions []domain.Transaction
	BankStatements     []domain.BankStatement
	StartDate          time.Time
	EndDate            time.Time
}

// ReconciliationOutput contains the results
type ReconciliationOutput struct {
	Matched         []MatchedPair
	UnmatchedSystem []domain.Transaction
	UnmatchedBank   []domain.BankStatement
	Discrepancies   []DiscrepancyPair
}

// MatchedPair represents a matched transaction
type MatchedPair struct {
	SystemTx domain.Transaction
	BankStmt domain.BankStatement
}

// DiscrepancyPair represents a transaction with amount discrepancy
type DiscrepancyPair struct {
	SystemTx    domain.Transaction
	BankStmt    domain.BankStatement
	Discrepancy decimal.Decimal
}

// Reconcile performs the two-phase reconciliation process
func (e *ReconciliationEngine) Reconcile(input ReconciliationInput) (*ReconciliationOutput, error) {
	logger.GetLogger().WithFields(map[string]interface{}{
		"system_count": len(input.SystemTransactions),
		"bank_count":   len(input.BankStatements),
		"start_date":   input.StartDate,
		"end_date":     input.EndDate,
	}).Info("Starting reconciliation")

	// Phase 1: Build hash maps for O(1) lookup
	bankMap := e.buildBankMap(input.BankStatements)

	output := &ReconciliationOutput{
		Matched:         make([]MatchedPair, 0),
		UnmatchedSystem: make([]domain.Transaction, 0),
		UnmatchedBank:   make([]domain.BankStatement, 0),
		Discrepancies:   make([]DiscrepancyPair, 0),
	}

	// Phase 2: Match and categorize
	matchedBankIDs := make(map[string]bool)

	// Iterate through system transactions
	for _, sysTx := range input.SystemTransactions {
		// Try to find matching bank statement
		bankStmt, found := bankMap[sysTx.TrxID]

		if !found {
			// Unmatched in system
			output.UnmatchedSystem = append(output.UnmatchedSystem, sysTx)
			continue
		}

		// Mark as matched
		matchedBankIDs[bankStmt.TrxRefID] = true

		// Check for amount discrepancy
		systemAmount := e.normalizeAmount(sysTx)
		discrepancy := systemAmount.Sub(bankStmt.Amount).Abs()

		if !discrepancy.IsZero() {
			// Amount mismatch
			output.Discrepancies = append(output.Discrepancies, DiscrepancyPair{
				SystemTx:    sysTx,
				BankStmt:    bankStmt,
				Discrepancy: discrepancy,
			})
		} else {
			// Perfect match
			output.Matched = append(output.Matched, MatchedPair{
				SystemTx: sysTx,
				BankStmt: bankStmt,
			})
		}
	}

	// Find unmatched bank statements
	for _, bankStmt := range input.BankStatements {
		if !matchedBankIDs[bankStmt.TrxRefID] {
			output.UnmatchedBank = append(output.UnmatchedBank, bankStmt)
		}
	}

	logger.GetLogger().WithFields(map[string]interface{}{
		"matched":          len(output.Matched),
		"unmatched_system": len(output.UnmatchedSystem),
		"unmatched_bank":   len(output.UnmatchedBank),
		"discrepancies":    len(output.Discrepancies),
	}).Info("Reconciliation completed")

	return output, nil
}

// buildSystemMap creates a hash map indexed by transaction ID
func (e *ReconciliationEngine) buildSystemMap(transactions []domain.Transaction) map[string]domain.Transaction {
	systemMap := make(map[string]domain.Transaction, len(transactions))
	for _, tx := range transactions {
		systemMap[tx.TrxID] = tx
	}
	return systemMap
}

// buildBankMap creates a hash map indexed by reference ID
func (e *ReconciliationEngine) buildBankMap(statements []domain.BankStatement) map[string]domain.BankStatement {
	bankMap := make(map[string]domain.BankStatement, len(statements))
	for _, stmt := range statements {
		// If duplicate, keep the first one (or implement your own logic)
		if _, exists := bankMap[stmt.TrxRefID]; !exists {
			bankMap[stmt.TrxRefID] = stmt
		}
	}
	return bankMap
}

// normalizeAmount converts transaction amount based on type
// DEBIT should be negative, CREDIT should be positive
func (e *ReconciliationEngine) normalizeAmount(tx domain.Transaction) decimal.Decimal {
	if tx.Type == domain.Debit {
		return tx.Amount.Neg()
	}
	return tx.Amount
}

// BuildResults converts reconciliation output to domain results
func (e *ReconciliationEngine) BuildResults(jobID string, output *ReconciliationOutput) []domain.ReconciliationResult {
	results := make([]domain.ReconciliationResult, 0)

	// Matched transactions
	for _, matched := range output.Matched {
		results = append(results, domain.ReconciliationResult{
			JobID:           jobID,
			TrxID:           &matched.SystemTx.TrxID,
			TrxRefID:        &matched.BankStmt.TrxRefID,
			SystemAmount:    &matched.SystemTx.Amount,
			BankAmount:      &matched.BankStmt.Amount,
			Discrepancy:     ptrDecimal(decimal.Zero),
			MatchStatus:     domain.Matched,
			BankSource:      &matched.BankStmt.Source,
			TransactionDate: &matched.SystemTx.TransactionTime,
		})
	}

	// Discrepancies
	for _, disc := range output.Discrepancies {
		results = append(results, domain.ReconciliationResult{
			JobID:           jobID,
			TrxID:           &disc.SystemTx.TrxID,
			TrxRefID:        &disc.BankStmt.TrxRefID,
			SystemAmount:    &disc.SystemTx.Amount,
			BankAmount:      &disc.BankStmt.Amount,
			Discrepancy:     &disc.Discrepancy,
			MatchStatus:     domain.Discrepancy,
			BankSource:      &disc.BankStmt.Source,
			TransactionDate: &disc.SystemTx.TransactionTime,
		})
	}

	// Unmatched system
	for _, sys := range output.UnmatchedSystem {
		results = append(results, domain.ReconciliationResult{
			JobID:           jobID,
			TrxID:           &sys.TrxID,
			SystemAmount:    &sys.Amount,
			MatchStatus:     domain.UnmatchedSystem,
			TransactionDate: &sys.TransactionTime,
		})
	}

	// Unmatched bank
	for _, bank := range output.UnmatchedBank {
		results = append(results, domain.ReconciliationResult{
			JobID:           jobID,
			TrxRefID:        &bank.TrxRefID,
			BankAmount:      &bank.Amount,
			MatchStatus:     domain.UnmatchedBank,
			BankSource:      &bank.Source,
			TransactionDate: &bank.Date,
		})
	}

	return results
}

// CalculateDiscrepancyTotal calculates sum of all discrepancies
func (e *ReconciliationEngine) CalculateDiscrepancyTotal(output *ReconciliationOutput) decimal.Decimal {
	total := decimal.Zero
	for _, disc := range output.Discrepancies {
		total = total.Add(disc.Discrepancy)
	}
	return total
}

func ptrDecimal(d decimal.Decimal) *decimal.Decimal {
	return &d
}

// StreamingReconciliationEngine performs reconciliation in batches for large datasets
type StreamingReconciliationEngine struct {
	*ReconciliationEngine
	batchSize int
}

func NewStreamingReconciliationEngine(strategy MatchingStrategy, batchSize int) *StreamingReconciliationEngine {
	return &StreamingReconciliationEngine{
		ReconciliationEngine: NewReconciliationEngine(strategy),
		batchSize:            batchSize,
	}
}

// ReconcileStreaming performs reconciliation in batches to manage memory
func (e *StreamingReconciliationEngine) ReconcileStreaming(
	systemBatches <-chan []domain.Transaction,
	bankStatements []domain.BankStatement,
) (*ReconciliationOutput, error) {

	// Build bank map once (assuming bank statements fit in memory)
	bankMap := e.buildBankMap(bankStatements)
	matchedBankIDs := make(map[string]bool)

	output := &ReconciliationOutput{
		Matched:         make([]MatchedPair, 0),
		UnmatchedSystem: make([]domain.Transaction, 0),
		UnmatchedBank:   make([]domain.BankStatement, 0),
		Discrepancies:   make([]DiscrepancyPair, 0),
	}

	// Process system transactions in batches
	for batch := range systemBatches {
		for _, sysTx := range batch {
			bankStmt, found := bankMap[sysTx.TrxID]

			if !found {
				output.UnmatchedSystem = append(output.UnmatchedSystem, sysTx)
				continue
			}

			matchedBankIDs[bankStmt.TrxRefID] = true
			systemAmount := e.normalizeAmount(sysTx)
			discrepancy := systemAmount.Sub(bankStmt.Amount).Abs()

			if !discrepancy.IsZero() {
				output.Discrepancies = append(output.Discrepancies, DiscrepancyPair{
					SystemTx:    sysTx,
					BankStmt:    bankStmt,
					Discrepancy: discrepancy,
				})
			} else {
				output.Matched = append(output.Matched, MatchedPair{
					SystemTx: sysTx,
					BankStmt: bankStmt,
				})
			}
		}
	}

	// Find unmatched bank statements
	for _, bankStmt := range bankStatements {
		if !matchedBankIDs[bankStmt.TrxRefID] {
			output.UnmatchedBank = append(output.UnmatchedBank, bankStmt)
		}
	}

	return output, nil
}

// Validate input parameters
func ValidateReconciliationInput(input ReconciliationInput) error {
	if input.StartDate.After(input.EndDate) {
		return fmt.Errorf("start date must be before or equal to end date")
	}
	return nil
}
