package service

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"recon-engine/internal/domain"
	"recon-engine/internal/matcher"
	"recon-engine/internal/parser"
	"recon-engine/internal/repository"
	"recon-engine/pkg/logger"
)

type ReconciliationService interface {
	Reconcile(systemFilePath string, bankFilePaths []string, startDate, endDate time.Time) (*domain.ReconciliationSummary, error)
	GetJobStatus(jobID string) (*domain.ReconciliationJob, error)
	GetJobSummary(jobID string) (*domain.ReconciliationSummary, error)
}

type reconciliationService struct {
	txRepo    repository.TransactionRepository
	reconRepo repository.ReconciliationRepository
	engine    *matcher.ReconciliationEngine
	batchSize int
}

func NewReconciliationService(
	txRepo repository.TransactionRepository,
	reconRepo repository.ReconciliationRepository,
	batchSize int,
) ReconciliationService {
	return &reconciliationService{
		txRepo:    txRepo,
		reconRepo: reconRepo,
		engine:    matcher.NewReconciliationEngine(&matcher.ExactMatchStrategy{}),
		batchSize: batchSize,
	}
}

func (s *reconciliationService) Reconcile(
	systemFilePath string,
	bankFilePaths []string,
	startDate, endDate time.Time,
) (*domain.ReconciliationSummary, error) {
	// Create reconciliation job
	jobID := uuid.New().String()
	job := &domain.ReconciliationJob{
		JobID:              jobID,
		StartDate:          startDate,
		EndDate:            endDate,
		Status:             domain.Processing,
		TotalDiscrepancies: decimal.Zero,
	}

	if err := s.reconRepo.CreateJob(job); err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	logger.GetLogger().WithField("job_id", jobID).Info("Starting reconciliation job")

	// Load system transactions from database
	systemTransactions, err := s.txRepo.GetByDateRange(startDate, endDate)
	if err != nil {
		s.updateJobStatus(jobID, domain.Failed, err.Error())
		return nil, fmt.Errorf("failed to load system transactions: %w", err)
	}

	// If system file path is provided, load from CSV instead
	if systemFilePath != "" {
		systemTransactions, err = s.loadSystemTransactionsFromCSV(systemFilePath)
		if err != nil {
			s.updateJobStatus(jobID, domain.Failed, err.Error())
			return nil, fmt.Errorf("failed to load system transactions from CSV: %w", err)
		}
	}

	// Load bank statements from all CSV files
	var allBankStatements []domain.BankStatement
	for _, bankFilePath := range bankFilePaths {
		bankStatements, err := s.loadBankStatementsFromCSV(bankFilePath)
		if err != nil {
			logger.GetLogger().WithError(err).WithField("file", bankFilePath).Warn("Failed to load bank statements")
			continue
		}
		allBankStatements = append(allBankStatements, bankStatements...)
	}

	if len(allBankStatements) == 0 {
		s.updateJobStatus(jobID, domain.Failed, "no bank statements loaded")
		return nil, fmt.Errorf("no bank statements loaded")
	}

	// Filter by date range
	systemTransactions = s.filterByDateRange(systemTransactions, startDate, endDate)
	allBankStatements = s.filterBankStatementsByDateRange(allBankStatements, startDate, endDate)

	// Perform reconciliation
	reconInput := matcher.ReconciliationInput{
		SystemTransactions: systemTransactions,
		BankStatements:     allBankStatements,
		StartDate:          startDate,
		EndDate:            endDate,
	}

	if err := matcher.ValidateReconciliationInput(reconInput); err != nil {
		s.updateJobStatus(jobID, domain.Failed, err.Error())
		return nil, err
	}

	output, err := s.engine.Reconcile(reconInput)
	if err != nil {
		s.updateJobStatus(jobID, domain.Failed, err.Error())
		return nil, fmt.Errorf("reconciliation failed: %w", err)
	}

	// Save results
	results := s.engine.BuildResults(jobID, output)
	if err := s.reconRepo.BulkCreateResults(results); err != nil {
		logger.GetLogger().WithError(err).Error("Failed to save results")
	}

	// Update job status
	totalDiscrepancies := s.engine.CalculateDiscrepancyTotal(output)
	job.TotalProcessed = len(systemTransactions) + len(allBankStatements)
	job.TotalMatched = len(output.Matched)
	job.TotalUnmatched = len(output.UnmatchedSystem) + len(output.UnmatchedBank)
	job.TotalDiscrepancies = totalDiscrepancies
	job.Status = domain.Completed

	if err := s.reconRepo.UpdateJob(job); err != nil {
		logger.GetLogger().WithError(err).Error("Failed to update job")
	}

	// Build summary
	summary := s.buildSummary(jobID, output, job)

	logger.GetLogger().WithField("job_id", jobID).Info("Reconciliation job completed")

	return summary, nil
}

