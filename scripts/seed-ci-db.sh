#!/usr/bin/env bash
# Create a minimal SQLite DB for CI / local container smoke tests.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${1:-$ROOT/deploy/ci-data}"
DB_PATH="$OUT_DIR/fund.db"
SQL_PATH="$ROOT/deploy/ci-seed.sql"

mkdir -p "$OUT_DIR"
rm -f "$DB_PATH" "$DB_PATH-wal" "$DB_PATH-shm"

if command -v sqlite3 >/dev/null 2>&1; then
  sqlite3 "$DB_PATH" < "$SQL_PATH"
elif command -v python3 >/dev/null 2>&1; then
  python3 - "$DB_PATH" "$SQL_PATH" <<'PY'
import sqlite3, sys
db_path, sql_path = sys.argv[1], sys.argv[2]
con = sqlite3.connect(db_path)
con.executescript(open(sql_path, encoding="utf-8").read())
con.commit()
con.close()
PY
elif command -v python >/dev/null 2>&1; then
  python - "$DB_PATH" "$SQL_PATH" <<'PY'
import sqlite3, sys
db_path, sql_path = sys.argv[1], sys.argv[2]
con = sqlite3.connect(db_path)
con.executescript(open(sql_path, encoding="utf-8").read())
con.commit()
con.close()
PY
else
  echo "Need sqlite3 or python to seed CI database" >&2
  exit 1
fi

echo "Seeded $DB_PATH"
