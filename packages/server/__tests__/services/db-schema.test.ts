import { describe, test, expect } from "bun:test";
import { spawnSync } from "node:child_process";
import { join } from "node:path";
import { pathToFileURL } from "node:url";

describe("initSchema transaction schema migration", () => {
  test("restores seq primary key while preserving conversion order legs", () => {
    const dbModuleUrl = pathToFileURL(join(import.meta.dir, "../../db.ts")).href;
    const script = `
      import { Database } from "bun:sqlite";
      import { initSchema } from ${JSON.stringify(dbModuleUrl)};

      const db = new Database(":memory:");
      db.run("CREATE TABLE fund_details (fund_code TEXT PRIMARY KEY, fund_name TEXT, fund_type TEXT)");
      db.run("INSERT INTO fund_details (fund_code, fund_name, fund_type) VALUES ('009504', 'Gold A', 'fund'), ('014880', 'Robot A', 'fund')");
      db.run(\`
        CREATE TABLE transactions (
          seq INTEGER,
          order_id TEXT NOT NULL,
          trade_time TEXT NOT NULL,
          confirm_date TEXT,
          trade_type TEXT,
          direction TEXT NOT NULL,
          fund_code TEXT NOT NULL,
          fund_name TEXT,
          confirm_amount REAL NOT NULL,
          confirm_share REAL,
          fee REAL,
          signed_cash_flow REAL,
          signed_share_change REAL
        )
      \`);
      db.run(\`
        INSERT INTO transactions
          (seq, order_id, trade_time, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share)
        VALUES
          (10, 'convert_001', '2026-01-02 10:00:00', '用户跨TA转换', 'convert_out', '009504', 'Gold A', 100, 50),
          (NULL, 'convert_001', '2026-01-02 10:00:00', '用户跨TA转换', 'convert_in', '014880', 'Robot A', 99, 80),
          (NULL, 'manual_001', '2026-01-03 10:00:00', '用户买入', 'buy', '014880', 'Robot A', 20, 10)
      \`);

      initSchema(db);

      const txPk = db.query("PRAGMA table_info('transactions')").all().filter((r) => r.pk).map((r) => r.name).join(",");
      if (txPk !== "seq") throw new Error("transactions seq primary key was not restored");

      const nullSeq = db.query("SELECT COUNT(*) AS n FROM transactions WHERE seq IS NULL").get();
      if (nullSeq.n !== 0) throw new Error("transactions NULL seq rows remain");

      const convertRows = db.query("SELECT COUNT(*) AS n FROM transactions WHERE order_id = 'convert_001'").get();
      if (convertRows.n !== 2) throw new Error("conversion order legs were collapsed");

      db.run(\`
        INSERT OR IGNORE INTO transactions
          (order_id, trade_time, trade_type, direction, fund_code, fund_name, confirm_amount, confirm_share)
        VALUES ('convert_001', '2026-01-02 10:00:00', '用户跨TA转换', 'convert_in', '014880', 'Robot A', 99, 80)
      \`);
      const afterDuplicate = db.query("SELECT COUNT(*) AS n FROM transactions WHERE order_id = 'convert_001' AND fund_code = '014880' AND direction = 'convert_in'").get();
      if (afterDuplicate.n !== 1) throw new Error("duplicate import identity was not ignored");
    `;

    const result = spawnSync(process.execPath, ["--eval", script], {
      cwd: join(import.meta.dir, "../.."),
      encoding: "utf8",
    });
    expect(result.status, result.stderr || result.stdout).toBe(0);
  });
});
