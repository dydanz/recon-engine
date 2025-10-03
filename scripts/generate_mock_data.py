#!/usr/bin/env python3
"""
Generate mock transaction data for testing the reconciliation engine.

Creates:
1. PostgreSQL database with ~10,000 system transactions
2. 5 CSV files representing different banks with ~2,000 rows each
3. Introduces random errors and discrepancies for testing
"""

import csv
import random
import os
from datetime import datetime, timedelta
from decimal import Decimal
from dotenv import load_dotenv
import psycopg2
from psycopg2.extras import execute_batch

# Load environment variables
load_dotenv()

# Configuration
OUTPUT_DIR = "test/testdata"
NUM_TRANSACTIONS = 100
NUM_BANK_FILES = 1
ROWS_PER_BANK = NUM_TRANSACTIONS
ERROR_RATE = 0.05  # 5% of rows will have errors

# Bank names
BANKS = ["bank_bca"]

# Transaction types
TYPES = ["DEBIT", "CREDIT"]

# Database configuration from .env
DB_CONFIG = {
    'host': os.getenv('DB_HOST', 'localhost'),
    'port': os.getenv('DB_PORT', '5434'),
    'user': os.getenv('DB_USER', 'postgres'),
    'password': os.getenv('DB_PASSWORD', 'postgres'),
    'database': os.getenv('DB_NAME', 'recon_db')
}


def get_db_connection():
    """Create and return PostgreSQL database connection"""
    try:
        conn = psycopg2.connect(**DB_CONFIG)
        return conn
    except psycopg2.Error as e:
        print(f"Error connecting to PostgreSQL database: {e}")
        raise


def generate_transaction_id(index):
    """Generate unique transaction ID"""
    return f"TRX{index:08d}"


def generate_random_amount():
    """Generate random transaction amount between 10 and 10000"""
    return round(random.uniform(10.0, 10000.0), 2)


def generate_random_datetime(start_date, end_date):
    """Generate random datetime between start and end dates"""
    time_delta = end_date - start_date
    random_seconds = random.randint(0, int(time_delta.total_seconds()))
    return start_date + timedelta(seconds=random_seconds)


def create_postgres_transactions():
    """Create transactions in PostgreSQL database"""
    os.makedirs(OUTPUT_DIR, exist_ok=True)

    print(f"Connecting to PostgreSQL database at {DB_CONFIG['host']}:{DB_CONFIG['port']}...")

    try:
        conn = get_db_connection()
        cursor = conn.cursor()

        # Clear existing test data
        print(f"Clearing existing transactions...")
        cursor.execute("DELETE FROM transactions WHERE trx_id LIKE 'TRX%'")
        conn.commit()

        print(f"Creating {NUM_TRANSACTIONS} system transactions...")

        # Generate transactions
        start_date = datetime(2024, 1, 1)
        end_date = datetime(2024, 12, 31)

        transactions = []
        for i in range(1, NUM_TRANSACTIONS + 1):
            trx_id = generate_transaction_id(i)
            amount = generate_random_amount()
            tx_type = random.choice(TYPES)
            tx_time = generate_random_datetime(start_date, end_date)

            transactions.append((
                trx_id,
                amount,
                tx_type,
                tx_time
            ))

        # Bulk insert using execute_batch for better performance
        insert_query = """
            INSERT INTO transactions (trx_id, amount, type, transaction_time)
            VALUES (%s, %s, %s, %s)
            ON CONFLICT (trx_id) DO NOTHING
        """

        execute_batch(cursor, insert_query, transactions, page_size=1000)
        conn.commit()

        # Get actual count
        cursor.execute("SELECT COUNT(*) FROM transactions WHERE trx_id LIKE 'TRX%'")
        count = cursor.fetchone()[0]

        cursor.close()
        conn.close()

        print(f"✓ Connected to PostgreSQL database: {DB_CONFIG['database']}")
        print(f"✓ Inserted {count} transactions")

        return transactions

    except psycopg2.Error as e:
        print(f"✗ Database error: {e}")
        raise


