# Archived QA Scripts

These Python scripts were replaced by proper Playwright E2E tests in `packages/web/e2e/`.

- `qa_test.py` - Original comprehensive QA test → converted to `packages/web/e2e/fund-dashboard.spec.ts`
- `qa_debug.py` - Debug script for troubleshooting
- `qa_edge.py` - Edge case testing
- `qa_final_verify.py` - Final verification check
- `qa_table_check.py` - Table rendering check
- `qa_verify.py` - Quick verification

Run E2E tests now with:
```bash
cd packages/web && npx playwright test
```