func (s *reconciliationService) GetJobStatus(jobID string) (*domain.ReconciliationJob, error) {
	return s.reconRepo.GetJobByID(jobID)
}

func (s *reconciliationService) GetJobSummary(jobID string) (*domain.ReconciliationSummary, error) {
	job, err := s.reconRepo.GetJobByID(jobID)
	if err != nil {
		return nil, err
	}

	discrepancies, _ := s.reconRepo.GetResultsByJobIDAndStatus(jobID, domain.Discrepancy)
	unmatchedSystem, _ := s.reconRepo.GetResultsByJobIDAndStatus(jobID, domain.UnmatchedSystem)
	unmatchedBank, _ := s.reconRepo.GetResultsByJobIDAndStatus(jobID, domain.UnmatchedBank)

	// Group unmatched bank by source
	unmatchedBankBySource := make(map[string][]domain.ReconciliationResult)
	for _, result := range unmatchedBank {
		source := "unknown"
		if result.BankSource != nil {
			source = *result.BankSource
		}
		unmatchedBankBySource[source] = append(unmatchedBankBySource[source], result)
	}

	return &domain.ReconciliationSummary{
		JobID:              jobID,
		TotalProcessed:     job.TotalProcessed,
		TotalMatched:       job.TotalMatched,
		TotalUnmatched:     job.TotalUnmatched,
		TotalDiscrepancies: job.TotalDiscrepancies,
		UnmatchedSystem:    unmatchedSystem,
		UnmatchedBank:      unmatchedBankBySource,
		Discrepancies:      discrepancies,
	}, nil
}

func (s *reconciliationService) loadSystemTransactionsFromCSV(filePath string) ([]domain.Transaction, error) {
	parser := parser.NewTransactionCSVParser()
	var transactions []domain.Transaction

	err := parser.Parse(filePath, s.batchSize, func(batch []domain.Transaction) error {
		transactions = append(transactions, batch...)
		return nil
	})

	return transactions, err
}

func (s *reconciliationService) loadBankStatementsFromCSV(filePath string) ([]domain.BankStatement, error) {
	source := extractBankSource(filePath)
	parser := parser.NewCSVBankStatementParser(source)
	var statements []domain.BankStatement

	err := parser.Parse(filePath, s.batchSize, func(batch []domain.BankStatement) error {
		statements = append(statements, batch...)
		return nil
	})

	return statements, err
}

func (s *reconciliationService) filterByDateRange(transactions []domain.Transaction, startDate, endDate time.Time) []domain.Transaction {
	filtered := make([]domain.Transaction, 0)
	for _, tx := range transactions {
		if (tx.TransactionTime.Equal(startDate) || tx.TransactionTime.After(startDate)) &&
			(tx.TransactionTime.Equal(endDate) || tx.TransactionTime.Before(endDate)) {
			filtered = append(filtered, tx)
		}
	}
	return filtered
}

