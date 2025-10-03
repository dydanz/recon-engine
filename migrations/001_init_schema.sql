-- Create transactions table
CREATE TABLE IF NOT EXISTS transactions (
    id SERIAL PRIMARY KEY,
    trx_id VARCHAR(255) UNIQUE NOT NULL,
    amount DECIMAL(20, 2) NOT NULL,
    type VARCHAR(10) NOT NULL CHECK (type IN ('DEBIT', 'CREDIT')),
    transaction_time TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create reconciliation_jobs table
CREATE TABLE IF NOT EXISTS reconciliation_jobs (
    id SERIAL PRIMARY KEY,
    job_id UUID UNIQUE NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('PENDING', 'PROCESSING', 'COMPLETED', 'FAILED')),
    total_processed INT DEFAULT 0,
    total_matched INT DEFAULT 0,
    total_unmatched INT DEFAULT 0,
    total_discrepancies DECIMAL(20, 2) DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create reconciliation_results table
CREATE TABLE IF NOT EXISTS reconciliation_results (
    id SERIAL PRIMARY KEY,
    job_id UUID NOT NULL REFERENCES reconciliation_jobs(job_id) ON DELETE CASCADE,
    trx_id VARCHAR(255),
    trx_ref_id VARCHAR(255),
    system_amount DECIMAL(20, 2),
    bank_amount DECIMAL(20, 2),
    discrepancy DECIMAL(20, 2),
    match_status VARCHAR(20) NOT NULL CHECK (match_status IN ('MATCHED', 'UNMATCHED_SYSTEM', 'UNMATCHED_BANK', 'DISCREPANCY')),
    bank_source VARCHAR(255),
    transaction_date TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for better query performance
CREATE INDEX idx_transactions_trx_id ON transactions(trx_id);
CREATE INDEX idx_transactions_time ON transactions(transaction_time);
CREATE INDEX idx_transactions_time_type ON transactions(transaction_time, type);
CREATE INDEX idx_reconciliation_jobs_job_id ON reconciliation_jobs(job_id);
CREATE INDEX idx_reconciliation_jobs_status ON reconciliation_jobs(status);
CREATE INDEX idx_reconciliation_results_job_id ON reconciliation_results(job_id);
CREATE INDEX idx_reconciliation_results_match_status ON reconciliation_results(match_status);

-- Create function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Create triggers for updated_at
CREATE TRIGGER update_transactions_updated_at BEFORE UPDATE ON transactions
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_reconciliation_jobs_updated_at BEFORE UPDATE ON reconciliation_jobs
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
