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

type ItemError struct {
	ID    int    `json:"id"`
	Error string `json:"error"`
}

type MovieResp struct {
	ID        int
	MovieInfo client.Movie
	Error     error
}

var ErrBadRange = errors.New("from must be positive and to must be >= from")

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

func (s *Service) LoadMovies(ctx context.Context, from, to int) ([]client.Movie, []ItemError, error) {
	if from < 1 || to < from || to-from > 10000 {
		return nil, nil, fmt.Errorf("%w: from=%d, to=%d", ErrBadRange, from, to)
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
		for i := from; i <= to; i++ {
			select {
			case <-ctx.Done():
				return
			case jobs <- i:
			}
		}
	}()

	movies := make([]client.Movie, 0, moviesNum)
	partial := make([]ItemError, 0, moviesNum)

	for res := range results {
		if res.Error != nil {
			partial = append(partial, ItemError{ID: res.ID, Error: res.Error.Error()})
		} else {
			movies = append(movies, res.MovieInfo)
		}
	}

	slices.SortFunc(movies, func(movie1, movie2 client.Movie) int { return cmp.Compare(movie1.ID, movie2.ID) })
	slices.SortFunc(partial, func(error1, error2 ItemError) int { return cmp.Compare(error1.ID, error2.ID) })

	if err := ctx.Err(); err != nil {
		return movies, partial, err
	}
	return movies, partial, nil
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
