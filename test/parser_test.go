package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"recon-engine/internal/domain"
	"recon-engine/internal/parser"
)

func TestCSVBankStatementParser_Parse(t *testing.T) {
	// Create temporary CSV file
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "bank_test.csv")

	csvContent := `trx_ref_id,amount,date
TX001,100.50,2024-01-15
TX002,-200.75,2024-01-16
TX003,300.00,2024-01-17
`

	err := os.WriteFile(csvFile, []byte(csvContent), 0644)
	assert.NoError(t, err)

	// Parse CSV
	parser := parser.NewCSVBankStatementParser("TestBank")
	var statements []domain.BankStatement

	err = parser.Parse(csvFile, 100, func(batch []domain.BankStatement) error {
		statements = append(statements, batch...)
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, len(statements))
	assert.Equal(t, "TX001", statements[0].TrxRefID)
	assert.Equal(t, "TestBank", statements[0].Source)
}

func TestCSVBankStatementParser_InvalidFormat(t *testing.T) {
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "invalid.csv")

	// Missing required columns
	csvContent := `id,value
1,100
`

	err := os.WriteFile(csvFile, []byte(csvContent), 0644)
	assert.NoError(t, err)

	parser := parser.NewCSVBankStatementParser("TestBank")
	err = parser.Parse(csvFile, 100, func(batch []domain.BankStatement) error {
		return nil
	})

	assert.Error(t, err)
}

func TestTransactionCSVParser_Parse(t *testing.T) {
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "transactions.csv")

	csvContent := `trx_id,amount,type,transaction_time
TX001,100.00,DEBIT,2024-01-15T10:00:00Z
TX002,200.00,CREDIT,2024-01-16T11:00:00Z
TX003,300.00,DEBIT,2024-01-17T12:00:00Z
`

	err := os.WriteFile(csvFile, []byte(csvContent), 0644)
	assert.NoError(t, err)

	parser := parser.NewTransactionCSVParser()
	var transactions []domain.Transaction

	err = parser.Parse(csvFile, 100, func(batch []domain.Transaction) error {
		transactions = append(transactions, batch...)
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 3, len(transactions))
	assert.Equal(t, "TX001", transactions[0].TrxID)
	assert.Equal(t, domain.Debit, transactions[0].Type)
}

func TestTransactionCSVParser_SkipsInvalidRows(t *testing.T) {
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "transactions_with_errors.csv")

	csvContent := `trx_id,amount,type,transaction_time
TX001,100.00,DEBIT,2024-01-15T10:00:00Z
TX002,invalid,CREDIT,2024-01-16T11:00:00Z
TX003,300.00,INVALID_TYPE,2024-01-17T12:00:00Z
TX004,400.00,CREDIT,2024-01-18T13:00:00Z
`

	err := os.WriteFile(csvFile, []byte(csvContent), 0644)
	assert.NoError(t, err)

	parser := parser.NewTransactionCSVParser()
	var transactions []domain.Transaction

	err = parser.Parse(csvFile, 100, func(batch []domain.Transaction) error {
		transactions = append(transactions, batch...)
		return nil
	})

	assert.NoError(t, err)
	// Should only parse valid rows (TX001 and TX004)
	assert.Equal(t, 2, len(transactions))
}
