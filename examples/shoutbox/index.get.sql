CREATE TABLE IF NOT EXISTS shoutbox (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    name TEXT NOT NULL,
    comment TEXT NOT NULL
);

-- @wtf-store shoutbox_entries
SELECT
    name,
    comment,
    time_relative(created_at) as created_at
FROM
    shoutbox
ORDER BY
    created_at DESC;

-- @wtf-store csrf_token
-- @wtf-capture csrf single
SELECT
    secure_hex(64) as token;

INSERT INTO
    response_cookies (name, value, secure, http_only, same_site, path)
VALUES
    ("csrf_token", @csrf, 0, 1, 'Lax', '/');

INSERT INTO
    response_meta (name, value)
VALUES
    ("status", "200"),
    ("wtf-tpl", "shoutbox/shoutbox.tpl.html");