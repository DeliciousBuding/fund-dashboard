"""Final investigation of button rendering"""
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

    # Navigate to 006479 (has buy tx)
    page.evaluate("""() => {
        for (const b of document.querySelectorAll('button[class*="menu-button"]')) {
            if ((b.textContent || '').includes('006479')) {
                b.click(); return true;
            }
        }
        // fallback
        return false;
    }""")
    page.wait_for_timeout(3000)
    page.wait_for_load_state('networkidle', timeout=15000)

    # Click transactions tab
    page.evaluate("""() => {
        for (const t of document.querySelectorAll('[role="tab"]')) {
            if ((t.textContent || '').includes('交易记录')) {
                t.click(); return true;
            }
        }
    }""")
    page.wait_for_timeout(2000)

    # Get ALL buttons in the transactions area - dump their full attributes
    tx_buttons = page.evaluate("""() => {
        const result = [];
        // Target the table area specifically
        const tableArea = document.querySelector('table')?.closest('div') || document.body;
        const buttons = tableArea.querySelectorAll('button');
        for (const b of buttons) {
            const allAttrs = {};
            for (const a of b.attributes) {
                allAttrs[a.name] = a.value;
            }
            result.push({
                text: b.textContent?.trim()?.substring(0, 30),
                innerHTML: b.innerHTML?.substring(0, 200),
                attrs: allAttrs,
            });
        }
        return result;
    }""")
    print("=== TX AREA BUTTONS ===")
    for b in tx_buttons:
        print(f"  text='{b['text']}' html='{b['innerHTML'][:80]}' attrs={json.dumps(b['attrs'], ensure_ascii=False)[:200]}")

    # Also check: do any svg elements have titles?
    svg_info = page.evaluate("""() => {
        const result = [];
        const tableArea = document.querySelector('table')?.closest('div') || document.body;
        const svgs = tableArea.querySelectorAll('svg');
        for (const s of svgs) {
            const title = s.querySelector('title');
            const parent = s.parentElement;
            result.push({
                svgTitle: title?.textContent,
                parentTag: parent?.tagName,
                parentClass: parent?.className?.substring(0, 60),
            });
        }
        return result.slice(0, 20);
    }""")
    print(f"\n=== SVG ELEMENTS IN TX AREA: {len(svg_info)} ===")
    for s in svg_info[:10]:
        print(f"  svgTitle='{s['svgTitle']}' parent={s['parentTag']} class={s['parentClass']}")

    # Now try clicking the toggle button by its position in the table row
    toggle_clicked = page.evaluate("""() => {
        const rows = document.querySelectorAll('table tr');
        for (const row of rows) {
            const badges = row.querySelectorAll('[class*="Badge"], [class*="badge"]');
            let hasAuto = false;
            for (const b of badges) {
                if ((b.textContent || '').includes('定投')) {
                    hasAuto = true;
                    break;
                }
            }
            if (hasAuto) {
                // Find the action buttons in this row (last cell)
                const cells = row.querySelectorAll('td, th');
                const lastCell = cells[cells.length - 1];
                if (lastCell) {
                    const buttons = lastCell.querySelectorAll('button');
                    // Click the first button (toggle) in this row
                    if (buttons.length >= 1) {
                        buttons[0].click();
                        return {clicked: true, btnCount: buttons.length};
                    }
                }
            }
        }
        return {clicked: false};
    }""")
    page.wait_for_timeout(2000)
    print(f"\n=== TOGGLE CLICK RESULT ===")
    print(json.dumps(toggle_clicked, ensure_ascii=False))

    # Check if page reloaded after toggle
    h1 = page.evaluate("document.querySelector('h1')?.textContent?.trim()")
    print(f"  After toggle: h1={h1}")

    browser.close()
