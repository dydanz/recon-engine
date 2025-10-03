package repository

import (
	"database/sql"
	"fmt"

	"recon-engine/internal/domain"
	"recon-engine/pkg/logger"
)

type ReconciliationRepository interface {
	CreateJob(job *domain.ReconciliationJob) error
	UpdateJob(job *domain.ReconciliationJob) error
	GetJobByID(jobID string) (*domain.ReconciliationJob, error)
	CreateResult(result *domain.ReconciliationResult) error
	BulkCreateResults(results []domain.ReconciliationResult) error
	GetResultsByJobID(jobID string) ([]domain.ReconciliationResult, error)
	GetResultsByJobIDAndStatus(jobID string, status domain.MatchStatus) ([]domain.ReconciliationResult, error)
}

type reconciliationRepository struct {
	db *sql.DB
}

func NewReconciliationRepository(db *sql.DB) ReconciliationRepository {
	return &reconciliationRepository{db: db}
}

func (r *reconciliationRepository) CreateJob(job *domain.ReconciliationJob) error {
	query := `
		INSERT INTO reconciliation_jobs (
			job_id, start_date, end_date, status,
			total_processed, total_matched, total_unmatched, total_discrepancies
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at
	`

	err := r.db.QueryRow(
		query,
		job.JobID,
		job.StartDate,
		job.EndDate,
		job.Status,
		job.TotalProcessed,
		job.TotalMatched,
		job.TotalUnmatched,
		job.TotalDiscrepancies,
	).Scan(&job.ID, &job.CreatedAt, &job.UpdatedAt)

	if err != nil {
		logger.GetLogger().WithError(err).Error("Failed to create reconciliation job")
		return err
	}

	return nil
}

func (r *reconciliationRepository) UpdateJob(job *domain.ReconciliationJob) error {
	query := `
		UPDATE reconciliation_jobs
		SET status = $1, total_processed = $2, total_matched = $3,
			total_unmatched = $4, total_discrepancies = $5, error_message = $6
		WHERE job_id = $7
	`

	_, err := r.db.Exec(
		query,
		job.Status,
		job.TotalProcessed,
		job.TotalMatched,
		job.TotalUnmatched,
		job.TotalDiscrepancies,
		job.ErrorMessage,
		job.JobID,
	)

	if err != nil {
		logger.GetLogger().WithError(err).Error("Failed to update reconciliation job")
		return err
	}

	return nil
}

func (r *reconciliationRepository) GetJobByID(jobID string) (*domain.ReconciliationJob, error) {
	query := `
		SELECT id, job_id, start_date, end_date, status,
			   total_processed, total_matched, total_unmatched, total_discrepancies,
			   error_message, created_at, updated_at
		FROM reconciliation_jobs
		WHERE job_id = $1
	`

	var job domain.ReconciliationJob
	err := r.db.QueryRow(query, jobID).Scan(
		&job.ID,
		&job.JobID,
		&job.StartDate,
		&job.EndDate,
		&job.Status,
		&job.TotalProcessed,
		&job.TotalMatched,
		&job.TotalUnmatched,
		&job.TotalDiscrepancies,
		&job.ErrorMessage,
		&job.CreatedAt,
		&job.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("reconciliation job not found")
	}
	if err != nil {
		logger.GetLogger().WithError(err).Error("Failed to get reconciliation job")
		return nil, err
	}

	return &job, nil
}

func (r *reconciliationRepository) CreateResult(result *domain.ReconciliationResult) error {
	query := `
		INSERT INTO reconciliation_results (
			job_id, trx_id, trx_ref_id, system_amount, bank_amount,
			discrepancy, match_status, bank_source, transaction_date
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at
	`

	err := r.db.QueryRow(
		query,
		result.JobID,
		result.TrxID,
		result.TrxRefID,
		result.SystemAmount,
		result.BankAmount,
		result.Discrepancy,
		result.MatchStatus,
		result.BankSource,
		result.TransactionDate,
	).Scan(&result.ID, &result.CreatedAt)

	if err != nil {
		logger.GetLogger().WithError(err).Error("Failed to create reconciliation result")
		return err
	}

	return nil
}

func (r *reconciliationRepository) BulkCreateResults(results []domain.ReconciliationResult) error {
	if len(results) == 0 {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		logger.GetLogger().WithError(err).Error("Failed to begin transaction")
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO reconciliation_results (
			job_id, trx_id, trx_ref_id, system_amount, bank_amount,
			discrepancy, match_status, bank_source, transaction_date
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`)
	if err != nil {
		logger.GetLogger().WithError(err).Error("Failed to prepare statement")
		return err
	}
	defer stmt.Close()

	for _, result := range results {
		_, err = stmt.Exec(
			result.JobID,
			result.TrxID,
			result.TrxRefID,
			result.SystemAmount,
			result.BankAmount,
			result.Discrepancy,
			result.MatchStatus,
			result.BankSource,
			result.TransactionDate,
		)
		if err != nil {
			logger.GetLogger().WithError(err).Error("Failed to insert reconciliation result")
			continue
		}
	}

	if err := tx.Commit(); err != nil {
		logger.GetLogger().WithError(err).Error("Failed to commit transaction")
		return err
	}

	return nil
}

func (r *reconciliationRepository) GetResultsByJobID(jobID string) ([]domain.ReconciliationResult, error) {
	query := `
		SELECT id, job_id, trx_id, trx_ref_id, system_amount, bank_amount,
			   discrepancy, match_status, bank_source, transaction_date, created_at
		FROM reconciliation_results
		WHERE job_id = $1
		ORDER BY created_at
	`

	rows, err := r.db.Query(query, jobID)
	if err != nil {
		logger.GetLogger().WithError(err).Error("Failed to query reconciliation results")
		return nil, err
	}
	defer rows.Close()

	var results []domain.ReconciliationResult
	for rows.Next() {
		var result domain.ReconciliationResult
		err := rows.Scan(
			&result.ID,
			&result.JobID,
			&result.TrxID,
			&result.TrxRefID,
			&result.SystemAmount,
			&result.BankAmount,
			&result.Discrepancy,
			&result.MatchStatus,
			&result.BankSource,
			&result.TransactionDate,
			&result.CreatedAt,
		)
		if err != nil {
			logger.GetLogger().WithError(err).Error("Failed to scan reconciliation result")
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

func (r *reconciliationRepository) GetResultsByJobIDAndStatus(jobID string, status domain.MatchStatus) ([]domain.ReconciliationResult, error) {
	query := `
		SELECT id, job_id, trx_id, trx_ref_id, system_amount, bank_amount,
			   discrepancy, match_status, bank_source, transaction_date, created_at
		FROM reconciliation_results
		WHERE job_id = $1 AND match_status = $2
		ORDER BY created_at
	`

	rows, err := r.db.Query(query, jobID, status)
	if err != nil {
		logger.GetLogger().WithError(err).Error("Failed to query reconciliation results")
		return nil, err
	}
	defer rows.Close()

	var results []domain.ReconciliationResult
	for rows.Next() {
		var result domain.ReconciliationResult
		err := rows.Scan(
			&result.ID,
			&result.JobID,
			&result.TrxID,
			&result.TrxRefID,
			&result.SystemAmount,
			&result.BankAmount,
			&result.Discrepancy,
			&result.MatchStatus,
			&result.BankSource,
			&result.TransactionDate,
			&result.CreatedAt,
		)
		if err != nil {
			logger.GetLogger().WithError(err).Error("Failed to scan reconciliation result")
			continue
		}
		results = append(results, result)
	}

	return results, nil
}
