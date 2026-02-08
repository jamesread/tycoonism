# HTTP auth (httpauthshim)

The backend uses [jamesread/httpauthshim](https://pkg.go.dev/github.com/jamesread/httpauthshim) for HTTP authentication. Configuration is loaded from the root key **`auth`** in the application config. The config file is `~/.config/wf/config.yaml`.

## Implementation notes (from library docs)

- **Entry point**: Create an `AuthShimContext` with `auth.NewAuthShimContext(cfg, sessionStorage)`. Do not mutate the config after creation.
- **Session storage**: Use `sessions.NewSessionStorage(sessions.NewYAMLPersistence())`. Sessions are stored under the config `BaseDir` (default `~/.config/auth/`) or `AUTH_HOME`. Call `ctx.Shutdown()` when the context is no longer needed.
- **Auth chain**: Add providers with `ctx.AddProvider(checkFunc)`. Use the context’s chain; avoid the deprecated global `auth.AddProvider()`.
- **Request auth**: Use `authCtx.AuthFromHttpReq(r)` or `authCtx.AuthFromHttpReqWithError(r)` in handlers. Guest users are returned when unauthenticated.
- **Providers**: JWT (header/cookie, HMAC/RSA/JWKS), OAuth2 (e.g. GitHub/Google), local users (Argon2id), trusted headers, mTLS, HTTP Basic, Bearer tokens. See pkg.go.dev for provider packages and wiring.

## Example config layout

Place the httpauthshim config under the **`auth`** key in `~/.config/wf/config.yaml`:

```yaml
auth:
  baseDir: "~/.config/auth"
  jwt:
    header: "Authorization"
    claimUsername: "sub"
    claimUserGroup: "groups"
  localUsers:
    enabled: true
    users:
      - username: "admin"
        usergroup: "admin"
        password: "$argon2id$v=19$m=65536,t=4,p=1$..."
  accessControlLists:
    - name: "admin"
      matchUsernames: ["admin"]
      matchUsergroups: ["admin"]
```

See [authpublic.Config](https://pkg.go.dev/github.com/jamesread/httpauthshim/authpublic#Config) and the [main package](https://pkg.go.dev/github.com/jamesread/httpauthshim) for all options (OAuth2, mTLS, trusted headers, etc.).

## OAuth2 with Vite dev proxy

When using the Vite dev server, `/oauth` is proxied to the backend. The proxy sets the request `Host` header to the backend (`localhost:4838`) so the backend sees the correct host.

**Provider URLs**: Each OAuth2 provider must use the **full** authorization URL (e.g. `https://github.com/login/oauth/authorize`). Do not set `authUrl` to a relative path (e.g. `/oauth`) or to the frontend URL, or the redirect after clicking “Sign in” will go to the wrong place (e.g. the dev server) and return 404. For built-in providers (`github`, `google`) you can omit `authUrl` so the library uses its defaults.
