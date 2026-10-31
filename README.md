# MamieNellie service

The backend for the MamieNellie recipes app: recipes, grocery items, units, categories and the
grocery list.

## Technologies

- Go, Gin, GORM, Postgres
- goose for versioned SQL migrations (applied at startup)
- Docker Compose for local development

## Layout

A modular monolith. Each module under `internal/` has four layers, and depends inwards only:

```
cmd/main.go                 wiring only
internal/
  platform/                 config, database, HTTP server: shared plumbing, no business rules
  shared/                   the small shared kernel: error kinds, the owner of the data
  auth/                     accounts, Sign in with Apple/Google, access and refresh tokens
  recipes/
    module.go               the only package other code uses
    entrypoint/api/         HTTP controllers and request/response types
    application/<usecase>/  one package per use case; each declares the repository it needs
    domain/model/           entities and business rules, no framework imports
    infrastructure/         Postgres repositories and migrations, the starter data file
```

## Accounts and sign-in

People sign in with Apple or Google. The app sends the provider's ID token to
`POST /v1/auth/apple` or `/v1/auth/google`; the service checks it against Apple's or Google's
published keys and the app's client IDs, creates the account on the first sign-in, and returns:

- an **access token** (15 minutes), sent with every other request as `Authorization: Bearer <token>`.
  Every piece of data belongs to the account it identifies.
- a **refresh token** (60 days), traded at `POST /v1/auth/refresh` for a new pair. Each works once;
  if a used one comes back, the whole session ends, in case it was stolen.

`POST /v1/auth/sign-out` ends a session; `DELETE /v1/me` deletes the account and everything saved
with it. Apple and Google sign-ins are separate accounts even with the same email.

Locally, `POST /v1/auth/dev` with `{"name":"ada"}` signs in without Apple or Google: in Swagger, call
it, then click **Authorize** and enter `Bearer <accessToken>`. It doesn't exist in prod.

## Units, categories and grocery items

- **Units are the same for everyone** and only change with a migration (`00002_create_catalog.sql`).
  A unit with a base converts to every unit with the same base: 1 cup = 240 ml and 1 tbsp = 15 ml,
  so 1 cup = 16 tbsp. Units without one (pack, head…) only convert through an item's own rules.
  `soldWhole` units are rounded up to whole numbers on the grocery list.
- **Categories and grocery items belong to each account.** Every account starts with its own copy
  of `internal/recipes/infrastructure/starterdata/starter_data.json`, made on sign-in. Starter
  records can be edited and hidden, not deleted; the account's own can be deleted.
- **An item's own conversions** ("Garlic: 1 head = 10 clove") are chained with the standard ones,
  so garlic goes head → clove → g → kg. A rule that already follows from the others or from the
  units is refused, so two rules can never disagree.
- **`GET /v1/catalog`** returns all of it, so the app can convert amounts offline. Send its `ETag`
  back as `If-None-Match`: while nothing changed, the answer is `304` with no body.
- The app converts amounts with the same logic.
  `internal/recipes/domain/model/testdata/conversion_cases.json` holds the cases both must agree on;
  the Go tests run them, and the app's tests should too.

## Environments

The same build runs everywhere; only environment variables differ (see `.env.example`).

| | local | prod (AWS, later) |
|---|---|---|
| `APP_ENV` | `local` | `prod` |
| `DATABASE_URL` | Postgres in Docker Compose | RDS |
| `JWT_SIGNING_KEY` | any 32+ characters | Secrets Manager (`mamie-nellie-auth`) |
| `APPLE_CLIENT_IDS`, `GOOGLE_CLIENT_IDS` | empty (use `/v1/auth/dev`) | the app's bundle ID and Google client IDs |
| Logs | text, with SQL | JSON |
| Swagger UI | http://localhost:8080/swagger/index.html | off |

Prod runs on AWS ECS Fargate; see [deploy/README.md](deploy/README.md) for the Terraform, the deploy
workflow, the `mamie-nellie` cost tags and the setup steps.

## Run locally

```bash
cp .env.example .env.local   # once
make run                     # starts Postgres in Docker, then the server on :8080
```

## Tests & linting

```bash
make test               # unit tests; database tests skip themselves
make test-integration   # everything, against Postgres in Docker
make lint
make swag               # regenerate docs/ after changing API annotations
```

## Authors & Copyrights

Shaimaa Sabry

[![GitHub](https://img.icons8.com/ios-glyphs/30/000000/github.png)](https://github.com/ShaimaaSabry)
[![LinkedIn](https://img.icons8.com/ios-filled/30/0A66C2/linkedin.png)](https://www.linkedin.com/in/shaimaa-sabry-161b71a0/)
