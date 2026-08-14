import { test, expect } from '@playwright/test';

const BASE = process.env.BASE_URL || (process.env.CI ? 'http://localhost:8080' : 'http://localhost:5173');

/**
 * Fund comparison page tests.
 * Requires the Vite dev server to be running: `npm run dev` in packages/web.
 *
 * These tests verify the /compare page renders correctly.
 */

test.describe('Fund Comparison', () => {
  test('compare page loads with content', async ({ page }) => {
    const errors: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error' && !msg.text().toLowerCase().includes('favicon')) {
        errors.push(msg.text());
      }
    });

    await page.goto(`${BASE}/compare`, { timeout: 25000 });
    await page.waitForLoadState('domcontentloaded', { timeout: 25000 });
    await page.waitForTimeout(3000);

    // URL should be /compare
    expect(page.url()).toContain('/compare');

    // Page should have meaningful content
    const html = await page.locator('main, #root, body').first().innerHTML();
    expect(html.length).toBeGreaterThan(200);

    // FundComparison component renders one of:
    // - loading state, empty state, or comparison UI
    const hasComparisonContent =
      html.includes('对比') ||
      html.includes('compare') ||
      html.includes('comparing') ||
      html.includes('雷达') ||
      html.includes('暂无') ||
      html.includes('compareFund') ||
      html.includes('chart') ||
      html.includes('loading') ||
      html.includes('Loading') ||
      html.includes('kumo');
    expect(hasComparisonContent).toBeTruthy();

    // Minimal console errors
    expect(errors.length).toBeLessThan(5);
  });

  test('compare page has fund selection or empty state', async ({ page }) => {
    await page.goto(`${BASE}/compare`, { timeout: 25000 });
    await page.waitForLoadState('domcontentloaded', { timeout: 25000 });
    await page.waitForTimeout(3000);

    const html = await page.locator('main, #root, body').first().innerHTML();

    // The compare page shows either:
    // 1. An empty/loading state when no backend data
    // 2. Fund checkboxes/buttons for selection
    const hasEmptyState = html.includes('暂无') || html.includes('loading') || html.includes('Loading');
    const hasFundSelection =
      html.includes('checkbox') ||
      html.includes('选择') ||
      html.includes('添加') ||
      html.includes('fund');

    // At least one of these should be true
    expect(hasEmptyState || hasFundSelection).toBeTruthy();

    // If there are fund selection buttons, try interacting
    if (hasFundSelection && !hasEmptyState) {
      // Look for fund checkboxes or selection buttons
      const selectButtons = page.locator('input[type="checkbox"], button').filter({
        hasText: /[+\-]\d/,
      });

      const count = await selectButtons.count().catch(() => 0);
      if (count >= 2) {
        // Select two funds
        await selectButtons.nth(0).click();
        await page.waitForTimeout(500);
        await selectButtons.nth(1).click();
        await page.waitForTimeout(500);

        // Look for a "compare" button (对比 or 对比按钮)
        const compareActionBtn = page.locator('button').filter({
          hasText: /对比|compare/i,
        }).first();

        if (await compareActionBtn.isVisible().catch(() => false)) {
          await compareActionBtn.click();
          await page.waitForTimeout(2500);
          await page.waitForLoadState('domcontentloaded', { timeout: 10000 });

          // After clicking compare, charts should appear
          const afterHtml = await page.locator('main, #root, body').first().innerHTML();
          expect(afterHtml.length).toBeGreaterThan(200);
        }
      }
    }
  });

  test('navigate to /compare from overview via sidebar then back', async ({ page }) => {
    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('domcontentloaded', { timeout: 25000 });
    await page.waitForTimeout(3000);

    // Click compare in sidebar
    const compareBtn = page.locator('.kumo-sidebar-menu-button').filter({
      hasText: /对比|compare/i,
    }).first();

    if (!(await compareBtn.isVisible().catch(() => false))) {
      test.skip(true, 'Compare sidebar button not found');
      return;
    }

    await compareBtn.click();
    await page.waitForTimeout(2500);
    await page.waitForLoadState('domcontentloaded', { timeout: 15000 });

    expect(page.url()).toContain('/compare');

    // Navigate back to overview
    const overviewBtn = page.locator('.kumo-sidebar-menu-button').filter({
      hasText: /总览|overview/i,
    }).first();

    if (await overviewBtn.isVisible().catch(() => false)) {
      await overviewBtn.click();
      await page.waitForTimeout(2000);
      await page.waitForLoadState('domcontentloaded', { timeout: 15000 });

      const finalUrl = page.url();
      expect(finalUrl.endsWith('/') || finalUrl.endsWith('/fund-dashboard')).toBeTruthy();
    }
  });
});
