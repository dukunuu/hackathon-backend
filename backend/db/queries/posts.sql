-- name: GetUserPosts :many
SELECT * FROM posts WHERE user_id = $1 ORDER BY created_at DESC;

-- name: ListPosts :many
SELECT
    p.id,
    p.title,
    p.description,
    p.status,
    p.priority,
    p.preview_url,
    p.post_type,
    p.user_id,
    p.max_volunteers,
    p.current_volunteers,
    p.category_id,
    p.location_lat,
    p.location_lng,
    p.address_text,
    p.created_at,
    p.updated_at,
    (
        SELECT json_agg(pi.image_url)
        FROM post_images pi
        WHERE pi.post_id = p.id
    ) AS images,
    (
        SELECT json_agg(pv.*)
        FROM post_volunteers pv
        WHERE pv.post_id = p.id
    ) AS volunteers
FROM posts p
ORDER BY p.created_at DESC;

-- name: CreatePost :one
INSERT INTO posts (
    title,
    description,
    status,
    priority,
    preview_url,
    post_type,
    user_id,
    max_volunteers,
    current_volunteers,
    category_id,
    location_lat,
    location_lng,
    address_text
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
) RETURNING *;

-- name: GetPost :one
SELECT
    p.id,
    p.title,
    p.description,
    p.status,
    p.priority,
    p.preview_url,
    p.post_type,
    p.user_id,
    p.max_volunteers,
    p.current_volunteers,
    p.category_id,
    p.location_lat,
    p.location_lng,
    p.address_text,
    p.created_at,
    p.updated_at,
    (
        SELECT json_agg(pi.image_url)
        FROM post_images pi
        WHERE pi.post_id = p.id
    ) AS images,
    (
        SELECT json_agg(pv.*)
        FROM post_volunteers pv
        WHERE pv.post_id = p.id
    ) AS volunteers
FROM posts p
WHERE p.id = $1;

-- name: DeletePost :exec
DELETE FROM posts WHERE id = $1;

-- name: UpdatePost :one
UPDATE posts SET
    title = COALESCE($2, title),
    description = COALESCE($3, description),
    status = COALESCE($4, status),
    priority = COALESCE($5, priority),
    preview_url = COALESCE($6, preview_url),
    post_type = COALESCE($7, post_type),
    user_id = COALESCE($8, user_id),
    max_volunteers = COALESCE($9, max_volunteers),
    current_volunteers = COALESCE($10, current_volunteers),
    category_id = COALESCE($11, category_id),
    location_lat = COALESCE($12, location_lat),
    location_lng = COALESCE($13, location_lng),
    address_text = COALESCE($14, address_text),
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: ListPostVolunteers :many
SELECT * FROM post_volunteers ORDER BY created_at DESC;

-- name: DeletePostVolunteer :exec
DELETE FROM post_volunteers WHERE post_id = $1 AND user_id = $2;

-- name: GetUserStats :one
SELECT
    (SELECT COUNT(*) FROM posts p WHERE p.user_id = $1) AS user_posts_count,
    (SELECT COUNT(*) FROM post_volunteers p_v WHERE p_v.user_id = $1) AS user_volunteer_count,
    (SELECT COUNT(*) FROM posts po WHERE po.user_id = $1 AND status = 'Шийдвэрлэгдсэн') AS approved_posts;


-- name: ApproveVolunteer :one
UPDATE post_volunteers SET status = 'approved', updated_at = CURRENT_TIMESTAMP WHERE post_id = $1 AND user_id = $2 RETURNING *;

-- name: RejectVolunteer :one
UPDATE post_volunteers SET status = 'rejected', updated_at = CURRENT_TIMESTAMP WHERE post_id = $1 AND user_id = $2 RETURNING *;

-- name: GetCategoryName :one
SELECT name from categories WHERE id = $1;

-- name: GetCategories :many
SELECT * from categories;

-- name: CreateCategory :one
INSERT INTO categories(name, description, endpoint, can_volunteer)
VALUES ($1, $2, $3, $4) RETURNING *;
