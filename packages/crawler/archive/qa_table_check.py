"""Check if table is actually rendered on transactions page"""
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

    # Navigate to 006479
    navigated = page.evaluate("""() => {
        for (const b of document.querySelectorAll('button[class*="menu-button"]')) {
            if ((b.textContent || '').includes('006479')) {
                b.click(); return b.textContent?.substring(0, 30);
            }
        }
        return null;
    }""")
    page.wait_for_timeout(3000)
    page.wait_for_load_state('networkidle', timeout=15000)
    print(f"Navigated to: {navigated}")

    # What's the current page content?
    h1 = page.evaluate("document.querySelector('h1')?.textContent?.trim()")
    print(f"H1: {h1}")

    # Count ALL tables on the page
    table_count = page.evaluate("() => document.querySelectorAll('table').length")
    print(f"Tables on page: {table_count}")

    # Find transaction table specifically
    tx_table = page.evaluate("""() => {
        const tables = document.querySelectorAll('table');
        for (const t of tables) {
            const headers = Array.from(t.querySelectorAll('th')).map(th => th.textContent?.trim());
            return {
                headers,
                rows: t.querySelectorAll('tbody tr').length,
                visible: t.offsetParent !== null,
            };
        }
        return null;
    }""")
    print(f"Tx table: {json.dumps(tx_table, ensure_ascii=False)}")

    # The transactions tab might need clicking first
    tabs = page.evaluate("""() => {
        return Array.from(document.querySelectorAll('[role="tab"]'))
            .map(t => ({text: t.textContent?.trim(), selected: t.getAttribute('aria-selected')}));
    }""")
    print(f"\nTabs: {json.dumps(tabs, ensure_ascii=False)}")

    # Click the transactions tab
    clicked_tx = page.evaluate("""() => {
        for (const t of document.querySelectorAll('[role="tab"]')) {
            const txt = t.textContent?.trim() || '';
            if (txt.includes('交易记录')) {
                t.click(); return txt;
            }
        }
        return null;
    }""")
    page.wait_for_timeout(2000)
    print(f"\nClicked tab: {clicked_tx}")

    # Check after click
    h1_after = page.evaluate("document.querySelector('h1')?.textContent?.trim()")
    print(f"H1 after click: {h1_after}")

    # Find all tables again
    all_tables_after = page.evaluate("""() => {
        const tables = document.querySelectorAll('table');
        const result = [];
        for (const t of tables) {
            const ths = Array.from(t.querySelectorAll('th')).map(th => th.textContent?.trim());
            const rows = t.querySelectorAll('tbody tr').length;
            const area = t.closest('[class*="p-0"]') || t.closest('[class*="LayerCard"]');
            result.push({
                headers: ths,
                rows,
                areaClass: area?.className?.substring(0, 60) || 'none',
            });
        }
        return result;
    }""")
    print(f"All tables after tx tab click: {json.dumps(all_tables_after, ensure_ascii=False, indent=2)}")

    # Look for the action buttons (UserIcon/RobotIcon, TrashIcon) in table cells
    action_cells = page.evaluate("""() => {
        const result = [];
        const tables = document.querySelectorAll('table');
        for (const t of tables) {
            const rows = t.querySelectorAll('tbody tr');
            rows.forEach((row, ri) => {
                const cells = row.querySelectorAll('td');
                const lastCell = cells[cells.length - 1];
                if (lastCell) {
                    const buttons = lastCell.querySelectorAll('button');
                    if (buttons.length > 0) {
                        const btnInfo = Array.from(buttons).map(b => ({
                            innerHTML: (b.innerHTML || '').substring(0, 60),
                            class: (b.className || '').substring(0, 40),
                        }));
                        result.push({row: ri, buttons: btnInfo});
                    }
                }
            });
        }
        return result.slice(0, 5);
    }""")
    print(f"\nAction cells: {json.dumps(action_cells, ensure_ascii=False, indent=2)}")

    browser.close()
