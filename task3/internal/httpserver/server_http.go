package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/AndyBarnv/golang-project-andybarnv/task3/internal/client"
	"github.com/AndyBarnv/golang-project-andybarnv/task3/internal/service"
)

type Service interface {
	LoadMovies(ctx context.Context, from, to int) ([]client.Movie, []service.ItemError, error)
	LastMovie(ctx context.Context) (client.Movie, error)
}

type movieAPI struct {
	svc Service
}

type loadMoviesResponse struct {
	Movies []client.Movie      `json:"movies"`
	Errors []service.ItemError `json:"errors"`
}

type APIError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
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

// loadMovies godoc
// @Summary      Загрузить список фильмов по диапазону ID
// @Description  Возвращает список фильмов и частичные ошибки загрузки
// @Tags         movies
// @Produce      json
// @Param        from  query     int  true  "Начальный ID"
// @Param        to    query     int  true  "Конечный ID"
// @Success      200   {object}  loadMoviesResponse
// @Failure      400   {object}  errorResponse
// @Failure      502   {object}  errorResponse
// @Router       /load-movies

func (api *movieAPI) loadMovies(w http.ResponseWriter, r *http.Request) {
	from, to, ok := movieBounds(w, r)
	if !ok {
		return
	}
	movies, partial, err := api.svc.LoadMovies(r.Context(), from, to)
	if err != nil {
		if r.Context().Err() != nil {
			log.Printf("load-movies: request aborted: %v", err)
			return
		}
		switch {
		case errors.Is(err, service.ErrBadRange):
			writeError(w, http.StatusBadRequest, "bad_range", err.Error())
		default:
			writeError(w, http.StatusBadGateway, "upstream_unavailable", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, loadMoviesResponse{Movies: movies, Errors: partial})
}

func (api *movieAPI) lastMovie(w http.ResponseWriter, r *http.Request) {
	movie, err := api.svc.LastMovie(r.Context())
	if err != nil {
		if r.Context().Err() != nil {
			log.Printf("last-movie: request aborted: %v", err)
			return
		}
		switch {
		case errors.Is(err, client.ErrNotFound):
			writeError(w, http.StatusNotFound, "not_found", err.Error())
		case errors.Is(err, context.DeadlineExceeded):
			writeError(w, http.StatusGatewayTimeout, "upstream_timeout", err.Error())
		default:
			writeError(w, http.StatusBadGateway, "upstream_unavailable", err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, movie)
}
