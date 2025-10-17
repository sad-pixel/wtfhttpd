INSERT INTO
    shoutbox (name, comment, created_at)
VALUES
    (
        (
            SELECT
                value
            FROM
                request_form
            WHERE
                name = 'name'
        ),
        (
            SELECT
                value
            FROM
                request_form
            WHERE
                name = 'comment'
        ),
        CURRENT_TIMESTAMP
    );

INSERT INTO
    response_meta (name, value)
VALUES
    ("status", "302"),
    ("location", "/shoutbox");