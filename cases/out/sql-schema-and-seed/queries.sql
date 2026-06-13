-- 1. All posts with their author's name
SELECT posts.id, posts.title, posts.body, posts.created_at, authors.name AS author_name
FROM posts
JOIN authors ON posts.author_id = authors.id;

-- 2. Count of posts per author
SELECT authors.name, COUNT(posts.id) AS post_count
FROM authors
LEFT JOIN posts ON authors.id = posts.author_id
GROUP BY authors.id, authors.name;

-- 3. Most recent post ordered by created_at
SELECT posts.id, posts.title, posts.body, posts.created_at, authors.name AS author_name
FROM posts
JOIN authors ON posts.author_id = authors.id
ORDER BY posts.created_at DESC
LIMIT 1;
