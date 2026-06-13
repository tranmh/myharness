CREATE TABLE authors (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT
);

CREATE TABLE posts (
    id INTEGER PRIMARY KEY,
    author_id INTEGER,
    title TEXT NOT NULL,
    body TEXT,
    created_at TEXT,
    FOREIGN KEY (author_id) REFERENCES authors(id)
);
