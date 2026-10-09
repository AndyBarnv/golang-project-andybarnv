package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const movieUrl = "https://homeworksite.site/%d/info.0.json"

type Movie struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Year     int    `json:"year"`
	Director string `json:"director"`
}

type MovieResp struct {
	ID        int
	MovieInfo Movie
	Err       error
}

func Worker(ctx context.Context, jobs <-chan int, results chan<- MovieResp, timeout int) {
	client := http.DefaultClient

	for id := range jobs {
		func(id int) {
			movieResp := MovieResp{ID: id}
			defer func() { results <- movieResp }()

			if err := ctx.Err(); err != nil {
				movieResp.Err = err
				return
			}

			url := fmt.Sprintf(movieUrl, id)

			ctxTimeout, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctxTimeout, http.MethodGet, url, nil)
			if err != nil {
				movieResp.Err = fmt.Errorf("Ошибка инициализации запроса: %w", err)
				return
			}

			res, err := client.Do(req)
			if err != nil {
				movieResp.Err = fmt.Errorf("Ошибка выполнения запроса: %w", err)
				return
			}
			defer res.Body.Close()

			if res.StatusCode != http.StatusOK {
				movieResp.Err = fmt.Errorf("Сервер вернул ошибку, код возврата: %d", res.StatusCode)
				return
			}

			var movie Movie
			err = json.NewDecoder(res.Body).Decode(&movie)
			if err != nil {
				movieResp.Err = fmt.Errorf("Ошибка декодирования JSON: %w", err)
				return
			}

			movieResp.MovieInfo = movie
		}(id)
	}
}

func main() {
	var from, to, workers, timeout int

	flag.IntVar(&from, "from", -1, "id первого фильма, обязательный флаг")
	flag.IntVar(&to, "to", -1, "id последнего фильма, обязательный флаг")
	flag.IntVar(&workers, "workers", 10, "количество воркеров в worker pool, по умолчанию 10")
	flag.IntVar(&timeout, "timeout", 5, "таймаут одного HTTP-запроса, по умолчанию 5 секунд")

	flag.Parse()
	if from == -1 || to == -1 {
		fmt.Fprintln(os.Stderr, "Ошибка: не все обязательные флаги указаны")
		flag.Usage()
		os.Exit(1)
	} else if from > to || from < 0 || to < 0 || workers <= 0 || timeout <= 0 {
		fmt.Fprintln(os.Stderr, "Ошибка: неккоректное значение флагов")
		flag.Usage()
		os.Exit(1)
	}

	ctxSig, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup

	jobs := make(chan int, workers)
	results := make(chan MovieResp)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			Worker(ctxSig, jobs, results, timeout)
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	go func() {
		defer close(jobs)
		for j := from; j <= to; j++ {
			select {
			case <-ctxSig.Done():
				return
			case jobs <- j:
			}
		}
	}()

	for res := range results {
		if err := res.Err; err != nil {
			if errors.Is(err, context.Canceled) {
				continue
			}
			fmt.Fprintln(os.Stderr, res.Err)
			continue
		}
		fmt.Printf("%d - %s - %d - %s\n", res.MovieInfo.ID, res.MovieInfo.Title, res.MovieInfo.Year, res.MovieInfo.Director)
	}
}
