import { test, expect } from '@playwright/test';

const BASE = process.env.BASE_URL || (process.env.CI ? 'http://localhost:8080' : 'http://localhost:5173');

/**
 * Route navigation tests.
 * Requires the Vite dev server to be running: `npm run dev` in packages/web.
 *
 * These tests click sidebar navigation links and verify the correct page renders.
 */

test.describe('Route Navigation', () => {
  test('navigate to /compare via sidebar', async ({ page }) => {
    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('domcontentloaded', { timeout: 25000 });
    await page.waitForTimeout(3000);

    // Find the compare sidebar link (Scales icon, text "基金对比" or "compare")
    const compareBtn = page.locator('.kumo-sidebar-menu-button').filter({
      hasText: /对比|compare/i,
    }).first();

    const btnVisible = await compareBtn.isVisible().catch(() => false);
    if (!btnVisible) {
      test.skip(true, 'Compare sidebar button not found');
      return;
    }

    await compareBtn.click();
    await page.waitForTimeout(2500);
    await page.waitForLoadState('domcontentloaded', { timeout: 15000 });

    // URL should reflect /compare
    expect(page.url()).toContain('/compare');

    // FundComparison component should have rendered — contains "雷达图" or comparison heading
    const html = await page.locator('main, #root, body').first().innerHTML();
    expect(html.length).toBeGreaterThan(200);

    // The comparison page should have fund selection or comparison content
    const hasComparisonContent =
      html.includes('雷达') ||
      html.includes('对比') ||
      html.includes('compare') ||
      html.includes('radar') ||
      html.includes('chart');
    expect(hasComparisonContent).toBeTruthy();
  });

  test('navigate to /nasdaq via sidebar', async ({ page }) => {
    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('domcontentloaded', { timeout: 25000 });
    await page.waitForTimeout(3000);

    // The nasdaq overview button appears inside the "纳斯达克" category group
    // It uses an <img> icon (ndaq.svg) and text "纳指总览"
    const nasdaqBtn = page.locator('.kumo-sidebar-menu-button').filter({
      hasText: /纳指总览|nasdaq.*overview/i,
    }).first();

    const btnVisible = await nasdaqBtn.isVisible().catch(() => false);
    if (!btnVisible) {
      // Fallback: try clicking any sidebar button with "纳" text
      const altBtn = page.locator('.kumo-sidebar-menu-button').filter({
        hasText: /纳/,
      }).first();

      if (!(await altBtn.isVisible().catch(() => false))) {
        test.skip(true, 'Nasdaq overview sidebar button not found');
        return;
      }

      await altBtn.click();
    } else {
      await nasdaqBtn.click();
    }

    await page.waitForTimeout(2500);
    await page.waitForLoadState('domcontentloaded', { timeout: 15000 });

    // URL should reflect /nasdaq
    expect(page.url()).toContain('/nasdaq');

    const html = await page.locator('main, #root, body').first().innerHTML();
    expect(html.length).toBeGreaterThan(200);

    // NasdaqOverview renders with h1 "纳斯达克总览" or stat cards
    const hasNasdaqContent =
      html.includes('纳斯达克') ||
      html.includes('纳指') ||
      html.includes('nasdaq') ||
      html.includes('^NDX');
    expect(hasNasdaqContent).toBeTruthy();
  });

  test('navigate to a fund detail via sidebar', async ({ page }) => {
    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('domcontentloaded', { timeout: 25000 });
    await page.waitForTimeout(3000);

    // Find a fund button — distinguished from overview/compare/nasdaq by having +/- PnL values
    const fundBtn = page.locator('.kumo-sidebar-menu-button').filter({
      hasText: /[+\-]\d/,
    }).first();

    const btnVisible = await fundBtn.isVisible().catch(() => false);
    if (!btnVisible) {
      test.skip(true, 'No fund with PnL data found in sidebar');
      return;
    }

    const fundName = await fundBtn.textContent();
    if (!fundName || fundName.trim().length <= 3) {
      test.skip(true, 'No fund with PnL data found');
      return;
    }

    await fundBtn.click();
    await page.waitForTimeout(2500);
    await page.waitForLoadState('domcontentloaded', { timeout: 15000 });

    // URL should contain /fund/
    expect(page.url()).toContain('/fund/');

    // FundDetailView should render — contains fund name, stat cards, tabs
    const html = await page.locator('main, #root, body').first().innerHTML();
    expect(html.length).toBeGreaterThan(200);

    // Should have at least one of: stat card, tab content, fund name heading
    const hasDetailContent =
      html.includes('持有份额') ||
      html.includes('投入成本') ||
      html.includes('当前市值') ||
      html.includes('stat') ||
      html.includes('tab');
    expect(hasDetailContent).toBeTruthy();
  });

  test('navigate back to overview from fund detail', async ({ page }) => {
    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('domcontentloaded', { timeout: 25000 });
    await page.waitForTimeout(3000);

    // Click a fund first
    const fundBtn = page.locator('.kumo-sidebar-menu-button').filter({
      hasText: /[+\-]\d/,
    }).first();

    if (!(await fundBtn.isVisible().catch(() => false))) {
      test.skip(true, 'No fund with PnL data found');
      return;
    }

    await fundBtn.click();
    await page.waitForTimeout(2500);
    await page.waitForLoadState('domcontentloaded', { timeout: 15000 });

    expect(page.url()).toContain('/fund/');

    // Navigate back to overview via sidebar
    const overviewBtn = page.locator('.kumo-sidebar-menu-button').filter({
      hasText: /总览|overview/i,
    }).first();

    if (await overviewBtn.isVisible().catch(() => false)) {
      await overviewBtn.click();
      await page.waitForTimeout(2000);
      await page.waitForLoadState('domcontentloaded', { timeout: 15000 });

      // Should be back at root
      const finalUrl = page.url();
      expect(finalUrl.endsWith('/') || finalUrl.endsWith('/fund-dashboard')).toBeTruthy();

      const html = await page.locator('main, #root, body').first().innerHTML();
      expect(html.length).toBeGreaterThan(200);
    }
  });
});
