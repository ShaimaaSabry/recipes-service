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

## Recipes, the menu and the grocery list

- **Recipes** have servings (1 to 99), ingredients and steps. An ingredient is one of the account's
  items, in a unit it's cooked in (or bought in, for something nobody cooks with).
  `GET /v1/recipes/{id}?servings=4` scales the amounts. An item a recipe uses can't be deleted, or
  lose that unit.
- **Photos** go straight from the app to S3: `POST /v1/recipes/{id}/photo/upload` returns a
  15-minute upload link, the app `PUT`s the image to it, then attaches it with
  `PUT /v1/recipes/{id}/photo`. The service then checks it's there, an image, and at most 10 MB.
  CloudFront serves it at `photoUrl`. Without `PHOTOS_BUCKET` (locally, by default), uploads answer
  `503`.
- **The upcoming menu** is planned meals: a recipe for a number of people. The app makes each meal's
  ID, so it can plan offline: `PUT /v1/meals/{id}` plans or updates a meal, and
  `DELETE /v1/meals/{id}` removes it. Both are safe to repeat. Removing a meal leaves the grocery
  list alone; deleting a recipe removes its meals.
- **The grocery list** has one line per item, named by item. The app changes it with
  `POST /v1/grocery-list/actions`: `add`, `setQuantity`, `setUnit`, `check`, `uncheck`, `remove`
  and `clearChecked`. Offline, the app queues these and previews the result with the same rules,
  then sends the queue.
  - Each action has an ID made by the app. A resent one is recognised (`alreadyApplied`) and not
    applied twice.
  - A refused one (`rejected`) changes nothing and can be sent again. `missingConversion` names the
    conversion to ask the user for.
  - A line keeps its exact amount (0.04 bottle). `buyQuantity` rounds it up for units sold whole
    (1 bottle); 1500 g on an item bought in kg becomes 1.5 kg.
  - `testdata/grocery_list_cases.json` has the shared cases for the app's preview.

## Importing what the app kept on the phone

Before accounts, the app stored everything on the phone. After the first sign-in, it sends that
data once with `POST /v1/imports`: an `importId` it makes and keeps, and its stored data as it is
(data version 4).

- Starter categories and items are matched to the account's copies by their seed IDs
  (`item-garlic` is `garlic`), and take on the local renames, units, conversions and hidden flags.
  The user's own are matched by name, or created.
- Units the service doesn't have become `piece`. Units are added to an item where a recipe or the
  list needs them, and never taken away while something uses them.
- Recipes are created. Meals not yet cooked are planned; cooked ones are dropped. Grocery lines
  join the list; an item already on it gets the amount added.
- Anything that can't be kept as it is is fixed or left out and listed in `notes`, to show the
  user. It all happens in one transaction, or none of it does.
- Resending the same `importId` returns the first result, with `imported: false`.
- The app then uploads each recipe's photo to its new ID (`recipeIds` maps old to new), and only
  then deletes its local data. Until then it keeps the data and tries again on the next launch.
- `internal/recipes/testdata/local_data_v4.json` is the app's seed data plus a user's changes,
  made from the app's `src/data/seed.ts`.

## Environments

The same build runs everywhere; only environment variables differ (see `.env.example`).

| | local | prod (AWS, later) |
|---|---|---|
| `APP_ENV` | `local` | `prod` |
| `DATABASE_URL` | Postgres in Docker Compose | RDS |
| `JWT_SIGNING_KEY` | any 32+ characters | Secrets Manager (`mamie-nellie-auth`) |
| `APPLE_CLIENT_IDS`, `GOOGLE_CLIENT_IDS` | empty (use `/v1/auth/dev`) | the app's bundle ID and Google client IDs |
| `PHOTOS_BUCKET`, `PHOTOS_URL` | empty (no photos) | the photos bucket and its CloudFront address (set by Terraform) |
| Logs | text, with SQL | JSON |
| Swagger UI | http://localhost:8080/swagger/index.html | off |

Prod runs on AWS ECS Fargate; see [deploy/README.md](deploy/README.md) for the Terraform, the deploy
workflow, the `mamie-nellie` cost tags and the setup steps.

## Run locally

```bash
cp .env.example .env.local   # once
make run                     # starts Postgres in Docker, then the server on :8080
```

Until the first deploy, migrations may still be edited in place. If startup then fails with a
migration error, reset the local database: `docker compose down -v && make run`.

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
