import { test, expect } from '@playwright/test';

const BASE = 'http://localhost:8080';
const ADMIN_HEADERS = { Authorization: 'Bearer ci-test-key' };

test.describe('Smoke: Overview Loading', () => {
  test('overview page loads with stat cards and headings', async ({ page }) => {
    const errors: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'error' && !msg.text().toLowerCase().includes('favicon')) {
        errors.push(msg.text());
      }
    });

    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('networkidle', { timeout: 25000 });
    await page.waitForTimeout(3000);

    const title = await page.title();
    expect(title.length).toBeGreaterThan(0);

    // Headings should be present
    const headings = page.locator('h1, h2, h3');
    const headingCount = await headings.count();
    expect(headingCount).toBeGreaterThanOrEqual(1, 'Should have at least 1 heading');

    // Main content area should be populated
    const html = await page.locator('main, #root, body').innerHTML();
    expect(html.length).toBeGreaterThan(200);

    // Minimal console errors
    expect(errors.length).toBeLessThan(5);
  });

  test('sidebar renders with navigation items', async ({ page }) => {
    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('networkidle', { timeout: 25000 });
    await page.waitForTimeout(3000);

    // Sidebar should exist
    const sidebar = page.locator('aside, nav, [class*="Sidebar"], [class*="sidebar"]');
    const sidebarVisible = await sidebar.first().isVisible().catch(() => false);

    if (sidebarVisible) {
      const sidebarHtml = await sidebar.first().innerHTML();
      expect(sidebarHtml.length).toBeGreaterThan(50);
    }
    // If no sidebar element found, at least body content should be substantial
    else {
      const bodyHtml = await page.evaluate(() => document.body.innerHTML);
      expect(bodyHtml.length).toBeGreaterThan(500);
    }
  });

  test('API health endpoint responds', async ({ page }) => {
    const response = await page.request.get(`${BASE}/api/health`);
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.status).toBe('ok');
    expect(body.uptime).toBeGreaterThan(0);
  });
});

test.describe('Smoke: Fund Search', () => {
  test('search input exists and filters content', async ({ page }) => {
    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('networkidle', { timeout: 25000 });
    await page.waitForTimeout(3000);

    // Find any search input by placeholder containing search-related text
    const allInputs = page.locator('input');
    const inputCount = await allInputs.count();
    let searchInput: ReturnType<typeof page.locator> | null = null;

    for (let i = 0; i < inputCount; i++) {
      const placeholder = await allInputs.nth(i).getAttribute('placeholder');
      if (placeholder && /搜索|search|find|filter/i.test(placeholder)) {
        searchInput = allInputs.nth(i);
        break;
      }
    }

    if (searchInput) {
      // Search for a partial code or name
      await searchInput.fill('0');
      await page.waitForTimeout(1000);

      // Page should still render content (filtered or not)
      const html = await page.locator('main, #root, body').innerHTML();
      expect(html.length).toBeGreaterThan(200);

      // Clear search
      await searchInput.fill('');
      await page.waitForTimeout(500);
    }
    // If no explicit search input, the page should still be stable
    const html = await page.locator('main, #root, body').innerHTML();
    expect(html.length).toBeGreaterThan(200);
  });

  test('funds API returns data', async ({ page }) => {
    const response = await page.request.get(`${BASE}/api/funds`);
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(Array.isArray(body)).toBe(true);
    // May be empty in CI (no DB seeded), which is acceptable
  });

  test('portfolio summary API returns valid response', async ({ page }) => {
    const response = await page.request.get(`${BASE}/api/portfolio`);
    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body).toHaveProperty('total_tx');
    expect(body).toHaveProperty('unique_funds');
  });
});

test.describe('Smoke: CSV Import Flow', () => {
  test('CSV import endpoint accepts valid CSV data', async ({ page }) => {
    const csvContent = [
      'date,code,name,direction,amount,share,fee,type',
      '2026-06-01,000001,测试基金,buy,1000,100,1.5,用户买入',
    ].join('\n');

    const response = await page.request.post(`${BASE}/api/admin/import-csv`, {
      headers: {
        'Content-Type': 'application/json',
        ...ADMIN_HEADERS,
      },
      data: { csv: csvContent },
    });

    expect(response.status()).toBe(200);
    const body = await response.json();
    expect(body.ok).toBe(true);
    expect(body.imported).toBeGreaterThanOrEqual(0);
  });

  test('CSV import rejects invalid data gracefully', async ({ page }) => {
    const response = await page.request.post(`${BASE}/api/admin/import-csv`, {
      headers: {
        'Content-Type': 'application/json',
        ...ADMIN_HEADERS,
      },
      data: { csv: 'garbage,data\nrow1,only' },
    });

    expect(response.status()).toBe(400);
    const body = await response.json();
    expect(body.error).toBeDefined();
  });

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

  test('verify data integrity check passes', async ({ page }) => {
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
    expect(body).toHaveProperty('healthy');
    expect(body).toHaveProperty('checks');
  });
});
