# leanPrice local operations

This runbook covers first administrator setup and password recovery for the local single-container instance. Do not publish the service beyond a trusted network path.

## First administrator

1. Start the service with the default loopback binding: `docker compose up --build -d`.
2. Read the one-time token from the protected data volume: `docker compose exec leanprice cat /data/setup-token`.
3. Open `http://127.0.0.1:8080/setup` and create the administrator. The setup token is consumed and its file is removed after successful setup.
4. Sign in at `http://127.0.0.1:8080/login`.

The token is not written to application logs. Treat it as a password while first-run setup is pending. If the file is missing before setup, the next startup generates a replacement token and updates its stored digest. Do not expose `/data` over a file share.

## Change the administrator password

Use **Settings → Change administrator password** while signed in. Password changes revoke all active sessions and send the browser back to sign-in.

## Recover access when the password is forgotten

The reset command must run while no leanPrice process has the database open. It reads both password entries from an interactive terminal without echo, updates the Argon2id hash, records an audit event and revokes all sessions.

```sh
docker compose stop leanprice
docker compose run --rm --no-deps leanprice admin-password-reset owner
docker compose up -d leanprice
```

Replace `owner` with the administrator name created during setup. The data volume and application image remain the same. The database process lock rejects the reset if the service is still running.

## Create and restore a backup

Create a consistent online backup while the service is running. The destination must be under `/data`, which is the persistent application volume:

```sh
make backup BACKUP=/data/backups/leanprice.sqlite
```

The command writes the SQLite snapshot and its `.manifest.json` sidecar, then validates the snapshot. Keep both files together. Do not copy only the SQLite main file while the application is running.

Restore a previously created backup with:

```sh
make restore BACKUP=/data/backups/leanprice.sqlite
```

The script stops the service, validates the backup and manifest, creates a pre-restore safety snapshot, restores the database and starts the service. It checks application readiness for up to one minute. If readiness fails and a pre-restore database existed, it stops the service and reinstates that safety snapshot before restarting. Keep the terminal open and do not interrupt the process while it is replacing the database. The restore command rejects paths outside `/data`.

The safety snapshot protects the database replacement and readiness check. It does not roll back a container image or application binary; use the previously recorded compatible image when recovering from an application upgrade. Keep a tested backup outside the device as well as on the `/data` volume.

## Network and proxy settings

- Native execution binds to `127.0.0.1:8080` by default. Compose publishes that port on loopback.
- `LEANPRICE_BIND_ADDRESS` controls the host-side Compose port publication; it defaults to loopback. `LEANPRICE_ADDR` controls the address inside the container and is set by Compose. Change either only when the host firewall and reverse-proxy trust boundary are configured for the intended clients.
- `LEANPRICE_TRUSTED_PROXIES` accepts comma-separated CIDR ranges. Forwarded HTTPS is trusted only when the immediate peer belongs to one of those ranges. Do not add client or general LAN ranges as trusted proxies.
- The first-run token remains required even when a proxy is configured. Retailer sources are disabled until separately approved; never use manual product entry as a reason to bypass source policy.
- Set `LEANPRICE_WEBHOOK_URL` and, if needed, `LEANPRICE_WEBHOOK_TOKEN` through the operator’s protected environment/secret mechanism. Private targets additionally require an exact matching `LEANPRICE_WEBHOOK_PRIVATE_CIDRS` allowlist. The Settings page can send a fixed test payload; it does not create a price alert. No raw endpoint or transport error is shown in that page.
