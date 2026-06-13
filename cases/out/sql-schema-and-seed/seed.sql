INSERT INTO authors (id, name, email) VALUES
    (1, 'Alice Johnson', 'alice@example.com'),
    (2, 'Bob Smith', 'bob@example.com');

INSERT INTO posts (id, author_id, title, body, created_at) VALUES
    (1, 1, 'Getting Started with SQL', 'SQL is a powerful language for managing relational databases.', '2024-01-15'),
    (2, 1, 'Advanced Query Techniques', 'Joins, subqueries, and window functions unlock the full power of SQL.', '2024-02-20'),
    (3, 2, 'Database Design Principles', 'Normalization and proper indexing are key to a well-designed schema.', '2024-03-10');
