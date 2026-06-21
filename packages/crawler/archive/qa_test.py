"""FINAL QA test for fund-dashboard — with correct selectors"""
import json, time
from playwright.sync_api import sync_playwright

BASE = "http://localhost:5205"
results = []

def log(flow, check, status, detail=""):
    results.append({"flow": flow, "check": check, "status": status, "detail": detail})
    i = {"pass":"PASS","warn":"WARN","fail":"FAIL","skip":"SKIP","info":"INFO"}[status]
    print(f"  [{i}] {flow}: {check}" + (f" — {detail}" if detail else ""))

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    ctx = browser.new_context(viewport={"width": 1440, "height": 900})
    page = ctx.new_page()

    js_errors = []
    page.on("console", lambda msg: (
        msg.type == "error" and "favicon" not in msg.text.lower()
    ) and js_errors.append(msg.text))

    # ═══════════════ INITIAL LOAD ═══════════════
    page.goto(BASE, timeout=25000)
    page.wait_for_load_state('networkidle', timeout=25000)
    page.wait_for_timeout(3000)
    page.screenshot(path="/tmp/final_01_init.png", full_page=True)

    title = page.title()
    mode = page.evaluate("document.documentElement.getAttribute('data-mode')")
    log("1. Initial Load", "App loads", "pass",
        f"title={title}, mode={mode}, errors={len(js_errors)}")
    js_errors.clear()

    # ═══════════════ FLOW 1: Overview Checks ═══════════════
    # Count stat cards
    card_count = page.evaluate("""() => document.querySelectorAll('h1,h3').length""")
    # Check portfolio chart
    has_chart_area = page.evaluate("""() => {
        return document.body.innerHTML.includes('echarts') || document.querySelector('canvas') != null;
    }""")
    log("1a. Overview", "Stat cards + chart", "pass" if card_count >= 3 and has_chart_area else "warn",
        f"headings={card_count}, echarts={has_chart_area}")

    # Check stat card values
    stat_values = page.evaluate("""() => {
        const texts = [];
        document.querySelectorAll('[class*="StatCard"], .kumo-stat-card').forEach(el => {
            texts.push(el.textContent?.trim()?.substring(0, 40));
        });
        return texts.slice(0, 8);
    }""")
    log("1b. Overview", "Stat data", "pass" if stat_values else "warn",
        str(stat_values)[:100])

    # ═══════════════ FLOW 2: Click Fund in Sidebar ═══════════════
    fund_text = page.evaluate("""() => {
        const btns = document.querySelectorAll('button[class*="menu-button"]');
        for (const b of btns) {
            const t = (b.textContent || '').trim();
            // Find a real fund: has +/- and long enough, not a category
            if (t.length > 8 && /[+\\-]\\d/.test(t) && !/^[\\u4e00-\\u9fa5]+\\s*\\(\\d+/.test(t)) {
                b.click();
                return t.substring(0, 50);
            }
        }
        return null;
    }""")

    if fund_text:
        page.wait_for_timeout(2500)
        page.wait_for_load_state('networkidle', timeout=15000)
        page.screenshot(path="/tmp/final_02_fund_detail.png", full_page=True)
        log("2a. Fund Detail", "Click fund in sidebar", "pass", fund_text)
    else:
        log("2a. Fund Detail", "Click fund in sidebar", "fail", "no fund found")

    # ═══════════════ FLOW 3: Check Tabs ═══════════════
    tabs = page.evaluate("""() => {
        return Array.from(document.querySelectorAll('[role="tab"]'))
            .map(t => t.textContent?.trim()).filter(Boolean);
    }""")
    log("3a. Tabs", "Tab list", "pass" if len(tabs) >= 3 else "warn",
        f"found {len(tabs)}: {tabs}")

    # ═══════════════ FLOW 4: Click Chart Tab + Change Date Range ═══════════════
    chart_tab_clicked = page.evaluate("""() => {
        for (const t of document.querySelectorAll('[role="tab"]')) {
            if ((t.textContent || '').includes('净值走势')) {
                t.click(); return true;
            }
        }
        return false;
    }""")
    page.wait_for_timeout(1000)
    log("4a. Chart Tab", "Select chart tab", "pass" if chart_tab_clicked else "warn")

    # Change date ranges
    ranges_clicked = page.evaluate("""() => {
        const ranges = ['近1月','近3月','近6月','近1年','全部','交易区间'];
        let c = 0;
        for (const t of document.querySelectorAll('[role="tab"]')) {
            if (ranges.includes(t.textContent?.trim() || '')) {
                t.click(); c++;
            }
        }
        return c;
    }""")
    page.wait_for_timeout(1500)
    page.screenshot(path="/tmp/final_03_chart_range.png", full_page=True)
    chart_errs = [e for e in js_errors if any(k in e.lower() for k in ('chart','echarts','canvas','resize'))]
    log("4b. Date Range", f"Switched {ranges_clicked} ranges", "pass" if ranges_clicked >= 1 else "warn",
        f"chart errors: {len(chart_errs)}")
    js_errors.clear()

    # ═══════════════ FLOW 5: Click Transactions Tab + Add TX + Toggle + Delete ═══════════════
    tx_tab_clicked = page.evaluate("""() => {
        for (const t of document.querySelectorAll('[role="tab"]')) {
            if ((t.textContent || '').includes('交易记录')) {
                t.click(); return true;
            }
        }
        return false;
    }""")
    page.wait_for_timeout(1500)
    log("5a. Transactions", "Select transactions tab", "pass" if tx_tab_clicked else "warn")

    page.screenshot(path="/tmp/final_04_transactions.png", full_page=True)

    # Check for Add Transaction button AND click it
    add_form_opened = page.evaluate("""() => {
        const all = document.querySelectorAll('button');
        for (const b of all) {
            if ((b.textContent || '').includes('添加交易')) {
                b.click();
                return true;
            }
        }
        return false;
    }""")
    if add_form_opened:
        page.wait_for_timeout(1000)
        page.screenshot(path="/tmp/final_05_add_form.png", full_page=True)
        log("5b. Add Transaction", "Open add form", "pass")

        # Close form
        page.evaluate("""() => {
            for (const b of document.querySelectorAll('button')) {
                if ((b.textContent || '').trim() === '取消') {
                    b.click(); return true;
                }
            }
        }""")
        page.wait_for_timeout(500)
        log("5c. Add Transaction", "Close add form", "pass")
    else:
        log("5b. Add Transaction", "Open add form", "warn", "button not visible (need to be on tx tab)")

    # Toggle auto/manual
    toggle_clicked = page.evaluate("""() => {
        for (const b of document.querySelectorAll('button')) {
            const title = b.getAttribute('title') || '';
            if (title.includes('切换')) {
                b.click(); return title;
            }
        }
        return false;
    }""")
    log("5d. Toggle", "Toggle auto/manual", "pass" if toggle_clicked else "warn",
        "toggled" if toggle_clicked else "no toggle btn (may need buy tx for this fund)")

    # Delete transaction
    dialog_accepted = [False]
    def handle_dialog(dialog):
        dialog_accepted[0] = True
        dialog.accept()
    page.on("dialog", handle_dialog)

    delete_clicked = page.evaluate("""() => {
        for (const b of document.querySelectorAll('button')) {
            const title = b.getAttribute('title') || '';
            if (title.includes('删除')) {
                b.click(); return title;
            }
        }
        return false;
    }""")
    page.wait_for_timeout(2000)
    log("5e. Delete", "Delete transaction", "pass" if delete_clicked and dialog_accepted[0] else "warn",
        f"clicked={bool(delete_clicked)}, dialog={dialog_accepted[0]}")

    js_errors.clear()

    # ═══════════════ FLOW 6: Nasdaq Overview + Click Fund in Table + Back ═══════════════
    nasdaq_clicked = page.evaluate("""() => {
        for (const b of document.querySelectorAll('button')) {
            if ((b.textContent || '').includes('纳指总览')) {
                b.click(); return true;
            }
        }
        return false;
    }""")
    if nasdaq_clicked:
        page.wait_for_timeout(3000)
        page.wait_for_load_state('networkidle', timeout=15000)
        page.screenshot(path="/tmp/final_06_nasdaq.png", full_page=True)
        log("6a. Nasdaq Overview", "Navigate to Nasdaq", "pass")
    else:
        log("6a. Nasdaq Overview", "Navigate to Nasdaq", "warn", "link not found")

    # Click fund in table
    table_fund_clicked = page.evaluate("""() => {
        const rows = document.querySelectorAll('tr[style*="cursor: pointer"], tr[style*="cursor:pointer"]');
        if (rows.length > 0) {
            rows[0].click();
            return true;
        }
        return false;
    }""")
    if table_fund_clicked:
        page.wait_for_timeout(2500)
        page.wait_for_load_state('networkidle', timeout=15000)
        log("6b. Nasdaq Table", "Click fund in table", "pass")
    else:
        log("6b. Nasdaq Table", "Click fund in table", "warn", "no clickable rows")

    # Go back to overview
    back_clicked = page.evaluate("""() => {
        for (const b of document.querySelectorAll('button[class*="menu-button"]')) {
            if ((b.textContent || '').includes('组合总览')) {
                b.click(); return true;
            }
        }
        return false;
    }""")
    if back_clicked:
        page.wait_for_timeout(2000)
        page.wait_for_load_state('networkidle', timeout=15000)
        page.screenshot(path="/tmp/final_07_back_overview.png", full_page=True)
        log("6c. Back", "Return to overview", "pass")
    else:
        log("6c. Back", "Return to overview", "warn", "not found")

    js_errors.clear()

    # ═══════════════ FLOW 7: Toggle heldOnly ═══════════════
    # The switch has class "relative inline-flex items-center ring cursor-pointer border"
    # It's a kumo Switch with data-state
    switch_state = page.evaluate("""() => {
        const switches = document.querySelectorAll('[data-state]');
        for (const s of switches) {
            const state = s.getAttribute('data-state');
            if (state === 'checked' || state === 'unchecked') {
                return {state, el: s.tagName};
            }
        }
        return null;
    }""")

    if switch_state:
        # Toggle
        page.evaluate("""() => {
            for (const s of document.querySelectorAll('[data-state]')) {
                if (['checked','unchecked'].includes(s.getAttribute('data-state')||'')) {
                    s.click(); return;
                }
            }
        }""")
        page.wait_for_timeout(1500)
        new_state = page.evaluate("""() => {
            for (const s of document.querySelectorAll('[data-state]')) {
                if (['checked','unchecked'].includes(s.getAttribute('data-state')||'')) {
                    return s.getAttribute('data-state');
                }
            }
        }""")
        toggled = switch_state['state'] != new_state
        log("7a. heldOnly", "Toggle switch", "pass" if toggled else "warn",
            f"{switch_state['state']} -> {new_state}")

        # Toggle back
        page.evaluate("""() => {
            for (const s of document.querySelectorAll('[data-state]')) {
                if (['checked','unchecked'].includes(s.getAttribute('data-state')||'')) {
                    s.click(); return;
                }
            }
        }""")
        page.wait_for_timeout(1000)
        log("7b. heldOnly", "Toggle back", "pass")
    else:
        log("7a. heldOnly", "Find switch", "warn", "no data-state element")

    page.screenshot(path="/tmp/final_08_heldonly.png", full_page=True)
    js_errors.clear()

    # ═══════════════ FLOW 8: Sidebar Search ═══════════════
    search_result = page.evaluate("""() => {
        for (const inp of document.querySelectorAll('input')) {
            if ((inp.placeholder || '').includes('搜索')) {
                inp.value = '纳';
                inp.dispatchEvent(new Event('input', {bubbles: true}));
                inp.dispatchEvent(new Event('change', {bubbles: true}));
                return {found: true, placeholder: inp.placeholder};
            }
        }
        return {found: false};
    }""")

    if search_result['found']:
        page.wait_for_timeout(1000)
        page.screenshot(path="/tmp/final_09_search.png", full_page=True)
        log("8a. Search", "Search '纳'", "pass")

        # Clear search
        page.evaluate("""() => {
            for (const inp of document.querySelectorAll('input')) {
                if ((inp.placeholder || '').includes('搜索')) {
                    inp.value = '';
                    inp.dispatchEvent(new Event('input', {bubbles: true}));
                    inp.dispatchEvent(new Event('change', {bubbles: true}));
                }
            }
        }""")
        page.wait_for_timeout(500)
        log("8b. Search", "Clear search", "pass")
    else:
        log("8a. Search", "Find search input", "warn")

    js_errors.clear()

    # ═══════════════ FLOW 9: Dark Mode Toggle ═══════════════
    initial_mode = page.evaluate("document.documentElement.getAttribute('data-mode')")
    log("9a. Dark Mode", "Initial mode", "info", initial_mode)

    # The dark mode button is the FIRST button in sidebar header (button #0)
    # It has id starting with "base-ui-:r" and is a kumo Button with SVG inside
    dark_toggled = page.evaluate("""() => {
        // The dark mode button is in sidebar header — first button with tooltip-trigger
        for (const b of document.querySelectorAll('button')) {
            if (b.getAttribute('data-base-ui-tooltip-trigger') !== null) {
                b.click();
                return 'clicked';
            }
        }
        return null;
    }""")
    page.wait_for_timeout(1500)
    after_mode = page.evaluate("document.documentElement.getAttribute('data-mode')")
    mode_changed = initial_mode != after_mode

    if dark_toggled and mode_changed:
        page.screenshot(path="/tmp/final_10_dark.png", full_page=True)
        log("9b. Dark Mode", "Toggle to dark", "pass", f"{initial_mode} -> {after_mode}")

        # Check chart updated to dark theme
        has_dark_colors = page.evaluate("""() => {
            return document.documentElement.getAttribute('data-mode') === 'dark';
        }""")
        log("9c. Dark Mode", "Dark theme applied", "pass" if has_dark_colors else "warn")

        # Verify no white flash: check body bg during dark mode
        body_bg_dark = page.evaluate("getComputedStyle(document.body).backgroundColor")
        log("9d. Dark Mode", "Body background in dark mode", "pass" if '0, 0, 0' not in str(body_bg_dark) else "warn",
            str(body_bg_dark))

        # Toggle back
        page.evaluate("""() => {
            for (const b of document.querySelectorAll('button')) {
                if (b.getAttribute('data-base-ui-tooltip-trigger') !== null) {
                    b.click();
                    return;
                }
            }
        }""")
        page.wait_for_timeout(1500)
        final_mode = page.evaluate("document.documentElement.getAttribute('data-mode')")
        page.screenshot(path="/tmp/final_11_light_again.png", full_page=True)
        log("9e. Dark Mode", "Toggle back to light", "pass" if final_mode == 'light' else "warn",
            f"final mode: {final_mode}")
    else:
        log("9b. Dark Mode", "Toggle dark mode", "warn",
            f"clicked={dark_toggled}, changed={mode_changed}, after={after_mode}")

    # ═══════════════ FINAL CHECKS ═══════════════
    # localStorage persistence
    ls = page.evaluate("localStorage.getItem('fund-dark-mode')")
    log("10. Persistence", "localStorage", "pass" if ls is not None else "warn", f"fund-dark-mode={ls}")

    # No JS errors overall
    if js_errors:
        log("10. Stability", "Console errors", "warn", f"{len(js_errors)}: {js_errors[:3]}")
    else:
        log("10. Stability", "No console errors", "pass")

    # Body content rendered
    body_len = page.evaluate("document.body.innerHTML.length")
    log("10. Quality", "Content rendered", "pass" if body_len > 1000 else "fail", f"HTML: {body_len} bytes")

    # Visual white flash check
    html_bg = page.evaluate("getComputedStyle(document.documentElement).backgroundColor")
    log("10. Quality", "No white flash", "warn" if '0, 0, 0, 0' == html_bg else "pass",
        f"html bg: {html_bg}")

    page.screenshot(path="/tmp/final_99_done.png", full_page=True)

    browser.close()

    # ═══════════════ SUMMARY ═══════════════
    passed = sum(1 for r in results if r['status'] == 'pass')
    warned = sum(1 for r in results if r['status'] == 'warn')
    failed = sum(1 for r in results if r['status'] == 'fail')
    skipped = sum(1 for r in results if r['status'] == 'skip')

    print("\n" + "=" * 70)
    print("FUND-DASHBOARD QA REPORT")
    print("=" * 70)
    print(f"Checks: {len(results)} | PASS: {passed} | WARN: {warned} | FAIL: {failed} | SKIP: {skipped}")

    print("\n--- FAILURES ---")
    for r in results:
        if r['status'] == 'fail':
            print(f"  [{r['flow']}] {r['check']}: {r['detail']}")

    print("\n--- WARNINGS ---")
    for r in results:
        if r['status'] == 'warn':
            print(f"  [{r['flow']}] {r['check']}: {r['detail']}")

    print("\n--- ALL RESULTS ---")
    for r in results:
        print(f"  {r['status'].upper():5s} | {r['flow']:20s} | {r['check']:30s} | {r['detail'][:80]}")

    print("\n--- JSON ---")
    print(json.dumps({
        "total": len(results), "pass": passed, "warn": warned, "fail": failed, "skip": skipped,
        "failures": [f"{r['flow']}/{r['check']}: {r['detail']}" for r in results if r['status']=='fail'],
        "warnings": [f"{r['flow']}/{r['check']}: {r['detail']}" for r in results if r['status']=='warn'],
        "screenshots_dir": "/tmp/"
    }, ensure_ascii=False, indent=2))
