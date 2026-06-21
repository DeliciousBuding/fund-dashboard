"""Edge case fix: test heldOnly, toggle, delete, and body bg warnings"""
import json, time
from playwright.sync_api import sync_playwright

BASE = "http://localhost:5205"

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    ctx = browser.new_context(viewport={"width": 1440, "height": 900})
    page = ctx.new_page()

    page.goto(BASE, timeout=25000)
    page.wait_for_load_state('networkidle', timeout=25000)
    page.wait_for_timeout(3000)

    # ── 1. heldOnly switch investigation ──
    print("=== HELDONLY SWITCH ===")
    switch_info = page.evaluate("""() => {
        const all = document.querySelectorAll('*');
        const results = [];
        for (const el of all) {
            const attrs = Array.from(el.attributes).map(a => a.name);
            if (attrs.includes('data-state') || el.getAttribute('role') === 'switch') {
                results.push({
                    tag: el.tagName,
                    role: el.getAttribute('role'),
                    'data-state': el.getAttribute('data-state'),
                    class: el.className?.substring(0, 100),
                    parentClass: el.parentElement?.className?.substring(0, 80),
                    parentTag: el.parentElement?.tagName,
                });
            }
        }
        return results;
    }""")
    print(json.dumps(switch_info, ensure_ascii=False, indent=2))

    # Find the switch by context: "仅持仓"/"全部" text nearby
    switch_by_text = page.evaluate("""() => {
        const all = document.querySelectorAll('*');
        for (const el of all) {
            const t = el.textContent || '';
            if (t.includes('仅持仓') || t.includes('全部')) {
                let parent = el.parentElement;
                for (let i = 0; i < 5 && parent; i++) {
                    const buttons = parent.querySelectorAll('button, [role="switch"]');
                    for (const b of buttons) {
                        // Find the switch toggle
                        const inner = b.querySelector('[data-state]');
                        if (inner) {
                            inner.click();
                            return {
                                method: 'text-proximity',
                                tag: inner.tagName,
                                state_before: inner.getAttribute('data-state'),
                            };
                        }
                    }
                    parent = parent.parentElement;
                }
            }
        }
        return {method: 'not-found'};
    }""")
    print(f"\nSwitch by text: {json.dumps(switch_by_text, ensure_ascii=False)}")
    page.wait_for_timeout(1000)

    # Check for any visual change
    sidebar_text = page.evaluate("""() => {
        const sidebar = document.querySelector('aside, [class*="sidebar"], [class*="Sidebar"]');
        if (!sidebar) return 'no sidebar';
        return sidebar.textContent?.substring(0, 500);
    }""")
    print(f"\nSidebar after toggle: {sidebar_text[:200] if sidebar_text else 'empty'}")

    # ── 2. Navigate to fund with buy transactions for toggle/delete test ──
    # First go back to overview
    page.evaluate("""() => {
        for (const b of document.querySelectorAll('button[class*="menu-button"]')) {
            if ((b.textContent || '').includes('组合总览')) {
                b.click(); return true;
            }
        }
    }""")
    page.wait_for_timeout(2000)
    page.wait_for_load_state('networkidle', timeout=15000)

    # Find a fund that HAS buy transactions (look for "手动" or "定投" badges)
    fund_with_buys = page.evaluate("""() => {
        // Click first fund with positive PnL (likely has active buy tx)
        for (const b of document.querySelectorAll('button[class*="menu-button"]')) {
            const t = (b.textContent || '').trim();
            // Look for fund with +[number] at end (positive PnL means bought lower)
            if (t.match(/\\+\\d+$/) && t.length > 10 && !t.includes('总览')) {
                b.click();
                return t.substring(0, 50);
            }
        }
        return null;
    }""")

    if fund_with_buys:
        page.wait_for_timeout(3000)
        page.wait_for_load_state('networkidle', timeout=15000)
        print(f"\n=== FUND WITH BUYS: {fund_with_buys} ===")

        # Click transactions tab
        page.evaluate("""() => {
            for (const t of document.querySelectorAll('[role="tab"]')) {
                if ((t.textContent || '').includes('交易记录')) {
                    t.click(); return true;
                }
            }
        }""")
        page.wait_for_timeout(1500)

        # Now find toggle and delete buttons
        tx_buttons = page.evaluate("""() => {
            const buttons = document.querySelectorAll('button');
            const info = [];
            for (const b of buttons) {
                const t = b.textContent?.trim() || '';
                const title = b.getAttribute('title') || '';
                if (title.includes('切换') || title.includes('删除')) {
                    info.push({
                        text: t.substring(0, 30),
                        title,
                        visible: b.offsetParent !== null,
                    });
                }
            }
            return info;
        }""")
        print(f"Toggle/Delete buttons: {json.dumps(tx_buttons, ensure_ascii=False)}")

        # Actually toggle
        toggle_result = page.evaluate("""() => {
            for (const b of document.querySelectorAll('button')) {
                const title = b.getAttribute('title') || '';
                if (title.includes('切换为手动') || title.includes('切换为定投')) {
                    b.click();
                    return title;
                }
            }
            return null;
        }""")
        page.wait_for_timeout(1500)
        print(f"Toggle result: {toggle_result}")

        # Delete
        delete_result = page.evaluate("""() => {
            for (const b of document.querySelectorAll('button')) {
                const title = b.getAttribute('title') || '';
                if (title.includes('删除')) {
                    b.click();
                    return title;
                }
            }
            return null;
        }""")
        page.wait_for_timeout(1500)
        print(f"Delete clicked: {delete_result}")

    else:
        print("No fund with positive PnL found")

    # ── 3. Body background investigation ──
    print("\n=== BODY BACKGROUND ===")
    bg_info = page.evaluate("""() => {
        const dm = document.documentElement.getAttribute('data-mode');
        const body = getComputedStyle(document.body);
        const html = getComputedStyle(document.documentElement);
        const main = document.querySelector('main');
        const mainBg = main ? getComputedStyle(main).backgroundColor : 'no main';
        const sidebar = document.querySelector('aside, [class*="sidebar"]');
        const sidebarBg = sidebar ? getComputedStyle(sidebar).backgroundColor : 'no sidebar';

        return {
            mode: dm,
            htmlBg: html.backgroundColor,
            bodyBg: body.backgroundColor,
            mainBg,
            sidebarBg,
            htmlClass: document.documentElement.className,
            bodyClass: document.body.className,
        };
    }""")
    print(json.dumps(bg_info, ensure_ascii=False, indent=2))

    # Toggle dark mode and check again
    page.evaluate("""() => {
        for (const b of document.querySelectorAll('button')) {
            if (b.getAttribute('data-base-ui-tooltip-trigger') !== null) {
                b.click(); return;
            }
        }
    }""")
    page.wait_for_timeout(1500)

    bg_dark = page.evaluate("""() => {
        const dm = document.documentElement.getAttribute('data-mode');
        const body = getComputedStyle(document.body);
        const html = getComputedStyle(document.documentElement);
        const main = document.querySelector('main');
        const mainBg = main ? getComputedStyle(main).backgroundColor : 'no main';

        return {mode: dm, htmlBg: html.backgroundColor, bodyBg: body.backgroundColor, mainBg};
    }""")
    print(f"\nDark mode bg: {json.dumps(bg_dark, ensure_ascii=False, indent=2)}")

    browser.close()
