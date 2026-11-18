package service

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/AndyBarnv/golang-project-andybarnv/task3/internal/client"
)

type Fetcher interface {
	GetMovie(ctx context.Context, id int) (client.Movie, error)
}

type Service struct {
	fetcher    Fetcher
	workersNum int
}

func NewService(fetcher Fetcher, workersNum int) *Service {
	return &Service{fetcher: fetcher, workersNum: workersNum}
}

type MovieResp struct {
	ID        int
	MovieInfo client.Movie
	Error     error
}

const maxRangeSize = 10_000

var ErrBadRange = errors.New("invalid range")

func (s *Service) worker(ctx context.Context, jobs chan int, results chan MovieResp) {
	for id := range jobs {
		movie, err := s.fetcher.GetMovie(ctx, id)
		if err != nil {
			results <- MovieResp{ID: id, Error: err}
			continue
		}
		results <- MovieResp{ID: id, MovieInfo: movie}
	}
}

func (s *Service) LoadMovies(ctx context.Context, from, to int) ([]MovieResp, error) {
	if from < 1 || to < from || to-from >= maxRangeSize {
		return nil, fmt.Errorf(
			"%w: range must be positive, ordered and contain at most %d movies: from=%d, to=%d",
			ErrBadRange, maxRangeSize, from, to,
		)
	}

	moviesNum := to - from + 1
	jobs := make(chan int)
	results := make(chan MovieResp, moviesNum)

	var wg sync.WaitGroup

	for i := 0; i < s.workersNum; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.worker(ctx, jobs, results)
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	go func() {
		defer close(jobs)
		for id := from; ; id++ {
			select {
			case <-ctx.Done():
				return
			case jobs <- id:
			}
			if id == to {
				return
			}
		}
	}()

	items := make([]MovieResp, 0, moviesNum)
	for res := range results {
		items = append(items, res)
	}

	slices.SortFunc(items, func(item1, item2 MovieResp) int { return cmp.Compare(item1.ID, item2.ID) })

	if err := ctx.Err(); err != nil {
		return items, err
	}
	return items, nil
}

func (s *Service) LastMovie(ctx context.Context) (client.Movie, error) {
	exists := func(id int) (bool, error) {
		_, err := s.fetcher.GetMovie(ctx, id)
		switch {
		case err == nil:
			return true, nil
		case errors.Is(err, client.ErrNotFound):
			return false, nil
		default:
			return false, err
		}
	}

	low, high := 0, 1
	for {
		ok, err := exists(high)
		if err != nil {
			return client.Movie{}, err
		}
		if !ok {
			break
		}
		low, high = high, high*2
	}
	if low == 0 {
		return client.Movie{}, client.ErrNotFound
	}

	for high-low > 1 {
		mid := (high + low) / 2
		ok, err := exists(mid)
		switch {
		case err != nil:
			return client.Movie{}, err
		case !ok:
			high = mid
		default:
			low = mid
		}
	}

	return s.fetcher.GetMovie(ctx, low)
}
