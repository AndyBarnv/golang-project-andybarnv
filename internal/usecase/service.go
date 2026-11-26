package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/AndyBarnv/golang-project-andybarnv/internal/domain"
	"golang.org/x/sync/errgroup"
)

const maxRangeSize = 10_000

type Fetcher interface {
	GetMovie(ctx context.Context, id int) (domain.Movie, error)
}

type Repository interface {
	SaveMovie(ctx context.Context, movie domain.Movie) error
	ListMovies(ctx context.Context, from, to int) ([]domain.Movie, error)
	LastMovie(ctx context.Context) (domain.Movie, error)
	ExistingIDs(ctx context.Context) ([]int, error)
}

type Service struct {
	fetcher    Fetcher
	repository Repository
	workersNum int
}

func NewService(fetcher Fetcher, repository Repository, workersNum int) (*Service, error) {
	if workersNum <= 0 || workersNum > 100 {
		return nil, fmt.Errorf("workersNum must be positive integer at most 100: %d", workersNum)
	}
	return &Service{
		fetcher:    fetcher,
		repository: repository,
		workersNum: workersNum,
	}, nil
}

func (s *Service) LoadMovies(ctx context.Context, from, to int) ([]domain.MovieItem, error) {
	if from < 1 || to < from || to-from >= maxRangeSize {
		return nil, fmt.Errorf(
			"%w: range must be positive, ordered and contain at most %d movies: from=%d, to=%d",
			domain.ErrBadRange, maxRangeSize, from, to,
		)
	}

	movies, err := s.repository.ListMovies(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("load movies %d..%d: %w", from, to, err)
	}

	moviesNum := to - from + 1
	items := make([]domain.MovieItem, 0, moviesNum)
	movieIdx := 0

	for id := from; id <= to; id++ {
		if movieIdx < len(movies) && movies[movieIdx].ID == id {
			items = append(items, domain.MovieItem{
				ID:    id,
				Movie: movies[movieIdx],
			})
			movieIdx++
			continue
		}

		items = append(items, domain.MovieItem{
			ID:    id,
			Error: domain.ErrNotStored,
		})
	}

	return items, nil
}

func (s *Service) LastMovie(ctx context.Context) (domain.Movie, error) {
	movie, err := s.repository.LastMovie(ctx)
	if err != nil {
		return domain.Movie{}, fmt.Errorf("get last movie: %w", err)
	}
	return movie, nil
}

func (s *Service) SyncDatabase(ctx context.Context) error {
	lastMovie, err := s.StandLastMovie(ctx)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get last movie to sync: %w", err)
	}
	ids, err := s.repository.ExistingIDs(ctx)
	if err != nil {
		return fmt.Errorf("get existing movie ids: %w", err)
	}

	existing := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		existing[id] = struct{}{}
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(s.workersNum)

	for id := 1; id <= lastMovie.ID; id++ {
		if _, ok := existing[id]; ok {
			continue
		}

		eg.Go(func() error {
			movie, err := s.fetcher.GetMovie(ctx, id)
			if err != nil {
				return fmt.Errorf("fetch movie %d: %w", id, err)
			}
			err = s.repository.SaveMovie(ctx, movie)
			if err != nil {
				return fmt.Errorf("save movie %d: %w", id, err)
			}
			return nil
		})
	}

	return eg.Wait()
}

func (s *Service) StandLastMovie(ctx context.Context) (domain.Movie, error) {
	exists := func(id int) (bool, error) {
		_, err := s.fetcher.GetMovie(ctx, id)
		switch {
		case err == nil:
			return true, nil
		case errors.Is(err, domain.ErrNotFound):
			return false, nil
		default:
			return false, err
		}
	}

	low, high := 0, 1
	for {
		ok, err := exists(high)
		if err != nil {
			return domain.Movie{}, err
		}
		if !ok {
			break
		}
		low, high = high, high*2
	}
	if low == 0 {
		return domain.Movie{}, domain.ErrNotFound
	}

	for high-low > 1 {
		mid := (high + low) / 2
		ok, err := exists(mid)
		switch {
		case err != nil:
			return domain.Movie{}, err
		case !ok:
			high = mid
		default:
			low = mid
		}
	}

	return s.fetcher.GetMovie(ctx, low)
}
