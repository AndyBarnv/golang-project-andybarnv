package postgres

import (
	"context"
	"errors"
	"fmt"

	db "github.com/AndyBarnv/golang-project-andybarnv/internal/adapter/postgres/sqlc"
	"github.com/AndyBarnv/golang-project-andybarnv/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	queries *db.Queries
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{queries: db.New(pool)}
}

func (r *Repository) SaveMovie(ctx context.Context, movie domain.Movie) error {
	err := r.queries.AddMovie(ctx, db.AddMovieParams{
		ID:       int64(movie.ID),
		Title:    movie.Title,
		Year:     int32(movie.Year),
		Director: movie.Director,
	})
	if err != nil {
		return fmt.Errorf("save movie %d: %w", movie.ID, err)
	}
	return nil
}

func (r *Repository) ListMovies(ctx context.Context, from, to int) ([]domain.Movie, error) {
	movies, err := r.queries.ListMovies(ctx, db.ListMoviesParams{
		FromID: int64(from),
		ToID:   int64(to),
	})
	if err != nil {
		return nil, fmt.Errorf("list movies %d..%d: %w", from, to, err)
	}
	return toDomainMovies(movies), nil
}

func (r *Repository) LastMovie(ctx context.Context) (domain.Movie, error) {
	movie, err := r.queries.LastMovie(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Movie{}, domain.ErrNotFound
		}
		return domain.Movie{}, fmt.Errorf("get last movie: %w", err)
	}
	return toDomainMovie(movie), nil
}

func (r *Repository) ExistingIDs(ctx context.Context) ([]int, error) {
	ids, err := r.queries.ListMovieIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list movie ids: %w", err)
	}
	res := make([]int, len(ids))
	for i, id := range ids {
		res[i] = int(id)
	}
	return res, nil
}

func toDomainMovies(movies []db.Movie) []domain.Movie {
	domainMovies := make([]domain.Movie, len(movies))
	for i, movie := range movies {
		domainMovies[i] = toDomainMovie(movie)
	}
	return domainMovies
}

func toDomainMovie(movie db.Movie) domain.Movie {
	return domain.Movie{
		ID:       int(movie.ID),
		Title:    movie.Title,
		Year:     int(movie.Year),
		Director: movie.Director,
	}
}
