#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Ant Fund transaction detail PDF converter.

Designed for Ant Group / Alipay fund transaction-detail PDFs in the format:
订单号, 交易时间, 交易类型, 基金名称, 组合基金名称, 基金代码,
申请金额, 申请份额, 确认金额, 确认份额, 手续费, 确认日期

Key hardening points:
- Uses PyMuPDF page.find_tables(), so it follows the original PDF grid instead of OCR.
- Merges page-top continuation rows caused by table row splitting across pages.
- Preserves 6-digit fund codes and long order IDs as strings.
- Treats 用户转换 differently from 用户跨TA转换:
  用户转换 PDF row is treated as target-fund conversion-in, and a synthetic source-fund
  conversion-out row is inferred from the running pre-conversion balance.
"""

from __future__ import annotations

import argparse
import csv
import json
import math
import re
import sys
from collections import Counter, defaultdict
from copy import deepcopy
from dataclasses import dataclass, asdict
from datetime import datetime
from pathlib import Path
from typing import Any, Dict, Iterable, List, Optional, Tuple

try:
    import fitz  # PyMuPDF
except Exception as exc:  # pragma: no cover
    raise SystemExit("PyMuPDF is required: pip install pymupdf") from exc


RAW_COLUMNS = [
    "订单号", "交易时间", "交易类型", "基金名称", "组合基金名称", "基金代码",
    "申请金额", "申请份额", "确认金额", "确认份额", "手续费", "确认日期",
]

CLEAN_COLUMNS = [
    "record_id", "seq", "source_page", "source_y0", "is_synthetic", "synthetic_reason",
    "order_id", "trade_time", "confirm_date", "trade_type", "direction", "leg_role",
    "fund_code", "fund_name", "combo_fund_name", "apply_amount", "apply_share",
    "confirm_amount", "confirm_share", "fee", "conversion_value", "inferred_nav",
    "signed_cash_flow", "signed_share_change", "raw_order_cell", "raw_trade_time_cell",
    "raw_fund_name_cell", "raw_confirm_date_cell", "source_inference_note",
]

TRADE_TYPES = {
    "用户买入", "定投买入", "机构分红", "用户卖出", "用户跨TA转换", "定投卖出", "机构强赎", "用户转换"
}

MONEY_FUND_HINTS = ["货币", "日日盈"]


def cell_text(x: Any) -> str:
    if x is None:
        return ""
    return str(x).replace("\u3000", " ").strip()


def normalize_compact(s: Any) -> str:
    """Remove whitespace and visual separators while keeping meaningful punctuation."""
    s = cell_text(s)
    s = s.replace("|", "")
    return re.sub(r"\s+", "", s)


def normalize_trade_type(s: Any) -> str:
    return normalize_compact(s)


def parse_float(s: Any) -> Optional[float]:
    s = normalize_compact(s)
    if not s or s in {"/", "-", "--", "nan", "None"}:
        return None
    s = s.replace(",", "")
    try:
        return float(s)
    except ValueError:
        return None


def round2(x: Optional[float]) -> Optional[float]:
    if x is None or (isinstance(x, float) and math.isnan(x)):
        return None
    return round(float(x) + 0.0, 2)


def parse_order_id(s: Any) -> str:
    return "".join(re.findall(r"\d", cell_text(s)))


def parse_fund_code(s: Any) -> str:
    digits = "".join(re.findall(r"\d", cell_text(s)))
    if not digits:
        return ""
    # The table column is a 6-digit fund code. If extraction ever contains extra text,
    # choose the last 6 digits because leading page/order fragments are more likely noise.
    return digits[-6:].zfill(6)


def parse_datetime_cell(s: Any) -> Optional[str]:
    # Date cells are split like "2026/06/0\n9 11:41". Digits-only is robust.
    digits = "".join(re.findall(r"\d", cell_text(s)))
    if len(digits) < 8:
        return None
    # With time: YYYYMMDDHHMM. With date only, use date.
    if len(digits) >= 12:
        y, m, d, hh, mm = digits[0:4], digits[4:6], digits[6:8], digits[8:10], digits[10:12]
        try:
            return datetime(int(y), int(m), int(d), int(hh), int(mm)).strftime("%Y-%m-%d %H:%M")
        except ValueError:
            return None
    y, m, d = digits[0:4], digits[4:6], digits[6:8]
    try:
        return datetime(int(y), int(m), int(d)).strftime("%Y-%m-%d")
    except ValueError:
        return None


def parse_date_cell(s: Any) -> Optional[str]:
    dt = parse_datetime_cell(s)
    return dt[:10] if dt else None


def clean_fund_name(s: Any) -> str:
    name = normalize_compact(s)
    # Correct a recurring visual split in this PDF: 红利低波动 may be split as 红利低波\n动
    # The generic whitespace removal already makes it 红利低波动.
    return name


def is_header_row(row: List[str]) -> bool:
    joined = "".join(normalize_compact(x) for x in row)
    return "订单号" in joined and "交易时间" in joined and "基金代码" in joined


def is_probable_new_tx(row: List[str]) -> bool:
    if len(row) < 12:
        return False
    order = parse_order_id(row[0])
    trade_type = normalize_trade_type(row[2])
    code = parse_fund_code(row[5])
    has_date_prefix = bool(re.match(r"^(20\d{6})", order))
    return (trade_type in TRADE_TYPES and has_date_prefix) or (has_date_prefix and len(code) == 6 and bool(parse_datetime_cell(row[1])))


def is_probable_continuation(row: List[str]) -> bool:
    if not row or is_header_row(row):
        return False
    non_empty = [i for i, x in enumerate(row) if cell_text(x)]
    if not non_empty:
        return False
    if is_probable_new_tx(row):
        return False
    # Continuation rows usually only contain fragments of order id / fund name / dates.
    return True


def append_cells(base: List[str], cont: List[str]) -> List[str]:
    out = list(base)
    for i, value in enumerate(cont[:len(out)]):
        value = cell_text(value)
        if not value:
            continue
        if out[i]:
            out[i] = out[i].rstrip() + "\n" + value
        else:
            out[i] = value
    return out


def extract_raw_rows(pdf_path: Path) -> Tuple[List[Dict[str, Any]], List[Dict[str, Any]], Dict[str, Any]]:
    doc = fitz.open(str(pdf_path))
    logical_rows: List[Dict[str, Any]] = []
    merge_log: List[Dict[str, Any]] = []
    raw_grid_rows = 0
    seq = 0

    for page_idx in range(doc.page_count):
        page = doc[page_idx]
        try:
            tables = page.find_tables().tables
        except Exception as exc:
            raise RuntimeError(f"table detection failed on page {page_idx + 1}: {exc}") from exc
        if not tables:
            continue
        # This statement has a single grid table per page; choose the largest table if multiple.
        table = max(tables, key=lambda t: (t.bbox[2] - t.bbox[0]) * (t.bbox[3] - t.bbox[1]))
        data = table.extract()
        if not data:
            continue
        raw_grid_rows += len(data)
        row_count = getattr(table, "row_count", len(data))
        # Estimate y0 of each extracted row for auditing. PyMuPDF row bbox API changes across versions,
        # so use a stable interpolation fallback.
        y_top, y_bottom = table.bbox[1], table.bbox[3]
        row_h = (y_bottom - y_top) / max(row_count, 1)
        for ridx, row in enumerate(data):
            row = [cell_text(x) for x in row]
            if len(row) < len(RAW_COLUMNS):
                row = row + [""] * (len(RAW_COLUMNS) - len(row))
            elif len(row) > len(RAW_COLUMNS):
                row = row[:len(RAW_COLUMNS)]
            y0 = round(y_top + row_h * ridx, 2)
            if is_header_row(row):
                continue
            if is_probable_new_tx(row):
                seq += 1
                logical_rows.append({
                    "seq": seq,
                    "source_page": page_idx + 1,
                    "source_y0": y0,
                    **{RAW_COLUMNS[i]: row[i] for i in range(len(RAW_COLUMNS))},
                })
            elif is_probable_continuation(row):
                if not logical_rows:
                    # Keep orphan continuation as issue-like raw row instead of dropping silently.
                    seq += 1
                    logical_rows.append({
                        "seq": seq,
                        "source_page": page_idx + 1,
                        "source_y0": y0,
                        **{RAW_COLUMNS[i]: row[i] for i in range(len(RAW_COLUMNS))},
                    })
                    continue
                prev = logical_rows[-1]
                before = [prev.get(c, "") for c in RAW_COLUMNS]
                after = append_cells(before, row)
                for i, col in enumerate(RAW_COLUMNS):
                    prev[col] = after[i]
                merge_log.append({
                    "continuation_page": page_idx + 1,
                    "continuation_y0": y0,
                    "merged_into_seq": prev["seq"],
                    "continuation_cells": row,
                    "before": before,
                    "after": after,
                })

    meta = {
        "pdf_pages": doc.page_count,
        "grid_raw_rows_including_headers": raw_grid_rows,
        "logical_transaction_rows": len(logical_rows),
        "continuation_rows_merged": len(merge_log),
    }
    doc.close()
    return logical_rows, merge_log, meta


def clean_one_raw(row: Dict[str, Any]) -> Dict[str, Any]:
    order_id = parse_order_id(row.get("订单号"))
    trade_time = parse_datetime_cell(row.get("交易时间"))
    confirm_date = parse_date_cell(row.get("确认日期"))
    trade_type = normalize_trade_type(row.get("交易类型"))
    fund_code = parse_fund_code(row.get("基金代码"))
    fund_name = clean_fund_name(row.get("基金名称"))
    combo = clean_fund_name(row.get("组合基金名称")) or None
    apply_amount = parse_float(row.get("申请金额"))
    apply_share = parse_float(row.get("申请份额"))
    confirm_amount = parse_float(row.get("确认金额"))
    confirm_share = parse_float(row.get("确认份额"))
    fee = parse_float(row.get("手续费")) or 0.0

    direction = "unknown"
    leg_role = ""
    share_change = 0.0
    cash_flow = 0.0
    conversion_value = None

    if trade_type in {"用户买入", "定投买入"}:
        direction, leg_role = "buy", "external_buy"
        share_change = confirm_share or 0.0
        cash_flow = -(confirm_amount or apply_amount or 0.0)
    elif trade_type in {"用户卖出", "定投卖出"}:
        direction, leg_role = "sell", "external_sell"
        share_change = -(confirm_share if confirm_share is not None else (apply_share or 0.0))
        cash_flow = (confirm_amount or 0.0) - fee
    elif trade_type == "机构分红":
        direction, leg_role = "dividend", "dividend"
        share_change = confirm_share or 0.0
        cash_flow = confirm_amount or 0.0
    elif trade_type == "机构强赎":
        direction, leg_role = "forced_redeem", "forced_redeem"
        share_change = -(confirm_share if confirm_share is not None else (apply_share or 0.0))
        cash_flow = (confirm_amount or 0.0) - fee
    elif trade_type == "用户跨TA转换":
        conversion_value = confirm_amount
        # Cross-TA conversion appears as a pair with the same order id.
        # In leg: apply_amount > 0 and apply_share is blank or '/'.
        if apply_amount is not None and apply_amount > 0 and apply_share is None:
            direction, leg_role = "convert_in", "cross_ta_in"
            share_change = confirm_share or 0.0
        else:
            direction, leg_role = "convert_out", "cross_ta_out"
            share_change = -(confirm_share if confirm_share is not None else (apply_share or 0.0))
        cash_flow = 0.0
    elif trade_type == "用户转换":
        # IMPORTANT: This PDF gives a single row naming the target fund.
        # The target gets confirm_share. The source is inferred later and added as a synthetic row.
        direction, leg_role = "convert_in", "normal_convert_target"
        conversion_value = confirm_amount
        share_change = confirm_share or 0.0
        cash_flow = 0.0

    inferred_nav = None
    if confirm_amount not in (None, 0) and confirm_share not in (None, 0):
        inferred_nav = confirm_amount / confirm_share

    return {
        "record_id": str(row.get("seq", "")),
        "seq": row.get("seq"),
        "source_page": row.get("source_page"),
        "source_y0": row.get("source_y0"),
        "is_synthetic": False,
        "synthetic_reason": "",
        "order_id": order_id,
        "trade_time": trade_time,
        "confirm_date": confirm_date,
        "trade_type": trade_type,
        "direction": direction,
        "leg_role": leg_role,
        "fund_code": fund_code,
        "fund_name": fund_name,
        "combo_fund_name": combo,
        "apply_amount": round2(apply_amount),
        "apply_share": round2(apply_share),
        "confirm_amount": round2(confirm_amount),
        "confirm_share": round2(confirm_share),
        "fee": round2(fee),
        "conversion_value": round2(conversion_value),
        "inferred_nav": round(inferred_nav, 6) if inferred_nav else None,
        "signed_cash_flow": round2(cash_flow),
        "signed_share_change": round2(share_change),
        "raw_order_cell": cell_text(row.get("订单号")),
        "raw_trade_time_cell": cell_text(row.get("交易时间")),
        "raw_fund_name_cell": cell_text(row.get("基金名称")),
        "raw_confirm_date_cell": cell_text(row.get("确认日期")),
        "source_inference_note": "",
    }


def sort_key_for_time(row: Dict[str, Any]) -> Tuple[str, int, str]:
    # Stable chronological order; seq is descending in the PDF, so ascending seq is reverse chronological.
    # Use seq as tie breaker only.
    return (row.get("trade_time") or "0000-00-00 00:00", int(row.get("seq") or 0), row.get("order_id") or "")


def add_normal_conversion_sources(clean_rows: List[Dict[str, Any]], tolerance: float = 0.02) -> Tuple[List[Dict[str, Any]], List[Dict[str, Any]]]:
    """Infer source legs for 用户转换 rows using running balances before the conversion.

    The PDF row for 用户转换 names the target fund only. The source fund is not printed
    in the same row. This function uses two conservative clues:
    1) exact pre-conversion balance equals the row's 申请份额;
    2) if not exact, a fund's final residual balance equals the row's 申请份额, so adding
       a synthetic source leg would close that residual to zero.

    The second clue is what fixes the common case where a user converted part of a position
    and later sold the remaining part; the raw summary otherwise shows a fake current holding.
    """
    rows = sorted(clean_rows, key=sort_key_for_time)
    balances: Dict[str, float] = defaultdict(float)
    names: Dict[str, str] = {}
    synthetic: List[Dict[str, Any]] = []
    events: List[Dict[str, Any]] = []

    # Final balances before adding any synthetic normal-conversion source rows.
    final_balances: Dict[str, float] = defaultdict(float)
    final_names: Dict[str, str] = {}
    for rr in clean_rows:
        code0 = rr.get("fund_code") or ""
        if code0:
            final_names[code0] = rr.get("fund_name") or final_names.get(code0, "")
            final_balances[code0] += float(rr.get("signed_share_change") or 0.0)

    def apply_balance(r: Dict[str, Any]) -> None:
        code = r.get("fund_code") or ""
        if code:
            names[code] = r.get("fund_name") or names.get(code, "")
            balances[code] += float(r.get("signed_share_change") or 0.0)
            # avoid tiny floating residue
            if abs(balances[code]) < 0.005:
                balances[code] = 0.0

    def score_candidates(source_share: float, target_code: str) -> List[Dict[str, Any]]:
        scored: List[Dict[str, Any]] = []
        for code, bal in balances.items():
            if code == target_code:
                continue
            bal2 = round(bal, 2)
            if bal2 + tolerance < round(source_share, 2):
                continue
            final2 = round(final_balances.get(code, 0.0), 2)
            after_final = round(final2 - source_share, 2)
            exact_pre_diff = abs(bal2 - round(source_share, 2))
            closes_final_diff = abs(final2 - round(source_share, 2))
            score = 0
            reason = []
            if exact_pre_diff <= tolerance:
                score += 100
                reason.append("exact_pre_balance")
            if closes_final_diff <= tolerance:
                score += 90
                reason.append("closes_final_residual")
            # Penalize candidates that would create a negative final position.
            if after_final < -tolerance:
                score -= 200
                reason.append("would_make_negative_final")
            if score > 0:
                scored.append({
                    "code": code,
                    "name": names.get(code) or final_names.get(code, ""),
                    "pre_balance": bal2,
                    "raw_final_balance": final2,
                    "after_final_if_source": after_final,
                    "score": score,
                    "reason": ",".join(reason),
                })
        scored.sort(key=lambda x: (-x["score"], abs(x["after_final_if_source"]), x["code"]))
        return scored

    for r in rows:
        if r.get("trade_type") == "用户转换" and not r.get("is_synthetic"):
            source_share = r.get("apply_share")
            target_code = r.get("fund_code") or ""
            candidates = score_candidates(float(source_share), target_code) if source_share is not None and source_share > 0 else []
            chosen = None
            if candidates:
                # Accept only if the top score is unique.
                if len(candidates) == 1 or candidates[0]["score"] > candidates[1]["score"]:
                    chosen = candidates[0]
            if chosen is not None:
                code, name, bal = chosen["code"], chosen["name"], chosen["pre_balance"]
                src = deepcopy(r)
                src["record_id"] = f"{r.get('record_id')}_SYN_SRC"
                src["is_synthetic"] = True
                src["synthetic_reason"] = "用户转换源基金由转换前余额/最终残差自动推断"
                src["direction"] = "convert_out"
                src["leg_role"] = "normal_convert_source_inferred"
                src["fund_code"] = code
                src["fund_name"] = name
                src["confirm_share"] = round2(source_share)
                src["signed_share_change"] = round2(-source_share)
                src["signed_cash_flow"] = 0.0
                src["source_inference_note"] = (
                    f"matched source by {chosen['reason']}; pre_balance={bal:.2f}; "
                    f"raw_final_balance={chosen['raw_final_balance']:.2f}; target={target_code}; original_row={r.get('record_id')}"
                )
                r["source_inference_note"] = f"target leg; source inferred as {code} {name}, source_share={source_share:.2f}"
                synthetic.append(src)
                events.append({
                    "order_id": r.get("order_id"),
                    "trade_time": r.get("trade_time"),
                    "target_fund_code": target_code,
                    "target_fund_name": r.get("fund_name"),
                    "target_confirm_share": r.get("confirm_share"),
                    "source_fund_code": code,
                    "source_fund_name": name,
                    "source_apply_share": round2(source_share),
                    "status": "inferred_" + chosen["reason"],
                    "candidates": candidates,
                })
                apply_balance(r)
                apply_balance(src)
                continue
            else:
                status = "no_candidate" if not candidates else "ambiguous_candidates"
                r["source_inference_note"] = f"WARNING: {status}; source_share={source_share}; candidates={candidates}"
                events.append({
                    "order_id": r.get("order_id"),
                    "trade_time": r.get("trade_time"),
                    "target_fund_code": target_code,
                    "target_fund_name": r.get("fund_name"),
                    "target_confirm_share": r.get("confirm_share"),
                    "source_apply_share": round2(source_share),
                    "status": status,
                    "candidates": candidates,
                })
        apply_balance(r)

    merged = clean_rows + synthetic
    merged.sort(key=lambda x: (x.get("trade_time") or "", int(x.get("seq") or 0), str(x.get("record_id"))))
    for idx, r in enumerate(merged, start=1):
        r["export_seq"] = idx
    return merged, events


def build_summary(rows: List[Dict[str, Any]]) -> Tuple[List[Dict[str, Any]], List[Dict[str, Any]]]:
    groups: Dict[str, Dict[str, Any]] = {}
    for r in rows:
        code = r.get("fund_code") or ""
        if not code:
            continue
        g = groups.setdefault(code, {
            "fund_code": code,
            "fund_name": r.get("fund_name") or "",
            "tx_count": 0,
            "real_tx_count": 0,
            "synthetic_tx_count": 0,
            "buy_amount": 0.0,
            "sell_amount": 0.0,
            "dividend_amount": 0.0,
            "conversion_in_amount": 0.0,
            "conversion_out_amount": 0.0,
            "fee_total": 0.0,
            "net_share_change": 0.0,
            "net_cash_flow": 0.0,
            "first_trade": None,
            "last_trade": None,
            "notes": "",
        })
        if r.get("fund_name"):
            g["fund_name"] = r.get("fund_name")
        g["tx_count"] += 1
        if r.get("is_synthetic"):
            g["synthetic_tx_count"] += 1
        else:
            g["real_tx_count"] += 1
        direction = r.get("direction")
        amt = float(r.get("confirm_amount") or 0.0)
        fee = float(r.get("fee") or 0.0)
        conv = float(r.get("conversion_value") or 0.0)
        if direction == "buy":
            g["buy_amount"] += amt
        elif direction in {"sell", "forced_redeem"}:
            g["sell_amount"] += amt
        elif direction == "dividend":
            g["dividend_amount"] += amt
        elif direction == "convert_in":
            g["conversion_in_amount"] += conv
        elif direction == "convert_out":
            g["conversion_out_amount"] += conv
        g["fee_total"] += fee
        g["net_share_change"] += float(r.get("signed_share_change") or 0.0)
        g["net_cash_flow"] += float(r.get("signed_cash_flow") or 0.0)
        t = (r.get("trade_time") or "")[:10]
        if t:
            if g["first_trade"] is None or t < g["first_trade"]:
                g["first_trade"] = t
            if g["last_trade"] is None or t > g["last_trade"]:
                g["last_trade"] = t
    out = []
    positions = []
    for g in groups.values():
        for k in ["buy_amount", "sell_amount", "dividend_amount", "conversion_in_amount", "conversion_out_amount", "fee_total", "net_share_change", "net_cash_flow"]:
            g[k] = round(float(g[k]), 2)
        if abs(g["net_share_change"]) < 0.005:
            g["net_share_change"] = 0.0
        if abs(g["net_share_change"]) >= 0.01:
            pos = dict(g)
            is_money = any(h in g["fund_name"] for h in MONEY_FUND_HINTS)
            if g["net_share_change"] < -0.01:
                pos["position_flag"] = "negative_money_fund_or_missing_income" if is_money else "negative_abnormal_review"
            else:
                pos["position_flag"] = "positive_position_by_statement"
            positions.append(pos)
        out.append(g)
    out.sort(key=lambda x: (-abs(x["net_share_change"]), x["fund_code"]))
    positions.sort(key=lambda x: (-x["net_share_change"], x["fund_code"]))
    return out, positions


def validate_rows(rows: List[Dict[str, Any]], positions: List[Dict[str, Any]], conversion_events: List[Dict[str, Any]], meta: Dict[str, Any]) -> List[Dict[str, Any]]:
    issues: List[Dict[str, Any]] = []
    for r in rows:
        rid = r.get("record_id")
        if not r.get("is_synthetic"):
            if not r.get("order_id") or len(r.get("order_id", "")) < 20:
                issues.append({"level": "error", "record_id": rid, "issue": "malformed_order_id", "value": r.get("raw_order_cell")})
            if not re.fullmatch(r"\d{6}", r.get("fund_code") or ""):
                issues.append({"level": "error", "record_id": rid, "issue": "malformed_fund_code", "value": r.get("fund_code")})
            if r.get("trade_type") not in TRADE_TYPES:
                issues.append({"level": "error", "record_id": rid, "issue": "unknown_trade_type", "value": r.get("trade_type")})
            if not r.get("trade_time"):
                issues.append({"level": "error", "record_id": rid, "issue": "unparsed_trade_time", "value": r.get("raw_trade_time_cell")})
            if not r.get("confirm_date"):
                issues.append({"level": "error", "record_id": rid, "issue": "unparsed_confirm_date", "value": r.get("raw_confirm_date_cell")})
    for ev in conversion_events:
        if not str(ev.get("status", "")).startswith("inferred_"):
            issues.append({"level": "warning", "record_id": ev.get("order_id"), "issue": "normal_conversion_source_not_unique", "detail": ev})
    for p in positions:
        if p.get("position_flag") == "negative_abnormal_review":
            issues.append({"level": "warning", "fund_code": p.get("fund_code"), "issue": "negative_position", "net_share_change": p.get("net_share_change")})
        elif p.get("position_flag") == "negative_money_fund_or_missing_income":
            issues.append({"level": "info", "fund_code": p.get("fund_code"), "issue": "negative_money_fund_likely_income_gap", "net_share_change": p.get("net_share_change")})
    return issues


def write_csv(path: Path, rows: List[Dict[str, Any]], columns: Optional[List[str]] = None) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    if columns is None:
        keys = []
        seen = set()
        for r in rows:
            for k in r.keys():
                if k not in seen:
                    keys.append(k); seen.add(k)
        columns = keys
    with path.open("w", newline="", encoding="utf-8-sig") as f:
        writer = csv.DictWriter(f, fieldnames=columns, extrasaction="ignore")
        writer.writeheader()
        for r in rows:
            writer.writerow(r)


def main(argv: Optional[List[str]] = None) -> int:
    parser = argparse.ArgumentParser(description="Convert Ant/Alipay fund transaction detail PDF into clean CSVs.")
    parser.add_argument("pdf", type=Path, help="input PDF")
    parser.add_argument("--out", type=Path, default=Path("ant_fund_export"), help="output directory")
    parser.add_argument("--conversion-tolerance", type=float, default=0.02, help="share tolerance for normal 用户转换 source inference")
    args = parser.parse_args(argv)

    pdf_path = args.pdf
    out_dir = args.out
    out_dir.mkdir(parents=True, exist_ok=True)

    raw_rows, merge_log, meta = extract_raw_rows(pdf_path)
    cleaned = [clean_one_raw(r) for r in raw_rows]
    normalized, conversion_events = add_normal_conversion_sources(cleaned, tolerance=args.conversion_tolerance)
    summary, positions = build_summary(normalized)
    issues = validate_rows(normalized, positions, conversion_events, meta)

    trade_type_counts = Counter(r.get("trade_type") for r in cleaned)
    direction_counts = Counter(r.get("direction") for r in normalized)
    qa = {
        **meta,
        "clean_rows_from_pdf": len(cleaned),
        "normalized_rows_including_synthetic": len(normalized),
        "synthetic_rows_added": sum(1 for r in normalized if r.get("is_synthetic")),
        "unique_fund_codes": len({r.get("fund_code") for r in cleaned if r.get("fund_code")}),
        "trade_type_counts": dict(trade_type_counts),
        "direction_counts": dict(direction_counts),
        "normal_conversion_events": conversion_events,
        "position_count_nonzero": len(positions),
        "issue_count": len([x for x in issues if x.get("level") in {"error", "warning"}]),
        "issues": issues,
    }

    write_csv(out_dir / "transactions_raw_merged.csv", raw_rows, ["seq", "source_page", "source_y0"] + RAW_COLUMNS)
    write_csv(out_dir / "transactions_normalized.csv", normalized, CLEAN_COLUMNS + ["export_seq"])
    write_csv(out_dir / "summary_by_fund_corrected.csv", summary)
    write_csv(out_dir / "current_positions_corrected.csv", positions)
    write_csv(out_dir / "normal_conversion_review.csv", conversion_events)
    write_csv(out_dir / "manual_review_issues.csv", issues)
    (out_dir / "qa_report_corrected.json").write_text(json.dumps(qa, ensure_ascii=False, indent=2), encoding="utf-8")
    (out_dir / "continuation_merge_log.json").write_text(json.dumps(merge_log, ensure_ascii=False, indent=2), encoding="utf-8")

    print(json.dumps({
        "out_dir": str(out_dir),
        "pdf_pages": meta["pdf_pages"],
        "logical_transaction_rows": meta["logical_transaction_rows"],
        "continuation_rows_merged": meta["continuation_rows_merged"],
        "normalized_rows_including_synthetic": len(normalized),
        "synthetic_rows_added": qa["synthetic_rows_added"],
        "issue_count": qa["issue_count"],
    }, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
