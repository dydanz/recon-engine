package repository

import (
	"database/sql"
	"fmt"
	"time"

	"recon-engine/internal/domain"
	"recon-engine/pkg/logger"
)

type TransactionRepository interface {
	Create(tx *domain.Transaction) error
	BulkCreate(transactions []domain.Transaction) error
	GetByTrxID(trxID string) (*domain.Transaction, error)
	GetByDateRange(startDate, endDate time.Time) ([]domain.Transaction, error)
	GetByDateRangeStream(startDate, endDate time.Time, batchSize int, callback func([]domain.Transaction) error) error
}

type transactionRepository struct {
	db *sql.DB
}

func NewTransactionRepository(db *sql.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

func (r *transactionRepository) Create(tx *domain.Transaction) error {
	query := `
		INSERT INTO transactions (trx_id, amount, type, transaction_time)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRow(
		query,
		tx.TrxID,
		tx.Amount,
		tx.Type,
		tx.TransactionTime,
	).Scan(&tx.ID, &tx.CreatedAt, &tx.UpdatedAt)

	if err != nil {
		logger.GetLogger().WithError(err).Error("Failed to create transaction")
		return err
	}

	return nil
}

func (r *transactionRepository) BulkCreate(transactions []domain.Transaction) error {
	if len(transactions) == 0 {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		logger.GetLogger().WithError(err).Error("Failed to begin transaction")
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO transactions (trx_id, amount, type, transaction_time)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (trx_id) DO NOTHING
	`)
	if err != nil {
		logger.GetLogger().WithError(err).Error("Failed to prepare statement")
		return err
	}
	defer stmt.Close()

	for _, transaction := range transactions {
		_, err = stmt.Exec(
			transaction.TrxID,
			transaction.Amount,
			transaction.Type,
			transaction.TransactionTime,
		)
		if err != nil {
			logger.GetLogger().WithError(err).WithField("trx_id", transaction.TrxID).Error("Failed to insert transaction")
			continue // Continue with next transaction instead of breaking
		}
	}

	if err := tx.Commit(); err != nil {
		logger.GetLogger().WithError(err).Error("Failed to commit transaction")
		return err
	}

	return nil
}

func (r *transactionRepository) GetByTrxID(trxID string) (*domain.Transaction, error) {
	query := `
		SELECT id, trx_id, amount, type, transaction_time, created_at, updated_at
		FROM transactions
		WHERE trx_id = $1
	`

	var tx domain.Transaction
	err := r.db.QueryRow(query, trxID).Scan(
		&tx.ID,
		&tx.TrxID,
		&tx.Amount,
		&tx.Type,
		&tx.TransactionTime,
		&tx.CreatedAt,
		&tx.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("transaction not found")
	}
	if err != nil {
		logger.GetLogger().WithError(err).Error("Failed to get transaction")
		return nil, err
	}

	return &tx, nil
}

func (r *transactionRepository) GetByDateRange(startDate, endDate time.Time) ([]domain.Transaction, error) {
	query := `
		SELECT id, trx_id, amount, type, transaction_time, created_at, updated_at
		FROM transactions
		WHERE transaction_time >= $1 AND transaction_time <= $2
		ORDER BY transaction_time
	`

	rows, err := r.db.Query(query, startDate, endDate)
	if err != nil {
		logger.GetLogger().WithError(err).Error("Failed to query transactions")
		return nil, err
	}
	defer rows.Close()

	var transactions []domain.Transaction
	for rows.Next() {
		var tx domain.Transaction
		err := rows.Scan(
			&tx.ID,
			&tx.TrxID,
			&tx.Amount,
			&tx.Type,
			&tx.TransactionTime,
			&tx.CreatedAt,
			&tx.UpdatedAt,
		)
		if err != nil {
			logger.GetLogger().WithError(err).Error("Failed to scan transaction")
			continue
		}
		transactions = append(transactions, tx)
	}

	return transactions, nil
}

// GetByDateRangeStream processes transactions in batches to avoid loading all into memory
func (r *transactionRepository) GetByDateRangeStream(startDate, endDate time.Time, batchSize int, callback func([]domain.Transaction) error) error {
	query := `
		SELECT id, trx_id, amount, type, transaction_time, created_at, updated_at
		FROM transactions
		WHERE transaction_time >= $1 AND transaction_time <= $2
		ORDER BY transaction_time
	`

	rows, err := r.db.Query(query, startDate, endDate)
	if err != nil {
		logger.GetLogger().WithError(err).Error("Failed to query transactions")
		return err
	}
	defer rows.Close()

	batch := make([]domain.Transaction, 0, batchSize)
	for rows.Next() {
		var tx domain.Transaction
		err := rows.Scan(
			&tx.ID,
			&tx.TrxID,
			&tx.Amount,
			&tx.Type,
			&tx.TransactionTime,
			&tx.CreatedAt,
			&tx.UpdatedAt,
		)
		if err != nil {
			logger.GetLogger().WithError(err).Error("Failed to scan transaction")
			continue
		}

		batch = append(batch, tx)

		if len(batch) >= batchSize {
			if err := callback(batch); err != nil {
				return err
			}
			batch = make([]domain.Transaction, 0, batchSize)
		}
	}

	// Process remaining items
	if len(batch) > 0 {
		if err := callback(batch); err != nil {
			return err
		}
	}

	return rows.Err()
}
