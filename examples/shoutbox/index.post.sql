INSERT INTO
    shoutbox (name, comment, created_at)
VALUES
    (
        @name,
        @comment,
        CURRENT_TIMESTAMP
    );

INSERT INTO
    response_meta (name, value)
VALUES
    ("status", "302"),
    ("location", "/shoutbox");