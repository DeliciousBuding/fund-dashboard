# Fund Dashboard Deploy Runbook
最后更新：2026-06-19 01:49

## CI/CD

GitHub Actions workflow: `.github/workflows/ci.yml`.

Required green gates:

- `workspace-test`: runs root `npm test`, covering server service/datasource/crawler tests and web Vitest.
- `test-server`: focused Bun server tests.
- `build-server`: `bun build main.ts --outdir dist --target bun`.
- `test-web`: full web Vitest.
- `build-web`: Vite production build.
- `docker-build-smoke`: builds backend and web Docker images on PR without pushing.
- `build-and-push`: on `main`/`master` push only, pushes GHCR images tagged `latest` and commit SHA.

## Local Verification

```bash
npm test
cd packages/server && bun build main.ts --outdir dist --target bun
cd ../web && npm run build
```

## Images

- Backend: `ghcr.io/deliciousbuding/fund-dashboard/backend:<sha|latest>`
- Web: `ghcr.io/deliciousbuding/fund-dashboard/web:<sha|latest>`

## Production Deploy

Production compose file: `deploy/docker-compose.yml`.

Create host-local environment file:

```bash
cp deploy/.env.example deploy/.env
$EDITOR deploy/.env
```

```bash
cd /path/to/fund-dashboard
./deploy/deploy.sh --check
./deploy/deploy.sh
```

Post-deploy checks:

```bash
curl -fsS http://127.0.0.1:8765/api/health
curl -fsS http://127.0.0.1:8765/api/portfolio/harness
docker compose -f deploy/docker-compose.yml ps
```

Hermes should call `get_investment_harness_snapshot` for facts-only portfolio context. The tool must not be used as an execution decision; Hermes/Agent owns all investment actions.

## Rollback

```bash
./deploy/rollback.sh
```
