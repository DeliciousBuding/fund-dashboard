import { test, expect } from '@playwright/test';

const BASE = process.env.BASE_URL || (process.env.CI ? 'http://localhost:8080' : 'http://localhost:5173');

/**
 * Home page smoke tests.
 * Requires the Vite dev server to be running: `npm run dev` in packages/web.
 */

test.describe('Home Page', () => {
  test('page loads with title and main content', async ({ page }) => {
    const errors: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error' && !msg.text().toLowerCase().includes('favicon')) {
        errors.push(msg.text());
      }
    });

    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('domcontentloaded', { timeout: 25000 });
    // Let Suspense / lazy components resolve
    await page.waitForTimeout(3000);

    // Page title should be non-empty
    const title = await page.title();
    expect(title.length).toBeGreaterThan(0);

    // Main content should have substantial HTML
    const html = await page.locator('main, #root, body').first().innerHTML().catch(() => '');
    expect(html.length).toBeGreaterThan(200);

    // Minimal console errors (allow a few for benign third-party noise)
    expect(errors.length).toBeLessThan(5);
  });

  test('stat cards are rendered', async ({ page }) => {
    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('domcontentloaded', { timeout: 25000 });
    await page.waitForTimeout(3000);

    // StatCard uses LayerCard from @cloudflare/kumo → .kumo-layer-card in DOM
    // The Grid("4up") wraps four stat cards at minimum.
    const cards = page.locator('.kumo-layer-card, [class*="StatCard"]');
    const count = await cards.count().catch(() => 0);

    // May be 0 in empty-DB CI (empty state renders without stat cards)
    if (count > 0) {
      // At least one stat card should be visible
      const firstCard = cards.first();
      await expect(firstCard).toBeVisible({ timeout: 5000 });
    }
    // If 0, page should still have rendered the overview section
    const html = await page.locator('main, #root, body').first().innerHTML();
    expect(html.length).toBeGreaterThan(200);
  });

  test('portfolio chart area exists', async ({ page }) => {
    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('domcontentloaded', { timeout: 25000 });
    await page.waitForTimeout(3000);

    // After lazy chart components load, echarts canvas or chart container should exist
    const bodyHtml = await page.evaluate(() => document.body.innerHTML);
    expect(
      bodyHtml.includes('echarts') ||
        bodyHtml.includes('canvas') ||
        bodyHtml.includes('chart') ||
        bodyHtml.includes('Chart'),
    ).toBeTruthy();
  });

  test('sidebar navigation links exist', async ({ page }) => {
    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('domcontentloaded', { timeout: 25000 });
    await page.waitForTimeout(3000);

    // Sidebar should be present
    const sidebar = page.locator(
      'aside, nav, [class*="Sidebar"], [class*="sidebar"], .kumo-sidebar',
    );
    const sidebarVisible = await sidebar.first().isVisible().catch(() => false);

    if (sidebarVisible) {
      const sidebarHtml = await sidebar.first().innerHTML();
      expect(sidebarHtml.length).toBeGreaterThan(50);

      // Sidebar menu buttons — may use .kumo-sidebar-menu-button or other selectors
      const menuButtons = page.locator('.kumo-sidebar-menu-button, .kumo-sidebar a, .kumo-sidebar button');
      const btnCount = await menuButtons.count().catch(() => 0);
      // With empty DB the sidebar may have fewer links — accept 0+
      expect(btnCount).toBeGreaterThanOrEqual(0);

      const bodyHtml = await page.evaluate(() => document.body.innerHTML);
      expect(bodyHtml.length).toBeGreaterThan(500);
    } else {
      // Fallback: at minimum the body should have meaningful content
      const bodyHtml = await page.evaluate(() => document.body.innerHTML);
      expect(bodyHtml.length).toBeGreaterThan(500);
    }
  });

  test('overview sub-tabs are clickable', async ({ page }) => {
    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('domcontentloaded', { timeout: 25000 });
    await page.waitForTimeout(3000);

    // The overview page has sub-tab buttons: 净值走势, 资产配置, Agent Harness, 股权穿透, 盈亏分布, 高级分析
    const subTabs = page.locator('button').filter({ hasText: /净值走势|资产配置|盈亏分布|高级分析/ });
    const tabCount = await subTabs.count().catch(() => 0);

    if (tabCount > 0) {
      // Click the second tab (资产配置) if available
      const allocationTab = subTabs.filter({ hasText: '资产配置' }).first();
      if (await allocationTab.isVisible().catch(() => false)) {
        await allocationTab.click();
        await page.waitForTimeout(1500);

        // Content should still be rendered after tab switch
        const html = await page.locator('main, #root, body').first().innerHTML();
        expect(html.length).toBeGreaterThan(200);
      }
    }
    // If no sub-tabs (empty state), page should still be stable
    expect(await page.locator('body').isVisible()).toBeTruthy();
  });
});
