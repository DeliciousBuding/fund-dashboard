import { test, expect } from '@playwright/test';

const BASE = process.env.BASE_URL || (process.env.CI ? 'http://localhost:8080' : 'http://localhost:5173');
const ADMIN_HEADERS = { Authorization: 'Bearer ci-test-key' };

test.describe('Smoke: Overview Loading', () => {
  test('overview page loads with content shell', async ({ page }) => {
    const errors: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'error' && !msg.text().toLowerCase().includes('favicon')) {
        errors.push(msg.text());
      }
    });

    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('domcontentloaded', { timeout: 25000 });
    // Prefer attached shell over fixed sleep (Suspense may still settle).
    const shell = page.locator('main, #root, body').first();
    await shell.waitFor({ state: 'attached', timeout: 15000 });
    await expect(shell).not.toBeEmpty({ timeout: 10000 });

    const title = await page.title();
    expect(title.length).toBeGreaterThan(0);

    const html = await shell.innerHTML().catch(() => '');
    expect(html.length).toBeGreaterThan(200);
    expect(errors.length).toBeLessThan(8);
  });

  test('API health endpoint responds with Go runtime boundary', async ({ page }) => {
    const response = await page.request.get(`${BASE}/api/health`);
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.status).toBe('ok');
    expect(body.service).toBeTruthy();
    expect(body.facts_only).toBe(true);
    expect(body.backup_producer_enabled).toBe(false);
    // Request-ID middleware should always set a response header.
    expect(response.headers()['x-request-id'] || response.headers()['X-Request-Id']).toBeTruthy();
  });
});

test.describe('Smoke: Core read APIs', () => {
  test('funds API returns an array', async ({ page }) => {
    const response = await page.request.get(`${BASE}/api/funds`);
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(Array.isArray(body)).toBe(true);
  });

  test('portfolio summary API returns valid response', async ({ page }) => {
    const response = await page.request.get(`${BASE}/api/portfolio`);
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body).toHaveProperty('total_tx');
    expect(body).toHaveProperty('unique_funds');
  });

  test('market indices API returns array contract', async ({ page }) => {
    const response = await page.request.get(`${BASE}/api/market/indices`);
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(Array.isArray(body)).toBe(true);
  });

  test('analysis compare requires codes and returns funds envelope when provided', async ({ page }) => {
    const bad = await page.request.get(`${BASE}/api/analysis/compare`);
    expect(bad.status()).toBe(400);

    const ok = await page.request.get(`${BASE}/api/analysis/compare?codes=019173,aapl`);
    expect(ok.status()).toBe(200);
    const body = await ok.json();
    expect(Array.isArray(body.funds)).toBe(true);
  });
});

test.describe('Smoke: Auth boundaries', () => {
  test('admin endpoints require auth', async ({ page }) => {
    const response = await page.request.get(`${BASE}/api/admin/status`);
    expect(response.status()).toBe(401);
  });

  test('admin endpoints work with valid auth', async ({ page }) => {
    const response = await page.request.get(`${BASE}/api/admin/status`, {
      headers: ADMIN_HEADERS,
    });
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.ok).toBe(true);
  });

  test('admin dashboard returns go_version', async ({ page }) => {
    const response = await page.request.get(`${BASE}/api/admin/dashboard`, {
      headers: ADMIN_HEADERS,
    });
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.ok).toBe(true);
    expect(body.system?.go_version).toBeTruthy();
    expect(body.system?.node_version).toBeUndefined();
  });

  test('verify data integrity check returns report', async ({ page }) => {
    const response = await page.request.get(`${BASE}/api/admin/verify`, {
      headers: ADMIN_HEADERS,
    });
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body).toHaveProperty('ok');
    expect(body).toHaveProperty('issues');
  });

  test('db integrity check returns report', async ({ page }) => {
    const response = await page.request.get(`${BASE}/api/admin/db-integrity`, {
      headers: ADMIN_HEADERS,
    });
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body).toHaveProperty('overall');
    expect(body.overall).toBe('ok');
    expect(body).toHaveProperty('checks');
  });

  test('mcp endpoint is fail-closed without key', async ({ page }) => {
    const response = await page.request.post(`${BASE}/mcp`, {
      data: { jsonrpc: '2.0', id: '1', method: 'tools/list' },
    });
    expect(response.status()).toBe(401);
  });

  test('mcp endpoint lists only implemented tools with admin key', async ({ page }) => {
    const response = await page.request.post(`${BASE}/mcp`, {
      headers: ADMIN_HEADERS,
      data: { jsonrpc: '2.0', id: '1', method: 'tools/list' },
    });
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.error).toBeFalsy();
    const tools = body.result?.tools || [];
    expect(Array.isArray(tools)).toBe(true);
    expect(tools.length).toBeGreaterThan(10);
    expect(tools.length).toBeGreaterThanOrEqual(40);
    expect(tools.length).toBeLessThanOrEqual(50);
    const names = tools.map((t: { name: string }) => t.name);
    expect(names).toContain('get_portfolio_summary');
    expect(names).toContain('crawl_nav');
    expect(names).toContain('add_fund');
    expect(names).toContain('delete_fund');
  });

  // CSV bulk import is not ported on Go; SPA path uses /api/transactions/import with EdgeAuth.
  test('legacy import-csv route is gone', async ({ page }) => {
    const response = await page.request.post(`${BASE}/api/admin/import-csv`, {
      headers: {
        'Content-Type': 'application/json',
        ...ADMIN_HEADERS,
      },
      data: { csv: 'date,code\n2026-06-01,000001' },
    });
    // Not implemented: 404 (or 405 if method-matched elsewhere). Must not succeed.
    expect([404, 405]).toContain(response.status());
  });
});
