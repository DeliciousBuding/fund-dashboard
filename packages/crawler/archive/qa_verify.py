"""Verify toggle/delete on a fund with buy transactions"""
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

    # Get ALL funds with their PnL from sidebar
    funds_info = page.evaluate("""() => {
        const result = [];
        for (const b of document.querySelectorAll('button[class*="menu-button"]')) {
            const t = (b.textContent || '').trim();
            if (t.length > 5 && !t.includes('总览')) {
                result.push({text: t.substring(0, 60), isCategory: /^[\\u4e00-\\u9fa5]+\\s*\\(/.test(t)});
            }
        }
        return result;
    }""")
    print("=== ALL SIDEBAR FUNDS ===")
    for f in funds_info:
        print(f"  {'CAT' if f['isCategory'] else 'FND'}: {f['text']}")

    # Instead of checking the sidebar text, let's navigate to a fund and check its transactions
    # via the API directly
    import urllib.request, json as j
    funds_resp = urllib.request.urlopen("http://localhost:8765/api/funds")
    all_funds = j.loads(funds_resp.read())

    # Find a fund with held shares > 0 (has transactions)
    held_funds = [f for f in all_funds if f.get('held_shares', 0) > 0.001]
    print(f"\n=== HELD FUNDS: {len(held_funds)} ===")
    for f in held_funds[:5]:
        print(f"  {f['code']} {f['name']}: shares={f['held_shares']}, pnl={f.get('unrealized_pnl')}")

    # Pick the first held fund and check its detail
    if held_funds:
        fund = held_funds[0]
        detail_resp = urllib.request.urlopen(f"http://localhost:8765/api/funds/{fund['code']}")
        detail = j.loads(detail_resp.read())
        txs = detail.get('transactions', [])
        buy_txs = [t for t in txs if t.get('direction') == 'buy']
        sell_txs = [t for t in txs if t.get('direction') == 'sell']

        print(f"\n=== FUND: {fund['code']} {fund['name']} ===")
        print(f"  TX count: {len(txs)}, Buy: {len(buy_txs)}, Sell: {len(sell_txs)}")
        if buy_txs:
            print(f"  First buy: type={buy_txs[0].get('trade_type')}, amount={buy_txs[0].get('amount')}")
        if sell_txs:
            print(f"  First sell: type={sell_txs[0].get('trade_type')}, amount={sell_txs[0].get('amount')}")

    # Navigate to this fund and check the transactions tab
    fund_code = held_funds[0]['code'] if held_funds else None
    if fund_code:
        # Navigate by clicking sidebar
        page.evaluate(f"""(code) => {{
            for (const b of document.querySelectorAll('button[class*="menu-button"]')) {{
                const t = b.textContent || '';
                if (t.includes(code)) {{
                    b.click();
                    return true;
                }}
            }}
            return false;
        }}""", fund_code)
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

        # List all buttons with titles
        all_btn_titles = page.evaluate("""() => {
            const result = [];
            for (const b of document.querySelectorAll('button')) {
                const title = b.getAttribute('title') || '';
                if (title && b.offsetParent !== null) {
                    result.push({title, text: (b.textContent || '').trim().substring(0, 20)});
                }
            }
            return result;
        }""")
        print(f"\n=== BUTTONS WITH TITLES ON TX PAGE ===")
        for b in all_btn_titles:
            print(f"  title='{b['title']}' text='{b['text']}'")

    # Also test the heldOnly switch properly
    print("\n=== HELDONLY SWITCH (fixed selector) ===")
    page.goto(BASE, timeout=25000)
    page.wait_for_load_state('networkidle', timeout=25000)
    page.wait_for_timeout(3000)

    switch_toggled = page.evaluate("""() => {
        const sw = document.querySelector('[role="switch"]');
        if (sw) {
            sw.click();
            return 'clicked';
        }
        return 'not found';
    }""")
    page.wait_for_timeout(1500)
    print(f"  Switch clicked: {switch_toggled}")
    # Toggle back
    page.evaluate("document.querySelector('[role=\"switch\"]')?.click()")
    page.wait_for_timeout(500)
    print(f"  Switch toggled back")

    browser.close()
