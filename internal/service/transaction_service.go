package service

import (
	"fmt"
	"time"

	"recon-engine/internal/domain"
	"recon-engine/internal/repository"
	"recon-engine/pkg/logger"
)

type TransactionService interface {
	Create(tx *domain.Transaction) error
	BulkCreate(transactions []domain.Transaction) error
	GetByTrxID(trxID string) (*domain.Transaction, error)
	GetByDateRange(startDate, endDate time.Time) ([]domain.Transaction, error)
}

type transactionService struct {
	repo repository.TransactionRepository
}

func NewTransactionService(repo repository.TransactionRepository) TransactionService {
	return &transactionService{repo: repo}
}

func (s *transactionService) Create(tx *domain.Transaction) error {
	// Validate transaction
	if err := s.validate(tx); err != nil {
		return err
	}

	return s.repo.Create(tx)
}

func (s *transactionService) BulkCreate(transactions []domain.Transaction) error {
	// Validate all transactions
	for i, tx := range transactions {
		if err := s.validate(&tx); err != nil {
			logger.GetLogger().WithError(err).WithField("index", i).Warn("Invalid transaction, skipping")
			continue
		}
	}

	return s.repo.BulkCreate(transactions)
}

func (s *transactionService) GetByTrxID(trxID string) (*domain.Transaction, error) {
	if trxID == "" {
		return nil, fmt.Errorf("trxID cannot be empty")
	}
	return s.repo.GetByTrxID(trxID)
}

func (s *transactionService) GetByDateRange(startDate, endDate time.Time) ([]domain.Transaction, error) {
	if startDate.After(endDate) {
		return nil, fmt.Errorf("start date cannot be after end date")
	}
	return s.repo.GetByDateRange(startDate, endDate)
}

func (s *transactionService) validate(tx *domain.Transaction) error {
	if tx.TrxID == "" {
		return fmt.Errorf("transaction ID is required")
	}

	if tx.Amount.IsZero() {
		return fmt.Errorf("amount must be non-zero")
	}

	if tx.Amount.IsNegative() {
		return fmt.Errorf("amount must be positive")
	}

	if tx.Type != domain.Debit && tx.Type != domain.Credit {
		return fmt.Errorf("invalid transaction type: %s", tx.Type)
	}

	if tx.TransactionTime.IsZero() {
		return fmt.Errorf("transaction time is required")
	}

	return nil
}
