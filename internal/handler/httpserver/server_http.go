package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/AndyBarnv/golang-project-andybarnv/internal/domain"
)

type Service interface {
	LoadMovies(ctx context.Context, from, to int) ([]domain.MovieItem, error)
	LastMovie(ctx context.Context) (domain.Movie, error)
}

type APIError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type movieAPI struct {
	svc Service
}

type movieResponse struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Year     int    `json:"year"`
	Director string `json:"director"`
}

type movieItem struct {
	ID       int    `json:"id"`
	Title    string `json:"title,omitempty"`
	Year     int    `json:"year,omitempty"`
	Director string `json:"director,omitempty"`
	Error    string `json:"error,omitempty"`
}

func NewHandler(svc Service) http.Handler {
	api := &movieAPI{svc: svc}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /load-movies", api.loadMovies)
	mux.HandleFunc("GET /last-movie", api.lastMovie)
	mux.HandleFunc("GET /openapi.json", serveSpec)
	mux.HandleFunc("GET /swagger/index.html", serveDocs)
	mux.HandleFunc("GET /swagger/{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/index.html", http.StatusFound)
	})
	return logging(mux)
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func (api *movieAPI) loadMovies(w http.ResponseWriter, r *http.Request) {
	from, to, ok := movieBounds(w, r)
	if !ok {
		return
	}
	items, err := api.svc.LoadMovies(r.Context(), from, to)
	if err != nil {
		if r.Context().Err() != nil {
			log.Printf("load-movies: request aborted: %v", err)
			return
		}
		switch {
		case errors.Is(err, domain.ErrBadRange):
			writeError(w, http.StatusBadRequest, "bad_range", err.Error())
		default:
			writeError(w, http.StatusServiceUnavailable, "database_unavailable", "failed to read movies from database")
		}
		return
	}

	out := make([]movieItem, 0, len(items))
	for _, it := range items {
		if it.Error != nil {
			out = append(out, movieItem{ID: it.ID, Error: it.Error.Error()})
		} else {
			out = append(out, movieItem{
				ID:       it.ID,
				Title:    it.Movie.Title,
				Year:     it.Movie.Year,
				Director: it.Movie.Director,
			})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (api *movieAPI) lastMovie(w http.ResponseWriter, r *http.Request) {
	movie, err := api.svc.LastMovie(r.Context())
	if err != nil {
		if r.Context().Err() != nil {
			log.Printf("last-movie: request aborted: %v", err)
			return
		}
		switch {
		case errors.Is(err, domain.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", err.Error())
		case errors.Is(err, context.DeadlineExceeded):
			writeError(w, http.StatusGatewayTimeout, "database_timeout", "database request timed out")
		default:
			writeError(w, http.StatusServiceUnavailable, "database_unavailable", "failed to read last movie from database")
		}
		return
	}
	out := movieResponse{
		ID:       movie.ID,
		Title:    movie.Title,
		Year:     movie.Year,
		Director: movie.Director,
	}
	writeJSON(w, http.StatusOK, out)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var response APIError
	response.Error.Code = code
	response.Error.Message = message
	writeJSON(w, status, response)
}

func movieBounds(w http.ResponseWriter, r *http.Request) (int, int, bool) {
	from, err := strconv.Atoi(r.URL.Query().Get("from"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_from", "from must be an integer")
		return 0, 0, false
	}
	to, err := strconv.Atoi(r.URL.Query().Get("to"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_to", "to must be an integer")
		return 0, 0, false
	}
	return from, to, true
}
