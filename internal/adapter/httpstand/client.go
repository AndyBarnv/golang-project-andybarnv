package httpstand

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/AndyBarnv/golang-project-andybarnv/internal/domain"
)

const movieURL = "%s/%d/info.0.json"

type Client struct {
	baseURL    string
	httpClient *http.Client
	timeout    time.Duration
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: http.DefaultClient,
		timeout:    timeout,
	}
}

func (cl *Client) GetMovie(ctx context.Context, id int) (domain.Movie, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, cl.timeout)
	defer cancel()

	url := fmt.Sprintf(movieURL, cl.baseURL, id)
	req, err := http.NewRequestWithContext(ctxTimeout, http.MethodGet, url, nil)
	if err != nil {
		return domain.Movie{}, fmt.Errorf("new request: %w", err)
	}

	res, err := cl.httpClient.Do(req)
	if err != nil {
		return domain.Movie{}, fmt.Errorf("do request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return domain.Movie{}, domain.ErrNotFound
	}
	if res.StatusCode != http.StatusOK {
		return domain.Movie{}, fmt.Errorf("unexpected upstream status: %s", res.Status)
	}

	var movie movieDTO
	if err := json.NewDecoder(res.Body).Decode(&movie); err != nil {
		return domain.Movie{}, fmt.Errorf("decode json: %w", err)
	}

	return movie.toDomain(), nil
}

type movieDTO struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Year     int    `json:"year"`
	Director string `json:"director"`
}

func (dto movieDTO) toDomain() domain.Movie {
	return domain.Movie{
		ID:       dto.ID,
		Title:    dto.Title,
		Year:     dto.Year,
		Director: dto.Director,
	}
}
