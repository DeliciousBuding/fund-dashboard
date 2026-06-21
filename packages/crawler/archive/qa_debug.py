"""Final targeted button inspection"""
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

    # Inspect button 0 (the SVG-only button in sidebar header - likely dark mode)
    btn0 = page.evaluate("""() => {
        const btn = document.querySelectorAll('button')[0];
        if (!btn) return {error: 'no button 0'};
        return {
            outerHTML: btn.outerHTML?.substring(0, 400),
            onClick: btn.onclick?.toString()?.substring(0, 200),
            allAttrs: Array.from(btn.attributes).map(a => a.name + '=' + a.value),
            parentHTML: btn.parentElement?.outerHTML?.substring(0, 500),
        };
    }""")
    print("=== BUTTON #0 (Dark mode?) ===")
    print(json.dumps(btn0, ensure_ascii=False, indent=2))

    # Find all elements with data-mode or dark/light references
    mode_elements = page.evaluate("""() => {
        const results = [];
        const all = document.querySelectorAll('*');
        for (const el of all) {
            const attrs = Array.from(el.attributes).map(a => a.name);
            if (attrs.some(a => a.includes('mode') || a.includes('dark') || a.includes('theme'))) {
                results.push({
                    tag: el.tagName,
                    attrs: Object.fromEntries(Array.from(el.attributes).map(a => [a.name, a.value])),
                    text: el.textContent?.trim()?.substring(0, 40),
                });
            }
        }
        return results;
    }""")
    print(f"\n=== DATA-MODE/DARK/THEME ATTRIBUTES ===")
    for e in mode_elements:
        print(f"  {e['tag']}: {json.dumps(e['attrs'], ensure_ascii=False)} text='{e['text']}'")

    # Check the useDarkMode hook effects
    dark_in_html = page.evaluate("() => document.documentElement.getAttribute('data-mode')")
    print(f"\n=== CURRENT DATA-MODE ===")
    print(f"  <html data-mode>='{dark_in_html}'")

    # Try clicking the first button (which should be dark mode toggle)
    btn0_click = page.evaluate("""() => {
        const btn = document.querySelectorAll('button')[0];
        if (btn) {
            btn.click();
            return true;
        }
        return false;
    }""")
    page.wait_for_timeout(1500)
    after_mode = page.evaluate("document.documentElement.getAttribute('data-mode')")
    print(f"\n=== AFTER CLICKING BUTTON #0 ===")
    print(f"  data-mode: {dark_in_html} -> {after_mode}")
    print(f"  toggled: {dark_in_html != after_mode}")

    # Toggle back
    page.evaluate("() => { document.querySelectorAll('button')[0].click(); }")
    page.wait_for_timeout(1500)
    final_mode = page.evaluate("document.documentElement.getAttribute('data-mode')")
    print(f"  After toggle back: {final_mode}")

    # Now navigate to a fund detail - use a fund button by its class
    fund_nav = page.evaluate("""() => {
        const menuButtons = document.querySelectorAll('[class*="menu-button"], [class*="MenuButton"]');
        for (const btn of menuButtons) {
            const text = btn.textContent?.trim() || '';
            // Find a real fund (has +/- number pattern, not category text)
            if (text.length > 10 && text.length < 50 &&
                !text.includes('组合总览') && !text.includes('纳指总览') &&
                !text.includes('纳斯达克') && !text.includes('科技主题') &&
                !text.includes('红利价值') && !text.includes('黄金商品') &&
                !text.includes('债券存单') && !text.includes('海外其他') &&
                !text.includes('货币基金') && !text.includes('其他')) {
                btn.click();
                return {clicked: true, text: text.substring(0, 40)};
            }
        }
        return {clicked: false};
    }""")
    print(f"\n=== NAVIGATE TO FUND ===")
    print(json.dumps(fund_nav, ensure_ascii=False))

    if fund_nav['clicked']:
        page.wait_for_timeout(3000)
        page.wait_for_load_state('networkidle', timeout=15000)
        # Now check what's on the page
        current = page.evaluate("""() => {
            const h1 = document.querySelector('h1')?.textContent?.trim();
            const tabs = Array.from(document.querySelectorAll('[role="tab"]')).map(t => t.textContent?.trim());
            const buttons = Array.from(document.querySelectorAll('button')).map(b => ({
                text: b.textContent?.trim()?.substring(0, 40),
                title: b.getAttribute('title') || '',
                visible: b.offsetParent !== null,
            }));
            return {h1, tabs, buttons: buttons.filter(b => b.visible && b.text)};
        }""")
        print(f"\n=== FUND DETAIL PAGE ===")
        print(json.dumps(current, ensure_ascii=False, indent=2))
    else:
        # Try alternative navigation
        alt_nav = page.evaluate("""() => {
            // Find any button in sidebar that's a fund (has +/- numbers)
            const allText = [];
            const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_ELEMENT);
            let node;
            while (node = walker.nextNode()) {
                if (node.tagName === 'BUTTON') {
                    const t = node.textContent?.trim() || '';
                    if (t && t.length > 5 && t.length < 70) {
                        allText.push({text: t, hasNum: /[+-]\\d/.test(t), isCategory: /^[\\u4e00-\\u9fff]+\\s*\\(\\d+/.test(t)});
                    }
                }
            }
            return allText.slice(0, 30);
        }""")
        print(f"\n=== ALL SIDEBAR BUTTON TEXTS ===")
        print(json.dumps(alt_nav, ensure_ascii=False, indent=2))

    browser.close()
