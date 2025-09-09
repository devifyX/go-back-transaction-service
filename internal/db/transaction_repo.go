package db

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/devifyX/go-back-transaction-service/internal/models"
)

type TransactionFilter struct {
	ID            *string
	UserID        *string
	CoinID        *string
	DataID        *string
	PlatformName  *string
	FromTimestamp *time.Time // inclusive
	ToTimestamp   *time.Time // inclusive
	Limit         int
	Offset        int
}

type TransactionRepo struct {
	pool *Pool
}

func NewTransactionRepo(pool *Pool) *TransactionRepo {
	return &TransactionRepo{pool: pool}
}

func (r *TransactionRepo) Insert(ctx context.Context, t models.Transaction) (*models.Transaction, error) {
	const q = `
		INSERT INTO transactions (
			coinid, userid, dataid, coinused, transactionTimestamp, expiryDate, platformName
		) VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, coinid, userid, dataid, coinused, transactionTimestamp, expiryDate, platformName
	`
	row := r.pool.QueryRow(ctx, q,
		t.CoinID,
		t.UserID,
		t.DataID,
		t.CoinUsed,
		t.TransactionTimestamp,
		t.ExpiryDate,
		t.PlatformName,
	)
	var out models.Transaction
	if err := row.Scan(
		&out.ID, &out.CoinID, &out.UserID, &out.DataID, &out.CoinUsed, &out.TransactionTimestamp, &out.ExpiryDate, &out.PlatformName,
	); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *TransactionRepo) GetByID(ctx context.Context, id string) (*models.Transaction, error) {
	const q = `
		SELECT id, coinid, userid, dataid, coinused, transactionTimestamp, expiryDate, platformName
		FROM transactions
		WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, q, id)
	var out models.Transaction
	if err := row.Scan(
		&out.ID, &out.CoinID, &out.UserID, &out.DataID, &out.CoinUsed, &out.TransactionTimestamp, &out.ExpiryDate, &out.PlatformName,
	); err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *TransactionRepo) List(ctx context.Context, f TransactionFilter) ([]models.Transaction, error) {
	sb := strings.Builder{}
	sb.WriteString("SELECT id, coinid, userid, dataid, coinused, transactionTimestamp, expiryDate, platformName FROM transactions WHERE 1=1")
	args := []any{}
	idx := 1

	add := func(clause string, val any) {
		sb.WriteString(" AND ")
		sb.WriteString(clause)
		args = append(args, val)
		idx++
	}

	if f.ID != nil && *f.ID != "" {
		add(fmt.Sprintf("id = $%d", idx), *f.ID)
	}
	if f.UserID != nil && *f.UserID != "" {
		add(fmt.Sprintf("userid = $%d", idx), *f.UserID)
	}
	if f.CoinID != nil && *f.CoinID != "" {
		add(fmt.Sprintf("coinid = $%d", idx), *f.CoinID)
	}
	if f.DataID != nil && *f.DataID != "" {
		add(fmt.Sprintf("dataid = $%d", idx), *f.DataID)
	}
	if f.PlatformName != nil && *f.PlatformName != "" {
		add(fmt.Sprintf("platformName = $%d", idx), *f.PlatformName)
	}
	if f.FromTimestamp != nil {
		add(fmt.Sprintf("transactionTimestamp >= $%d", idx), *f.FromTimestamp)
	}
	if f.ToTimestamp != nil {
		add(fmt.Sprintf("transactionTimestamp <= $%d", idx), *f.ToTimestamp)
	}

	sb.WriteString(" ORDER BY transactionTimestamp DESC")
	limit := 100
	if f.Limit > 0 && f.Limit <= 1000 {
		limit = f.Limit
	}
	sb.WriteString(fmt.Sprintf(" LIMIT %d", limit))
	if f.Offset > 0 {
		sb.WriteString(fmt.Sprintf(" OFFSET %d", f.Offset))
	}

	rows, err := r.pool.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Transaction
	for rows.Next() {
		var t models.Transaction
		if err := rows.Scan(&t.ID, &t.CoinID, &t.UserID, &t.DataID, &t.CoinUsed, &t.TransactionTimestamp, &t.ExpiryDate, &t.PlatformName); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
