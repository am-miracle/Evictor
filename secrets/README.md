# Local secrets

Actual secret files are ignored by git and mounted read-only under
`/run/secrets/`. Never commit files from this directory.

Required files:

| File | Purpose |
| --- | --- |
| `postgres_password` | PostgreSQL password |
| `database_url` | Backend PostgreSQL connection URL |
| `encryption_key` | Backend credential-encryption key |

Generate development secrets with `./scripts/setup-secrets.sh dev`. For
staging and production, use a separate protected directory or secrets manager
and set `SECRETS_DIR` to it.
