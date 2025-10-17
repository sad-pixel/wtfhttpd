CREATE TABLE IF NOT EXISTS shoutbox (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    name TEXT NOT NULL,
    comment TEXT NOT NULL
);

INSERT INTO
    response_meta (name, value)
VALUES
    ("status", "200"),
    ("wtf-tpl", "shoutbox/shoutbox.html");

SELECT
    *
FROM
    shoutbox
ORDER BY
    created_at DESC;