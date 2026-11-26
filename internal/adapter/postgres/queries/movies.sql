-- name: AddMovie :exec
INSERT INTO movies (id, title, year, director)
VALUES ($1, $2, $3, $4)  
ON CONFLICT (id) DO NOTHING;

-- name: ListMovies :many
SELECT id, title, year, director
FROM movies
WHERE id BETWEEN sqlc.arg(from_id) AND sqlc.arg(to_id)
ORDER BY id;

-- name: LastMovie :one
SELECT id, title, year, director 
FROM movies 
ORDER BY id DESC
LIMIT 1;

-- name: ListMovieIDs :many
SELECT id FROM movies;
