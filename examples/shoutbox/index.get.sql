CREATE TABLE IF NOT EXISTS shoutbox (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    name TEXT NOT NULL,
    comment TEXT NOT NULL
);

-- @wtf-store shoutbox_entries
SELECT
    *
FROM
    shoutbox
ORDER BY
    created_at DESC;

INSERT INTO
    response_meta (name, value)
VALUES
    ("secure_token", secure_hex(64));

-- @wtf-store csrf_token
SELECT
    value
FROM
    response_meta
WHERE
    name = "secure_token";

INSERT INTO
    response_meta (name, value)
VALUES
    ("status", "200"),
    ("wtf-tpl", "shoutbox/shoutbox.html"),
    (
        "Set-Cookie",
        "csrf_token=" || (
            SELECT
                value
            FROM
                response_meta
            WHERE
                name = "secure_token"
        )
    );