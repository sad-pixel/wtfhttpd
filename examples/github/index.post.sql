-- @wtf-validate username required
-- @wtf-capture cached_data single
SELECT
    cache_get('github_user_' || @username);

-- @wtf-capture api_response single
SELECT
    CASE
        WHEN @cached_data IS NOT NULL THEN @cached_data
        ELSE (
            SELECT
                http_get("https://api.github.com/users/" || @username)
        )
    END;

SELECT
    CASE
        WHEN @cached_data IS NULL THEN cache_set('github_user_' || @username, @api_response)
        ELSE NULL
    END;

-- @wtf-capture user single
SELECT
    json_extract(@api_response, '$.body');

-- @wtf-store user_data
SELECT
    json_extract(@user, '$.login') as username,
    json_extract(@user, '$.location') as location,
    json_extract(@user, '$.followers') as followers,
    json_extract(@user, '$.following') as following,
    json_extract(@user, '$.bio') as bio,
    json_extract(@user, '$.avatar_url') as avatar_url,
    json_extract(@user, '$.name') as name,
    CASE
        WHEN @cached_data IS NOT NULL THEN 'HIT'
        ELSE 'MISS'
    END as cache_status;

INSERT INTO
    response_meta (name, value)
VALUES
    ("wtf-tpl", "github/index.html")