func (s *reconciliationService) filterBankStatementsByDateRange(statements []domain.BankStatement, startDate, endDate time.Time) []domain.BankStatement {
	filtered := make([]domain.BankStatement, 0)
	for _, stmt := range statements {
		if (stmt.Date.Equal(startDate) || stmt.Date.After(startDate)) &&
			(stmt.Date.Equal(endDate) || stmt.Date.Before(endDate)) {
			filtered = append(filtered, stmt)
		}
	}
	return filtered
}

func (s *reconciliationService) updateJobStatus(jobID string, status domain.JobStatus, errorMsg string) {
	job, err := s.reconRepo.GetJobByID(jobID)
	if err != nil {
		return
	}

	job.Status = status
	if errorMsg != "" {
		job.ErrorMessage = &errorMsg
	}

	s.reconRepo.UpdateJob(job)
}

func (s *reconciliationService) buildSummary(jobID string, output *matcher.ReconciliationOutput, job *domain.ReconciliationJob) *domain.ReconciliationSummary {
	// Convert matched pairs to results
	matched := make([]domain.ReconciliationResult, len(output.Matched))
	for i, m := range output.Matched {
		matched[i] = domain.ReconciliationResult{
			JobID:           jobID,
			TrxID:           &m.SystemTx.TrxID,
			TrxRefID:        &m.BankStmt.TrxRefID,
			SystemAmount:    &m.SystemTx.Amount,
			BankAmount:      &m.BankStmt.Amount,
			MatchStatus:     domain.Matched,
			BankSource:      &m.BankStmt.Source,
			TransactionDate: &m.SystemTx.TransactionTime,
		}
	}

	// Convert discrepancies
	discrepancies := make([]domain.ReconciliationResult, len(output.Discrepancies))
	for i, d := range output.Discrepancies {
		discrepancies[i] = domain.ReconciliationResult{
			JobID:           jobID,
			TrxID:           &d.SystemTx.TrxID,
			TrxRefID:        &d.BankStmt.TrxRefID,
			SystemAmount:    &d.SystemTx.Amount,
			BankAmount:      &d.BankStmt.Amount,
			Discrepancy:     &d.Discrepancy,
			MatchStatus:     domain.Discrepancy,
			BankSource:      &d.BankStmt.Source,
			TransactionDate: &d.SystemTx.TransactionTime,
		}
	}

	// Convert unmatched system
	unmatchedSystem := make([]domain.ReconciliationResult, len(output.UnmatchedSystem))
	for i, u := range output.UnmatchedSystem {
		unmatchedSystem[i] = domain.ReconciliationResult{
			JobID:           jobID,
			TrxID:           &u.TrxID,
			SystemAmount:    &u.Amount,
			MatchStatus:     domain.UnmatchedSystem,
			TransactionDate: &u.TransactionTime,
		}
	}

	// Convert and group unmatched bank by source
	unmatchedBankBySource := make(map[string][]domain.ReconciliationResult)
	for _, u := range output.UnmatchedBank {
		result := domain.ReconciliationResult{
			JobID:           jobID,
			TrxRefID:        &u.TrxRefID,
			BankAmount:      &u.Amount,
			MatchStatus:     domain.UnmatchedBank,
			BankSource:      &u.Source,
			TransactionDate: &u.Date,
		}
		unmatchedBankBySource[u.Source] = append(unmatchedBankBySource[u.Source], result)
	}

	return &domain.ReconciliationSummary{
		JobID:              jobID,
		TotalProcessed:     job.TotalProcessed,
		TotalMatched:       job.TotalMatched,
		TotalUnmatched:     job.TotalUnmatched,
		TotalDiscrepancies: job.TotalDiscrepancies,
		UnmatchedSystem:    unmatchedSystem,
		UnmatchedBank:      unmatchedBankBySource,
		Discrepancies:      discrepancies,
	}
}

func extractBankSource(filePath string) string {
	fileName := filepath.Base(filePath)
	// Extract bank name from filename (e.g., "bank_bca.csv" -> "bca")
	return fileName
}
