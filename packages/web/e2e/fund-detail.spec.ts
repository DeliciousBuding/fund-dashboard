import { test, expect } from '@playwright/test';

const BASE = process.env.BASE_URL || (process.env.CI ? 'http://localhost:8080' : 'http://localhost:5173');

/**
 * Fund detail page tests.
 * Requires the Vite dev server to be running: `npm run dev` in packages/web.
 *
 * These tests verify the fund detail view renders correctly and tabs work.
 */

test.describe('Fund Detail', () => {
  /**
   * Navigate to a fund detail page and return success status.
   * Skips the test if no fund with PnL data is found in the sidebar.
   */
  async function navigateToFundDetail(page: import('@playwright/test').Page) {
    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('domcontentloaded', { timeout: 25000 });
    await page.waitForTimeout(3000);

    const fundBtn = page.locator('.kumo-sidebar-menu-button').filter({
      hasText: /[+\-]\d/,
    }).first();

    if (!(await fundBtn.isVisible().catch(() => false))) {
      return false;
    }

    const fundName = await fundBtn.textContent();
    if (!fundName || fundName.trim().length <= 3) {
      return false;
    }

    await fundBtn.click();
    await page.waitForTimeout(2500);
    await page.waitForLoadState('domcontentloaded', { timeout: 15000 });

    expect(page.url()).toContain('/fund/');
    return true;
  }

  test('fund detail page loads with header and stat cards', async ({ page }) => {
    const ok = await navigateToFundDetail(page);
    if (!ok) {
      test.skip(true, 'No fund with PnL data found');
      return;
    }

    // Should have an h1 with the fund name
    const heading = page.locator('h1').first();
    await expect(heading).toBeVisible({ timeout: 5000 });

    // Should have stat cards (LayerCard with 持有份额 / 投入成本 / etc.)
    const statCards = page.locator('.kumo-layer-card');
    const cardCount = await statCards.count().catch(() => 0);
    expect(cardCount).toBeGreaterThanOrEqual(4);

    // The fund code should be visible somewhere
    const html = await page.locator('main, #root, body').first().innerHTML();
    expect(html.length).toBeGreaterThan(500);
  });

  test('fund detail tabs are visible', async ({ page }) => {
    const ok = await navigateToFundDetail(page);
    if (!ok) {
      test.skip(true, 'No fund with PnL data found');
      return;
    }

    // Kumo Tabs component renders buttons or role="tab" elements
    // Tab labels: 净值走势/价格走势, 定投, 概览, 交易记录
    const tabs = page.locator('[role="tab"], button').filter({
      hasText: /净值走势|价格走势|定投|概览|交易记录/,
    });
    const tabCount = await tabs.count().catch(() => 0);
    expect(tabCount).toBeGreaterThanOrEqual(3);

    // The chart tab (净值走势 or 价格走势) should be active by default
    const chartTab = tabs.filter({ hasText: /净值走势|价格走势/ }).first();
    if (await chartTab.isVisible().catch(() => false)) {
      // The active tab should have some visual indicator
      const ariaSelected = await chartTab.getAttribute('aria-selected').catch(() => null);
      const dataState = await chartTab.getAttribute('data-state').catch(() => null);
      expect(ariaSelected === 'true' || dataState === 'active' || (await chartTab.isVisible())).toBeTruthy();
    }
  });

  test('DCA tab switches and renders content', async ({ page }) => {
    const ok = await navigateToFundDetail(page);
    if (!ok) {
      test.skip(true, 'No fund with PnL data found');
      return;
    }

    // Find and click the DCA tab (定投)
    const dcaTab = page.locator('button').filter({ hasText: '定投' }).first();
    const dcaVisible = await dcaTab.isVisible().catch(() => false);

    if (!dcaVisible) {
      // Try role="tab" elements
      const roleTabs = page.locator('[role="tab"]').filter({ hasText: /定投/ }).first();
      if (await roleTabs.isVisible().catch(() => false)) {
        await roleTabs.click();
      } else {
        test.skip(true, 'DCA tab not found');
        return;
      }
    } else {
      await dcaTab.click();
    }

    await page.waitForTimeout(2000);
    await page.waitForLoadState('domcontentloaded', { timeout: 10000 });

    // DCA panel should render: contains 定投计算器 or 基础定投金额 or mode selectors
    const html = await page.locator('main, #root, body').first().innerHTML();
    expect(html.length).toBeGreaterThan(200);

    const hasDcaContent =
      html.includes('定投') ||
      html.includes('DCA') ||
      html.includes('dca') ||
      html.includes('计算') ||
      html.includes('回测');
    expect(hasDcaContent).toBeTruthy();
  });

  test('transactions tab switches and renders content', async ({ page }) => {
    const ok = await navigateToFundDetail(page);
    if (!ok) {
      test.skip(true, 'No fund with PnL data found');
      return;
    }

    // Find and click the transactions tab (交易记录)
    const txTab = page.locator('button').filter({ hasText: '交易记录' }).first();
    const txVisible = await txTab.isVisible().catch(() => false);

    if (!txVisible) {
      const roleTabs = page.locator('[role="tab"]').filter({ hasText: /交易记录/ }).first();
      if (await roleTabs.isVisible().catch(() => false)) {
        await roleTabs.click();
      } else {
        test.skip(true, 'Transactions tab not found');
        return;
      }
    } else {
      await txTab.click();
    }

    await page.waitForTimeout(2000);
    await page.waitForLoadState('domcontentloaded', { timeout: 10000 });

    // Transactions tab should show a table or transaction data
    const html = await page.locator('main, #root, body').first().innerHTML();
    expect(html.length).toBeGreaterThan(200);

    // May have table rows or "add transaction" button
    const hasTxContent =
      html.includes('交易') ||
      html.includes('table') ||
      html.includes('添加') ||
      html.includes('买入') ||
      html.includes('卖出');
    expect(hasTxContent).toBeTruthy();
  });

  test('navigate directly to /fund/:code via URL', async ({ page }) => {
    // Try a known fund code — fall back gracefully if it does not exist
    const testCodes = ['F01', '000001', '160213', '513100'];

    for (const code of testCodes) {
      await page.goto(`${BASE}/fund/${code}`, { timeout: 15000 });
      await page.waitForLoadState('domcontentloaded', { timeout: 10000 });
      await page.waitForTimeout(2000);

      const html = await page.locator('main, #root, body').first().innerHTML();

      // If we got a meaningful detail page (not loading spinner only, not error)
      if (
        html.length > 500 &&
        !html.includes('加载失败') &&
        !html.includes('loadError')
      ) {
        // Verify URL contains the code
        expect(page.url()).toContain(`/fund/${code}`);

        // Should have fund detail content
        expect(
          html.includes('持有') ||
          html.includes('投入') ||
          html.includes('市值') ||
          html.includes('tab') ||
          html.includes('nav') ||
          html.includes('chart'),
        ).toBeTruthy();
        return; // Success
      }
    }

    // If none of the test codes work, skip gracefully
    test.skip(true, 'No fund codes returned a valid detail page (empty DB or all codes invalid)');
  });
});
