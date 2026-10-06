package lead

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const queryTimeout = 3 * time.Second

type Lead struct {
	ID        int64
	Name      string
	Email     string
	Company   string
	Message   string
	CreatedAt time.Time
}

type CreateParams struct {
	Name    string
	Email   string
	Company string
	Message string
}

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, params CreateParams) (Lead, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	lead := Lead{
		Name:    params.Name,
		Email:   params.Email,
		Company: params.Company,
		Message: params.Message,
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO leads (name, email, company, message)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`, params.Name, params.Email, params.Company, params.Message).
		Scan(&lead.ID, &lead.CreatedAt)
	if err != nil {
		return Lead{}, fmt.Errorf("create lead: %w", err)
	}

	return lead, nil
}