def create_bank_csv_files(system_transactions):
    """Create CSV files for each bank with some matching and some mismatched data"""

    # Shuffle and distribute transactions across banks
    available_transactions = system_transactions.copy()
    random.shuffle(available_transactions)

    for bank_index, bank_name in enumerate(BANKS):
        csv_file = os.path.join(OUTPUT_DIR, f"{bank_name}.csv")

        print(f"\nCreating {bank_name}.csv with ~{ROWS_PER_BANK} rows...")

        with open(csv_file, 'w', newline='') as f:
            writer = csv.writer(f)
            writer.writerow(["trx_ref_id", "amount", "date"])

            rows_written = 0
            error_rows = 0
            matched_rows = 0
            unmatched_rows = 0
            discrepancy_rows = 0

            # Calculate how many transactions this bank should process
            start_idx = bank_index * (len(available_transactions) // NUM_BANK_FILES)
            end_idx = start_idx + ROWS_PER_BANK

            bank_transactions = available_transactions[start_idx:end_idx]

            for trx_id, amount, tx_type, tx_time in bank_transactions:
                if rows_written >= ROWS_PER_BANK:
                    break

                # Parse transaction time
                if isinstance(tx_time, str):
                    tx_datetime = datetime.fromisoformat(tx_time)
                else:
                    tx_datetime = tx_time
                date_str = tx_datetime.strftime("%Y-%m-%d")

                # Introduce random errors
                should_error = random.random() < ERROR_RATE

                if should_error:
                    error_type = random.choice([
                        "wrong_format_amount",
                        "wrong_format_date",
                        "missing_field",
                        "amount_discrepancy"
                    ])

                    if error_type == "wrong_format_amount":
                        # Invalid amount format
                        writer.writerow([trx_id, "invalid_amount", date_str])
                        error_rows += 1
                    elif error_type == "wrong_format_date":
                        # Invalid date format
                        writer.writerow([trx_id, amount, "2024-13-45"])  # Invalid date
                        error_rows += 1
                    elif error_type == "missing_field":
                        # Missing field
                        writer.writerow([trx_id, "", date_str])
                        error_rows += 1
                    elif error_type == "amount_discrepancy":
                        # Correct format but wrong amount
                        discrepancy_amount = amount + random.uniform(-100, 100)
                        bank_amount = -discrepancy_amount if tx_type == "DEBIT" else discrepancy_amount
                        writer.writerow([trx_id, f"{bank_amount:.2f}", date_str])
                        discrepancy_rows += 1
                else:
                    # Correct matching transaction
                    # Bank amounts: negative for debits, positive for credits
                    bank_amount = -amount if tx_type == "DEBIT" else amount
                    writer.writerow([trx_id, f"{bank_amount:.2f}", date_str])
                    matched_rows += 1

                rows_written += 1

            # Add some unmatched bank transactions (not in system)
            num_unmatched = int(ROWS_PER_BANK * 0.05)  # 5% unmatched
            for i in range(num_unmatched):
                fake_trx_id = f"BANK_{bank_name.upper()}_{i:06d}"
                fake_amount = generate_random_amount()
                fake_date = generate_random_datetime(
                    datetime(2024, 1, 1),
                    datetime(2024, 12, 31)
                ).strftime("%Y-%m-%d")

                writer.writerow([fake_trx_id, f"{fake_amount:.2f}", fake_date])
                unmatched_rows += 1
                rows_written += 1

        print(f"  ✓ {rows_written} total rows")
        print(f"  ✓ {matched_rows} matched transactions")
        print(f"  ✓ {discrepancy_rows} discrepancies")
        print(f"  ✓ {unmatched_rows} unmatched bank transactions")
        print(f"  ✓ {error_rows} error rows (wrong format)")


def main():
    print("=" * 60)
    print("Mock Data Generator for Transaction Reconciliation")
    print("=" * 60)
    print(f"\nDatabase Configuration:")
    print(f"  Host: {DB_CONFIG['host']}")
    print(f"  Port: {DB_CONFIG['port']}")
    print(f"  Database: {DB_CONFIG['database']}")
    print(f"  User: {DB_CONFIG['user']}")
    print("")

    try:
        # Create system transactions in PostgreSQL
        transactions = create_postgres_transactions()

        # Create bank CSV files
        create_bank_csv_files(transactions)

        print("\n" + "=" * 60)
        print("✓ Mock data generation completed successfully!")
        print("=" * 60)
        print(f"\nGenerated files in: {OUTPUT_DIR}/")
        for bank in BANKS:
            print(f"  - {bank}.csv")
        print(f"  - system_transactions_sample.csv")
        print(f"  - bank_sample.csv")
        print(f"\nDatabase: {DB_CONFIG['database']} (PostgreSQL)")
        print(f"  - {NUM_TRANSACTIONS} transactions in 'transactions' table")

    except Exception as e:
        print(f"\n✗ Error generating mock data: {e}")
        import traceback
        traceback.print_exc()
        exit(1)


if __name__ == "__main__":
    main()
