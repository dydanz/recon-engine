package parser

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"recon-engine/internal/domain"
	"recon-engine/pkg/logger"
)

// BankStatementParser interface for parsing bank statement files
type BankStatementParser interface {
	Parse(filePath string, batchSize int, callback func([]domain.BankStatement) error) error
}

// CSVBankStatementParser implements streaming CSV parser
type CSVBankStatementParser struct {
	source string // Bank identifier
}

func NewCSVBankStatementParser(source string) *CSVBankStatementParser {
	return &CSVBankStatementParser{source: source}
}

// Parse reads CSV file in streaming mode and processes in batches
func (p *CSVBankStatementParser) Parse(filePath string, batchSize int, callback func([]domain.BankStatement) error) error {
	file, err := os.Open(filePath)
	if err != nil {
		logger.GetLogger().WithError(err).WithField("file", filePath).Error("Failed to open file")
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	// Read header
	header, err := reader.Read()
	if err != nil {
		logger.GetLogger().WithError(err).Error("Failed to read CSV header")
		return fmt.Errorf("failed to read header: %w", err)
	}

	// Map header columns
	columnMap := mapColumns(header)
	if !validateColumns(columnMap) {
		return fmt.Errorf("invalid CSV format: missing required columns (trx_ref_id, amount, date)")
	}

	batch := make([]domain.BankStatement, 0, batchSize)
	lineNumber := 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.GetLogger().WithError(err).WithField("line", lineNumber).Warn("Failed to read CSV row, skipping")
			lineNumber++
			continue
		}

		lineNumber++

		statement, err := p.parseRecord(record, columnMap, lineNumber)
		if err != nil {
			logger.GetLogger().WithError(err).WithField("line", lineNumber).Warn("Failed to parse record, skipping")
			continue
		}

		batch = append(batch, *statement)

		if len(batch) >= batchSize {
			if err := callback(batch); err != nil {
				return err
			}
			batch = make([]domain.BankStatement, 0, batchSize)
		}
	}

	// Process remaining items
	if len(batch) > 0 {
		if err := callback(batch); err != nil {
			return err
		}
	}

	return nil
}

func (p *CSVBankStatementParser) parseRecord(record []string, columnMap map[string]int, lineNumber int) (*domain.BankStatement, error) {
	if len(record) < len(columnMap) {
		return nil, fmt.Errorf("incomplete record at line %d", lineNumber)
	}

	// Parse trx_ref_id
	trxRefID := strings.TrimSpace(record[columnMap["trx_ref_id"]])
	if trxRefID == "" {
		return nil, fmt.Errorf("empty trx_ref_id at line %d", lineNumber)
	}

	// Parse amount
	amountStr := strings.TrimSpace(record[columnMap["amount"]])
	amount, err := decimal.NewFromString(amountStr)
	if err != nil {
		return nil, fmt.Errorf("invalid amount '%s' at line %d: %w", amountStr, lineNumber, err)
	}

	// Parse date - try multiple formats
	dateStr := strings.TrimSpace(record[columnMap["date"]])
	date, err := parseDate(dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date '%s' at line %d: %w", dateStr, lineNumber, err)
	}

	return &domain.BankStatement{
		TrxRefID: trxRefID,
		Amount:   amount,
		Date:     date,
		Source:   p.source,
	}, nil
}

func mapColumns(header []string) map[string]int {
	columnMap := make(map[string]int)
	for i, col := range header {
		normalized := strings.ToLower(strings.TrimSpace(col))
		columnMap[normalized] = i
	}
	return columnMap
}

func validateColumns(columnMap map[string]int) bool {
	requiredColumns := []string{"trx_ref_id", "amount", "date"}
	for _, col := range requiredColumns {
		if _, exists := columnMap[col]; !exists {
			return false
		}
	}
	return true
}

func parseDate(dateStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		"02/01/2006",
		"01/02/2006",
		"2006/01/02",
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// TransactionCSVParser for parsing system transactions from CSV
type TransactionCSVParser struct{}

func NewTransactionCSVParser() *TransactionCSVParser {
	return &TransactionCSVParser{}
}

func (p *TransactionCSVParser) Parse(filePath string, batchSize int, callback func([]domain.Transaction) error) error {
	file, err := os.Open(filePath)
	if err != nil {
		logger.GetLogger().WithError(err).WithField("file", filePath).Error("Failed to open file")
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	// Read header
	header, err := reader.Read()
	if err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}

	columnMap := mapColumns(header)
	if !validateTransactionColumns(columnMap) {
		return fmt.Errorf("invalid CSV format: missing required columns")
	}

	batch := make([]domain.Transaction, 0, batchSize)
	lineNumber := 1

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.GetLogger().WithError(err).WithField("line", lineNumber).Warn("Failed to read CSV row, skipping")
			lineNumber++
			continue
		}

		lineNumber++

		transaction, err := p.parseTransactionRecord(record, columnMap, lineNumber)
		if err != nil {
			logger.GetLogger().WithError(err).WithField("line", lineNumber).Warn("Failed to parse record, skipping")
			continue
		}

		batch = append(batch, *transaction)

		if len(batch) >= batchSize {
			if err := callback(batch); err != nil {
				return err
			}
			batch = make([]domain.Transaction, 0, batchSize)
		}
	}

	if len(batch) > 0 {
		if err := callback(batch); err != nil {
			return err
		}
	}

	return nil
}

func (p *TransactionCSVParser) parseTransactionRecord(record []string, columnMap map[string]int, lineNumber int) (*domain.Transaction, error) {
	trxID := strings.TrimSpace(record[columnMap["trx_id"]])
	if trxID == "" {
		return nil, fmt.Errorf("empty trx_id")
	}

	amountStr := strings.TrimSpace(record[columnMap["amount"]])
	amount, err := decimal.NewFromString(amountStr)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}

	typeStr := strings.ToUpper(strings.TrimSpace(record[columnMap["type"]]))
	if typeStr != string(domain.Debit) && typeStr != string(domain.Credit) {
		return nil, fmt.Errorf("invalid transaction type: %s", typeStr)
	}

	timeStr := strings.TrimSpace(record[columnMap["transaction_time"]])
	transactionTime, err := parseDate(timeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid transaction_time: %w", err)
	}

	return &domain.Transaction{
		TrxID:           trxID,
		Amount:          amount,
		Type:            domain.TransactionType(typeStr),
		TransactionTime: transactionTime,
	}, nil
}

func validateTransactionColumns(columnMap map[string]int) bool {
	requiredColumns := []string{"trx_id", "amount", "type", "transaction_time"}
	for _, col := range requiredColumns {
		if _, exists := columnMap[col]; !exists {
			return false
		}
	}
	return true
}

// Helper function to convert amount based on transaction type for bank statements
func NormalizeAmount(amount decimal.Decimal, isNegative bool) decimal.Decimal {
	if isNegative && amount.IsPositive() {
		return amount.Neg()
	}
	return amount
}

// ParseFloat helper for backward compatibility
func ParseFloat(s string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(s), 64)
}
