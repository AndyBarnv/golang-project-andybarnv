package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const movieURL = "%s/%d/info.0.json"

var ErrNotFound = errors.New("movie not found")

type Movie struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Year     int    `json:"year"`
	Director string `json:"director"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
}

func (cl *Client) GetMovie(ctx context.Context, id int) (Movie, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, cl.timeout)
	defer cancel()

	url := fmt.Sprintf(movieURL, cl.baseURL, id)
	req, err := http.NewRequestWithContext(ctxTimeout, http.MethodGet, url, nil)
	if err != nil {
		return Movie{}, fmt.Errorf("new request: %w", err)
	}

	res, err := cl.httpClient.Do(req)
	if err != nil {
		return Movie{}, fmt.Errorf("do request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return Movie{}, ErrNotFound
	}
	if res.StatusCode != http.StatusOK {
		return Movie{}, fmt.Errorf("unexpected upstream status: %s", res.Status)
	}

	var movie Movie
	err = json.NewDecoder(res.Body).Decode(&movie)
	if err != nil {
		return Movie{}, fmt.Errorf("decode json: %w", err)
	}

	return movie, nil
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: http.DefaultClient,
		timeout:    timeout,
	}
}
