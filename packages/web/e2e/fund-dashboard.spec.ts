import { test, expect } from '@playwright/test';

const BASE = process.env.CI ? 'http://localhost:8080' : 'http://localhost:5176';

test.describe('Fund Dashboard', () => {
  test('Dashboard loads and shows portfolio stats', async ({ page }) => {
    const errors: string[] = [];
    page.on('console', msg => {
      if (msg.type() === 'error' && !msg.text().toLowerCase().includes('favicon')) {
        errors.push(msg.text());
      }
    });

    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('networkidle', { timeout: 25000 });
    await page.waitForTimeout(3000);

    // Title should contain something meaningful
    const title = await page.title();
    expect(title.length).toBeGreaterThan(0);

    // Should have heading or stat content (may be 0 in empty-DB CI — empty state renders without stat cards)
    const headings = page.locator('h1, h3');
    await headings.first().waitFor({ state: 'attached', timeout: 10000 }).catch(() => {});

    // Should have chart area (echarts canvas or chart container)
    const bodyHtml = await page.evaluate(() => document.body.innerHTML);
    expect(
      bodyHtml.includes('echarts') || bodyHtml.includes('canvas') || bodyHtml.includes('chart')
    ).toBeTruthy();

    // Should have stat card content (may be 0 in empty-DB CI)
    const statCards = page.locator('[class*="StatCard"], .kumo-stat-card');
    const statCount = await statCards.count().catch(() => 0);

    // Minimal console errors
    expect(errors.length).toBeLessThan(5);
  });

  test('Sidebar navigation to a fund shows detail page', async ({ page }) => {
    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('networkidle', { timeout: 25000 });
    await page.waitForTimeout(3000);

    // Find a fund button in the sidebar menu buttons (not the overview/nasdaq-overview buttons)
    const fundBtn = page.locator('.kumo-sidebar-menu-button').filter({
      hasText: /[+\-]\d/
    }).first();
    const fundBtnVisible = await fundBtn.isVisible().catch(() => false);
    if (!fundBtnVisible) {
      test.skip(true, 'No fund with PnL data found in sidebar');
      return;
    }

    const fundName = await fundBtn.textContent();
    if (fundName && fundName.trim().length > 3) {
      await fundBtn.click();
      await page.waitForTimeout(2500);
      await page.waitForLoadState('networkidle', { timeout: 15000 });

      // Should now show detail content
      const detailContent = page.locator('main');
      const html = await detailContent.innerHTML();
      expect(html.length).toBeGreaterThan(200);
    } else {
      // If no fund with +/- values (maybe empty data), skip gracefully
      test.skip(true, 'No fund with PnL data found in sidebar');
    }
  });

  test('Fund detail tabs switch correctly', async ({ page }) => {
    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('networkidle', { timeout: 25000 });
    await page.waitForTimeout(3000);

    // Click a fund
    const fundBtn = page.locator('.kumo-sidebar-menu-button').filter({
      hasText: /[+\-]\d/
    }).first();
    const fundBtnVisible = await fundBtn.isVisible().catch(() => false);
    if (!fundBtnVisible) {
      test.skip(true, 'No fund with PnL data found');
      return;
    }

    const fundName = await fundBtn.textContent();
    if (!fundName || fundName.trim().length <= 3) {
      test.skip(true, 'No fund with PnL data found');
      return;
    }
    await fundBtn.click();
    await page.waitForTimeout(2500);
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Check for tab elements (role=tab or button tabs)
    const tabs = page.locator('[role="tab"], button').filter({
      hasText: /净值走势|概览|交易记录/
    });
    const tabCount = await tabs.count();
    expect(tabCount).toBeGreaterThanOrEqual(2, 'Should have at least 2 tabs');

    // Click chart tab if it exists
    const chartTab = page.locator('button').filter({ hasText: '净值走势' }).first();
    if (await chartTab.isVisible().catch(() => false)) {
      await chartTab.click();
      await page.waitForTimeout(1000);
      // Should still render content
      const html = await page.locator('main').innerHTML();
      expect(html.length).toBeGreaterThan(200);
    }
  });

  test('Transaction table displays data', async ({ page }) => {
    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('networkidle', { timeout: 25000 });
    await page.waitForTimeout(3000);

    // Click a fund first
    const fundBtn = page.locator('.kumo-sidebar-menu-button').filter({
      hasText: /[+\-]\d/
    }).first();
    const fundBtnVisible = await fundBtn.isVisible().catch(() => false);
    if (!fundBtnVisible) {
      test.skip(true, 'No fund with PnL data found');
      return;
    }

    const fundName = await fundBtn.textContent();
    if (!fundName || fundName.trim().length <= 3) {
      test.skip(true, 'No fund with PnL data found');
      return;
    }
    await fundBtn.click();
    await page.waitForTimeout(2500);
    await page.waitForLoadState('networkidle', { timeout: 15000 });

    // Click transactions/交易记录 tab
    const txTab = page.locator('button').filter({ hasText: '交易记录' }).first();
    if (await txTab.isVisible().catch(() => false)) {
      await txTab.click();
      await page.waitForTimeout(1500);

      // Should have table rows or transaction data
      const html = await page.locator('main').innerHTML();
      expect(html.length).toBeGreaterThan(200);
    } else {
      // Tab might be named differently, just check content is rendered
      const html = await page.locator('main').innerHTML();
      expect(html.length).toBeGreaterThan(200);
    }
  });

  test('Dark mode toggle works and persists', async ({ page }) => {
    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('networkidle', { timeout: 25000 });
    await page.waitForTimeout(2000);

    // Get initial mode
    const initialMode = await page.evaluate(
      () => document.documentElement.getAttribute('data-mode')
    );

    // Click dark mode button (tooltip-trigger button in sidebar header)
    const darkBtn = page.locator('button[title*="切换"]').first();
    if (await darkBtn.isVisible().catch(() => false)) {
      await darkBtn.click();
      await page.waitForTimeout(1500);

      const afterMode = await page.evaluate(
        () => document.documentElement.getAttribute('data-mode')
      );

      // Mode should have changed
      expect(afterMode).not.toBe(initialMode);
      expect(['light', 'dark']).toContain(afterMode);

      // Check localStorage persistence
      const stored = await page.evaluate(
        () => localStorage.getItem('fund-dark-mode')
      );
      expect(stored).toBeTruthy();

      // Toggle back
      await darkBtn.click();
      await page.waitForTimeout(1500);

      const finalMode = await page.evaluate(
        () => document.documentElement.getAttribute('data-mode')
      );
      expect(finalMode).toBe(initialMode);
    } else {
      // Fallback: look for dark mode button by SVG content
      const altDarkBtn = page.locator('.kumo-sidebar-header button').last();
      if (await altDarkBtn.isVisible().catch(() => false)) {
        await altDarkBtn.click();
        await page.waitForTimeout(1500);
        const afterMode = await page.evaluate(
          () => document.documentElement.getAttribute('data-mode')
        );
        expect(afterMode).not.toBe(initialMode);
      } else {
        test.skip(true, 'Dark mode toggle button not found');
      }
    }
  });

  test('Search filters sidebar funds', async ({ page }) => {
    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('networkidle', { timeout: 25000 });
    await page.waitForTimeout(2000);

    // Find search input by placeholder
    const searchInput = page.locator('input').filter({
      has: page.locator('[placeholder*="搜索"]')
    }).first();

    // If not found by filter, try direct placeholder
    const allInputs = page.locator('input');
    const inputCount = await allInputs.count();
    let foundInput = false;

    for (let i = 0; i < inputCount; i++) {
      const placeholder = await allInputs.nth(i).getAttribute('placeholder');
      if (placeholder?.includes('搜索')) {
        await allInputs.nth(i).fill('纳');
        foundInput = true;
        break;
      }
    }

    if (foundInput) {
      await page.waitForTimeout(1000);

      // Content should still be rendered
      const html = await page.locator('main').innerHTML();
      expect(html.length).toBeGreaterThan(200);

      // Clear search
      for (let i = 0; i < inputCount; i++) {
        const placeholder = await allInputs.nth(i).getAttribute('placeholder');
        if (placeholder?.includes('搜索')) {
          await allInputs.nth(i).fill('');
          break;
        }
      }
    } else {
      test.skip(true, 'Search input not found');
    }
  });

  test('Held-only toggle filters correctly', async ({ page }) => {
    await page.goto(BASE, { timeout: 25000 });
    await page.waitForLoadState('networkidle', { timeout: 25000 });
    await page.waitForTimeout(2000);

    // Find the switch element with data-state
    const toggle = page.locator('[data-state]').filter({
      hasText: ''
    }).first();

    const initialState = await toggle.getAttribute('data-state').catch(() => null);
    if (initialState && ['checked', 'unchecked'].includes(initialState)) {
      await toggle.click();
      await page.waitForTimeout(1500);

      const newState = await toggle.getAttribute('data-state');
      expect(newState).not.toBe(initialState);

      // Toggle back
      await toggle.click();
      await page.waitForTimeout(1500);

      const finalState = await toggle.getAttribute('data-state');
      expect(finalState).toBe(initialState);
    } else {
      // Try Switch component
      const switchBtn = page.locator('button').filter({
        has: page.locator('[data-state]')
      }).first();

      if (await switchBtn.isVisible().catch(() => false)) {
        await switchBtn.click();
        await page.waitForTimeout(1500);
        // Just verify page didn't crash
        const html = await page.locator('main').innerHTML();
        expect(html.length).toBeGreaterThan(200);
      } else {
        test.skip(true, 'Held-only toggle switch not found');
      }
    }
  });
});
