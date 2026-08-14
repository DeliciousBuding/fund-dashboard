# HISTORICAL helper: count MCP tools from smoke tmp JSON (threshold 44).
# Prefer ./scripts/smoke-prod.sh as the operator entrypoint.
import json, os
from pathlib import Path

cands = []
for k in ("SMOKE_TMP", "TMPDIR", "TEMP", "TMP"):
    v = os.environ.get(k)
    if v:
        cands.append(Path(v) / "fd-tools.json")
cands.append(Path("/tmp/fd-tools.json"))
path = next((c for c in cands if c.exists()), None)
if path is None:
    raise SystemExit("fd-tools.json not found")
data = json.loads(path.read_text(encoding="utf-8"))
tools = data.get("result", {}).get("tools") or data.get("tools") or []
raise SystemExit(0 if len(tools) >= 44 else 1)
