/** /api/admin — 外部管理 API (Hermes/agent 调用)
 *
 *  诊断 · 刷新 · 导入 · 爬虫控制 · 重算 · 校验 · 交易CRUD · 证券CRUD
 *  Stock support: security_type, market columns on fund_details & related tables.
 *
 *  子路由:
 *    admin/crud.ts      — 交易 & 证券 CRUD
 *    admin/ops.ts       — 运维操作 (诊断 · 批量导入 · 重算 · 校验 · 完整性 · 备份)
 *    admin/freshness.ts — 数据新鲜度 & 告警
 *    admin/import.ts    — CSV 导入 · 爬虫触发
 */

import { Hono } from "hono";
import crudRouter from "./admin/crud";
import opsRouter from "./admin/ops";
import freshnessRouter from "./admin/freshness";
import importRouter from "./admin/import";
import dashboardRouter from "./admin/dashboard";

const router = new Hono();
router.route("/", crudRouter);
router.route("/", opsRouter);
router.route("/", freshnessRouter);
router.route("/", importRouter);
router.route("/", dashboardRouter);
export default router;
