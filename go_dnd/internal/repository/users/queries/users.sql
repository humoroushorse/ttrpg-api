-- name: GetUserByID :one
SELECT * FROM dnd.user
WHERE id = $1;

-- name: UpdateUser :one
INSERT INTO dnd.user (id, username, profile_picture_url)
VALUES ($1, $2, $3)
ON CONFLICT (id) DO UPDATE
SET
    username = EXCLUDED.username,
    profile_picture_url = EXCLUDED.profile_picture_url
RETURNING *;
