package lead

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const queryTimeout = 3 * time.Second

type Repository interface {
	Create(context.Context, Lead) (Lead, error)
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, lead Lead) (Lead, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	err := r.db.QueryRow(ctx, `
		INSERT INTO leads (name, email, company, message, category, priority, summary, recommended_action)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at`, lead.Name, lead.Email, lead.Company, lead.Message,
		lead.Classification.Category, lead.Classification.Priority, lead.Classification.Summary,
		lead.Classification.RecommendedAction).
		Scan(&lead.ID, &lead.CreatedAt)
	if err != nil {
		return Lead{}, fmt.Errorf("create lead: %w", err)
	}

	return lead, nil
}
