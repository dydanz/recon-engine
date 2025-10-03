package test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"recon-engine/internal/domain"
	"recon-engine/internal/matcher"
)

func TestReconciliationEngine_Reconcile(t *testing.T) {
	engine := matcher.NewReconciliationEngine(&matcher.ExactMatchStrategy{})

	now := time.Now()

	systemTxs := []domain.Transaction{
		{TrxID: "TX001", Amount: decimal.NewFromFloat(100.00), Type: domain.Debit, TransactionTime: now},
		{TrxID: "TX002", Amount: decimal.NewFromFloat(200.00), Type: domain.Credit, TransactionTime: now},
		{TrxID: "TX003", Amount: decimal.NewFromFloat(300.00), Type: domain.Debit, TransactionTime: now},
		{TrxID: "TX004", Amount: decimal.NewFromFloat(400.00), Type: domain.Credit, TransactionTime: now},
	}

	bankStmts := []domain.BankStatement{
		{TrxRefID: "TX001", Amount: decimal.NewFromFloat(-100.00), Date: now, Source: "BankA"},
		{TrxRefID: "TX002", Amount: decimal.NewFromFloat(200.00), Date: now, Source: "BankA"},
		{TrxRefID: "TX003", Amount: decimal.NewFromFloat(-350.00), Date: now, Source: "BankB"}, // Discrepancy
		{TrxRefID: "TX999", Amount: decimal.NewFromFloat(999.00), Date: now, Source: "BankB"},  // Unmatched
	}

	input := matcher.ReconciliationInput{
		SystemTransactions: systemTxs,
		BankStatements:     bankStmts,
		StartDate:          now.Add(-24 * time.Hour),
		EndDate:            now.Add(24 * time.Hour),
	}

	output, err := engine.Reconcile(input)

	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.Equal(t, 2, len(output.Matched), "Should have 2 perfect matches (TX001, TX002)")
	assert.Equal(t, 1, len(output.Discrepancies), "Should have 1 discrepancy (TX003)")
	assert.Equal(t, 1, len(output.UnmatchedSystem), "Should have 1 unmatched system tx (TX004)")
	assert.Equal(t, 1, len(output.UnmatchedBank), "Should have 1 unmatched bank stmt (TX999)")
}

func TestReconciliationEngine_BuildResults(t *testing.T) {
	engine := matcher.NewReconciliationEngine(&matcher.ExactMatchStrategy{})
	jobID := "test-job-123"

	now := time.Now()

	output := &matcher.ReconciliationOutput{
		Matched: []matcher.MatchedPair{
			{
				SystemTx: domain.Transaction{TrxID: "TX001", Amount: decimal.NewFromFloat(100.00), Type: domain.Credit, TransactionTime: now},
				BankStmt: domain.BankStatement{TrxRefID: "TX001", Amount: decimal.NewFromFloat(100.00), Date: now, Source: "BankA"},
			},
		},
		UnmatchedSystem: []domain.Transaction{
			{TrxID: "TX002", Amount: decimal.NewFromFloat(200.00), Type: domain.Debit, TransactionTime: now},
		},
		UnmatchedBank: []domain.BankStatement{
			{TrxRefID: "TX003", Amount: decimal.NewFromFloat(300.00), Date: now, Source: "BankB"},
		},
		Discrepancies: []matcher.DiscrepancyPair{
			{
				SystemTx:    domain.Transaction{TrxID: "TX004", Amount: decimal.NewFromFloat(400.00), Type: domain.Credit, TransactionTime: now},
				BankStmt:    domain.BankStatement{TrxRefID: "TX004", Amount: decimal.NewFromFloat(450.00), Date: now, Source: "BankA"},
				Discrepancy: decimal.NewFromFloat(50.00),
			},
		},
	}

	results := engine.BuildResults(jobID, output)

	assert.Equal(t, 4, len(results), "Should have 4 total results")

	// Check matched
	matchedCount := 0
	for _, r := range results {
		if r.MatchStatus == domain.Matched {
			matchedCount++
		}
	}
	assert.Equal(t, 1, matchedCount)
}

func TestReconciliationEngine_CalculateDiscrepancyTotal(t *testing.T) {
	engine := matcher.NewReconciliationEngine(&matcher.ExactMatchStrategy{})

	output := &matcher.ReconciliationOutput{
		Discrepancies: []matcher.DiscrepancyPair{
			{Discrepancy: decimal.NewFromFloat(50.00)},
			{Discrepancy: decimal.NewFromFloat(30.00)},
			{Discrepancy: decimal.NewFromFloat(20.00)},
		},
	}

	total := engine.CalculateDiscrepancyTotal(output)

	assert.True(t, total.Equal(decimal.NewFromFloat(100.00)), "Total discrepancy should be 100.00")
}

func TestValidateReconciliationInput(t *testing.T) {
	now := time.Now()

	// Valid input
	validInput := matcher.ReconciliationInput{
		StartDate: now.Add(-24 * time.Hour),
		EndDate:   now,
	}

	err := matcher.ValidateReconciliationInput(validInput)
	assert.NoError(t, err)

	// Invalid: start date after end date
	invalidInput := matcher.ReconciliationInput{
		StartDate: now,
		EndDate:   now.Add(-24 * time.Hour),
	}

	err = matcher.ValidateReconciliationInput(invalidInput)
	assert.Error(t, err)
}
