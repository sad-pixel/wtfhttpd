# WtfHttpd

The HTTP server that makes you go _wtf_!

---

> This is obviously not production ready, nor is it complete, though the features described here do work.

`wtfhttpd` is a cgi-esque web server written in Go that asks: what if your SQL script was your backend?

It uses file-based routing to map HTTP requests directly to .sql files, executes them against a SQLite database, and renders the results as either JSON or HTML via templates.

## File Based Routing

Routes are determined by the filesystem layout within the webroot directory.

- `webroot/index.sql` -> `ANY /`
- `webroot/users.sql` -> `ANY /users`
- `webroot/users.get.sql` -> `GET /users`
- `webroot/users.post.sql` -> `POST /users`

Supported methods are .get, .post, .put, .patch, .delete, .options. Files without a method extension respond to any HTTP method.

`wtfhttpd` supports path parameters for dynamic routes. If a request is made to /users/123, it will be handled by webroot/users/{id}.get.sql and the parameter will be available in the `path_params` table.

## Request Context

For each request, wtfhttpd creates a set of temporary tables that you can query.

- `path_params`: Contains URL path parameters.
  - `SELECT value FROM path_params WHERE name = 'id'`
- `query_params`: Contains URL query string parameters.
  - `SELECT value FROM query_params WHERE name = 'search'`
- `request_headers`: Contains all HTTP request headers.
- `request_form`: Contains all request form data fields (uploads aren't supported yet!)
- `request_meta`: Contains metadata like `method`, `path`, and `remote_addr`.
- `env_vars`: Contains environment variables from the server process.

## Parameter Binding

Path parameters, form data fields, and query parameters are also provided as named parameters to the SQL files.
For example, a query param called `name` can be referred to as `@name` in the SQL file, instead of having to write `SELECT value FROM query_params WHERE name = 'name'`

There is special handling for form data and query params that end with `[]`, which are intended for array-like use cases (example: checkboxes).
wtfhttpd will strip the `[]` and encode a json array, which can be accessed in SQL as `json_each(@id)`, assuming the param was called `id[]`

The order of precedence is Path > Form > Query.

## Early Returning

Routes can terminate early by calling the special function `wtf_abort`.
An optional http code and message can be passed to it.

## Response Handling

By default, the result of your final SELECT query is returned as `application/json.`

To control the response, you can INSERT into the `response_meta` table:

- Set HTTP Status: `INSERT INTO response_meta VALUES ('status', '404');`
- Set a Header: `INSERT INTO response_meta VALUES ('Content-Type', 'text/plain');`
- Render a Template: `INSERT INTO response_meta VALUES ('wtf-tpl', 'path/to/template.html');`

Templates use jinja2 syntax, and can be anywhere in the webroot.

A built-in status page is available at `/\_wtf` to view server statistics like uptime and the number of discovered routes.

## Configuration

`wtfhttpd` may be configured using a `wtf.toml` config file. The default values are shown below

```toml
host = "127.0.0.1"
port = 8080
db = "wtf.db"
web_root = "webroot"
live_reload = true
```
