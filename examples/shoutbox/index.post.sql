-- @wtf-validate name required,max=64
-- @wtf-validate comment required,max=512
-- @wtf-validate csrf_token required,len=64
-- Check if CSRF token is invalid
SELECT
    wtf_abort(403, 'Invalid CSRF token')
WHERE
    (
        SELECT
            value
        FROM
            request_headers
        WHERE
            name = 'Cookie'
            AND value LIKE '%csrf_token=%'
    ) NOT LIKE '%csrf_token=' || @csrf_token || '%';

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