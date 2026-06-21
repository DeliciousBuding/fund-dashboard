import { useState, useEffect, useRef, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Text, Button } from '@cloudflare/kumo'
import { CaretLeft } from '@phosphor-icons/react'
import { use as echartsUse, getInstanceByDom } from 'echarts/core'
import { TreemapChart } from 'echarts/charts'
import { TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import { getTheme, chartTooltip } from '../styles/theme'
import { useEChart } from '../hooks/useEChart'
import { Card } from './ui/Card'
import {
  fetchPortfolioPenetration,
  type PenetrationStock,
  type PenetrationFund,
} from '../api'

echartsUse([TreemapChart, TooltipComponent, CanvasRenderer])

/** Sector -> color mapping for treemap rectangles.
 *  Covers both Chinese and US stock sectors. */
const SECTOR_COLORS: Record<string, string> = {
  tech: '#3172d9',         // blue — 信息技术/科技
  financial: '#199c63',    // green — 金融
  consumer: '#e07b2c',     // amber — 消费
  healthcare: '#be4bdb',   // purple — 医疗健康
  energy: '#f08c00',       // orange — 能源
  industrial: '#0ca678',   // teal — 工业/制造
  materials: '#2b8a3e',    // dark green — 材料
  realestate: '#c92a2a',   // red — 房地产
  utilities: '#1c7ed6',    // sky blue — 公用事业
  telecom: '#5c7cfa',      // indigo — 通信服务
  communication: '#5c7cfa', // indigo — 通信 (US sector name)
  consumer_cyclical: '#f59f00', // golden — 可选消费 (US sector name)
  consumer_defensive: '#e8590c', // deep orange — 必选消费
  other: '#868e96',         // gray
}

/** Best-effort sector classifier from stock name keywords.
 *  Supports both Chinese and US stock names. */
function classifySector(name: string): string {
  const n = name.toLowerCase()

  // US tech giants and keywords
  if (/科技|软件|互联网|人工智能|芯片|半导体|电子|计算|数据|信息|通信|5g|ai|cloud|software|机器人|apple|microsoft|google|alphabet|amazon|meta|nvidia|tesla|netflix|adobe|salesforce|oracle|intel|amd|broadcom|qualcomm|cisco|ibm|sap|shopify|snowflake|palantir|datadog|crowdstrike|servicenow|workday|zoom|square|block|uber|lyft|airbnb|snap|pinterest|spotify|twilio|okta|zscaler|mongodb|atlassian|splunk|docuSign|hubspot|paypal/.test(n))
    return 'tech'

  // US financial
  if (/银行|保险|证券|金融|信托|基金|bank|jpmorgan|goldman|morgan stanley|wells fargo|citi|visa|mastercard|american express|berkshire|blackrock|blackstone|schwab|fidelity|paypal|stripe|coinbase|robinhood|chubb|aig|metlife|prudential|allianz|marsh|aflac|travelers|pnc|us bancorp|truist|capital one|discover|amex/.test(n))
    return 'financial'

  // US consumer / retail / discretionary
  if (/食品|饮料|白酒|消费|零售|汽车|家电|服装|旅游|酒店|传媒|娱乐|游戏|教育|walmart|costco|target|amazon|home depot|lowe's|nike|starbucks|mcdonald|pepsi|coca.?cola|p&g|procter|unilever|colgate|est[eé]e lauder|lululemon|tjx|ross|dollar|best buy|chipotle|domino|yum|booking|expedia|marriott|hilton|delta|united airlines|southwest|carnival|royal caribbean|disney|netflix|activision|electronic arts|take.?two|roblox|ford|general motors|toyota|honda|tesla|rivian|lucid/.test(n))
    return 'consumer'

  // US healthcare
  if (/医药|医疗|生物|制药|健康|医院|基因|疫苗|johnson|pfizer|moderna|novartis|roche|merck|abbvie|bristol.?myers|gilead|amgen|eli lilly|regeneron|biogen|vertex|illumina|thermo fisher|danaher|agilent|baxter|medtronic|stryker|boston scientific|intuitive surgical|hca|unitedhealth|cigna|humana|cvs|walgreens|centene|elevance|mckesson|cardi[ao]/.test(n))
    return 'healthcare'

  // US energy
  if (/石油|石化|能源|电力|煤炭|燃气|新能源|光伏|风电|锂电|电池|exxon|chevron|conocophillips|schlumberger|halliburton|baker hughes|occidental|devon|pioneer|marathon|valero|phillips 66|enphase|first solar|solar|plug power|fuelcell|nextera|duke energy|dominion|southern|exelon|consolidated edison|sempra|pg&e/.test(n))
    return 'energy'

  // US industrial / aerospace / defense
  if (/制造|机械|航空|航天|军工|船舶|高铁|建筑|建材|工程|物流|boeing|airbus|lockheed|raytheon|northrop|general dynamics|l3harris|honeywell|general electric|3m|caterpillar|deere|union pacific|csl|norfolk southern|fedex|ups|southwest airlines|delta air|united airlines|american airlines|emerson|rockwell|parker|eaton|illinois tool|stanley black|cummin|paccar|waste management|republic services/.test(n))
    return 'industrial'

  // US materials / mining / chemicals
  if (/钢铁|有色|化工|矿业|黄金|稀土|水泥|玻璃|造纸|dupont|dow|lyondell|basell|air products|linde|sherwin.?williams|ppg|eastman|celanese|newmont|barrick|freeport.?mcmoran|southern copper|nutrien|corteva|mosaic|albemarle|livent|steel dynamics|nucor|arcelormittal|rio tinto|bhp|vale|glencore/.test(n))
    return 'materials'

  // US real estate
  if (/地产|房地产|园区|物业|prologis|public storage|equinix|digital realty|welltower|avalonbay|equity residential|simon property|boston properties|crown castle|american tower|sba comm|ventas|alexandria|realty income/.test(n))
    return 'realestate'

  // US utilities
  if (/公用|水务|环保|燃气|duke energy|dominion|southern|exelon|nextera|edison|sempra|american electric|xcel|evergy|alliant|atmos|ug[ic]|public service|centerpoint|consolidated edison|waste management/.test(n))
    return 'utilities'

  // US communication / telecom
  if (/电信|联通|移动|通信|verizon|at&t|t.?mobile|comcast|charter|dish|disney|warner|paramount|fox|news.?corp|omnicom|interpublic|t-?mobile/.test(n))
    return 'communication'

  return 'other'
}

export default function PortfolioPenetration({ dark }: { dark: boolean }) {
  const { t } = useTranslation();
  const theme = getTheme(dark);
  const [data, setData] = useState<PenetrationStock[]>([])
  const [totalValue, setTotalValue] = useState(0)
  const [equityCount, setEquityCount] = useState(0)
  const [uniqueStocks, setUniqueStocks] = useState(0)
  const [selectedStock, setSelectedStock] = useState<PenetrationStock | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  useEffect(() => {
    abortRef.current?.abort()
    const ctrl = new AbortController()
    abortRef.current = ctrl
    setLoading(true)
    setError(null)
    fetchPortfolioPenetration(ctrl.signal)
      .then(res => {
        if (!ctrl.signal.aborted) {
          setData(res.penetration)
          setTotalValue(res.total_portfolio_value)
          setEquityCount(res.equity_fund_count)
          setUniqueStocks(res.unique_stocks)
          setLoading(false)
        }
      })
      .catch(e => {
        if (e.name !== 'AbortError') {
          console.warn('[penetration]', e)
          setError(e.message)
          setLoading(false)
        }
      })
    return () => ctrl.abort()
  }, [])

  // ── Treemap option ──────────────────────────────────────────────
  const treemapOption = useMemo(() => {
    if (!data.length) return {} as Record<string, unknown>

    type TreemapNode = {
      name: string
      value: number
      stock_code: string
      stock_name: string
      weight_pct: number
      held_by_funds: PenetrationFund[]
      itemStyle?: { color: string }
    }

    const nodes: TreemapNode[] = data.map(s => ({
      name: s.stock_name,
      value: s.total_exposure_cny,
      stock_code: s.stock_code,
      stock_name: s.stock_name,
      weight_pct: s.weight_pct,
      held_by_funds: s.held_by_funds,
      itemStyle: {
        color: SECTOR_COLORS[classifySector(s.stock_name)] || SECTOR_COLORS.other,
      },
    }))

    const sectorName: Record<string, string> = {
      tech: '科技', financial: '金融', consumer: '消费', healthcare: '医疗',
      energy: '能源', industrial: '工业', materials: '材料', realestate: '房地产',
      utilities: '公用事业', telecom: '通信', communication: '通信',
      consumer_cyclical: '可选消费', consumer_defensive: '必选消费',
    }

    return {
      tooltip: {
        trigger: 'item',
        ...chartTooltip(theme),
        formatter: (params: any) => {
          const d = params.data
          if (!d) return ''
          const funds = (d.held_by_funds as PenetrationFund[]) || []
          const fundList = funds
            .sort((a, b) => b.weight_pct - a.weight_pct)
            .map(f => `${f.fund_name}: ${f.weight_pct.toFixed(1)}%`)
            .join('<br/>  ')
          const sector = classifySector(d.stock_name || d.name)
          return `<strong>${d.stock_name || d.name}</strong> (${d.stock_code || ''})<br/>
            行业: ${sectorName[sector] || sector}<br/>
            总敞口: ¥${(d.value as number).toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}<br/>
            占比: ${(d.weight_pct as number).toFixed(2)}%<br/>
            持有基金:<br/>  ${fundList || '无'}`
        },
      },
      series: [
        {
          type: 'treemap',
          roam: false,
          nodeClick: 'link',
          breadcrumb: {
            show: true,
            height: 28,
            bottom: 0,
            itemStyle: {
              color: theme.surfaceHover,
              borderColor: theme.border,
              textStyle: { color: theme.text, fontSize: 11 },
            },
            emphasis: {
              itemStyle: { color: theme.surfaceHover },
            },
          },
          label: {
            show: true,
            formatter: (p: any) => {
              const name = p.name || ''
              const pct = p.data?.weight_pct ?? 0
              return `${name.length > 6 ? name.slice(0, 6) + '…' : name}\n${pct.toFixed(1)}%`
            },
            fontSize: 11,
            color: theme.text,
          },
          upperLabel: {
            show: true,
            height: 20,
            fontSize: 10,
            color: theme.textMuted,
          },
          itemStyle: {
            borderColor: theme.surface,
            borderWidth: 2,
            gapWidth: 2,
          },
          emphasis: {
            label: { fontSize: 13, fontWeight: 'bold' },
            itemStyle: { shadowBlur: 10, shadowColor: 'rgba(0,0,0,0.3)' },
          },
          levels: [
            {
              colorMappingBy: 'value',
              itemStyle: { gapWidth: 2 },
            },
          ],
          data: nodes,
        },
      ],
    }
  }, [data, dark])

  const chartRef = useEChart(treemapOption, [treemapOption])

  // ── Click event binding for treemap drill-down ─────────────────
  useEffect(() => {
    const dom = chartRef.current
    if (!dom) return
    const inst = getInstanceByDom(dom)
    if (!inst) return

    const handler = (params: any) => {
      if (params.data && params.data.stock_code) {
        setSelectedStock(params.data as unknown as PenetrationStock & { stock_code: string; held_by_funds: PenetrationFund[] })
      }
    }

    inst.off('click')
    inst.on('click', handler)

    return () => { inst.off('click', handler) }
  }, [treemapOption])

  // ── Placeholder helper ──────────────────────────────────────────
  const placeholder = (msg: string, testid: string) => (
    <div data-testid={testid} style={{ padding: '60px 0', display: 'flex', alignItems: 'center', justifyContent: 'center', color: theme.textMuted, fontVariantNumeric: 'tabular-nums' }}>
      <Text variant="secondary" as="span" size="sm">{msg}</Text>
    </div>
  )

  // ── Detail view: selected stock breakdown ──
  if (selectedStock) {
    const sector = classifySector(selectedStock.stock_name)
    const sectorColor = SECTOR_COLORS[sector] || SECTOR_COLORS.other
    const sectorName: Record<string, string> = {
      tech: '科技', financial: '金融', consumer: '消费', healthcare: '医疗',
      energy: '能源', industrial: '工业', materials: '材料', realestate: '房地产',
      utilities: '公用事业', telecom: '通信', communication: '通信',
      consumer_cyclical: '可选消费', consumer_defensive: '必选消费',
    }
    return (
      <Card dark={dark}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginBottom: 16 }}>
          <Button variant="secondary" size="sm" onClick={() => setSelectedStock(null)}
            style={{ padding: '4px 8px', minWidth: 32 }}>
            <CaretLeft size={16} />
          </Button>
          <div>
            <Text variant="heading3" as="h3">{selectedStock.stock_name}</Text>
            <Text variant="secondary" as="span" size="xs">
              {selectedStock.stock_code}
              <span style={{
                marginLeft: 8, fontSize: 10, fontWeight: 600,
                padding: '1px 6px', borderRadius: 3,
                background: sectorColor, color: '#fff',
              }}>
                {sectorName[sector] || sector}
              </span>
            </Text>
          </div>
        </div>

        <Card dark={dark} style={{ marginBottom: 16 }} padded={false}>
          <div style={{ padding: '16px 20px' }}>
            <div style={{ display: 'flex', gap: 32, flexWrap: 'wrap' }}>
              <div>
                <Text variant="secondary" as="span" size="xs">总敞口</Text>
                <div style={{ fontSize: 18, fontWeight: 600, color: theme.text }}>¥{selectedStock.total_exposure_cny.toLocaleString()}</div>
              </div>
              <div>
                <Text variant="secondary" as="span" size="xs">组合占比</Text>
                <div style={{ fontSize: 18, fontWeight: 600, color: sectorColor }}>{selectedStock.weight_pct.toFixed(2)}%</div>
              </div>
              <div>
                <Text variant="secondary" as="span" size="xs">持有基金数</Text>
                <div style={{ fontSize: 18, fontWeight: 600, color: theme.text }}>{selectedStock.held_by_funds.length}</div>
              </div>
            </div>
          </div>
        </Card>

        <Card dark={dark} padded={false}>
          <div style={{ padding: '16px 20px 4px' }}>
            <div style={{ fontSize: 16, fontWeight: 600, marginBottom: 12, color: theme.text }}>
              {t('portfolio.holdDetail', '持有明细')}
            </div>
          </div>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr style={{ borderBottom: `1px solid ${theme.border}` }}>
                <th style={{ textAlign: 'left', padding: '8px 20px', fontWeight: 600, color: theme.textMuted, fontSize: 11 }}>基金名称</th>
                <th style={{ textAlign: 'right', padding: '8px 20px', fontWeight: 600, color: theme.textMuted, fontSize: 11 }}>持仓权重</th>
                <th style={{ textAlign: 'right', padding: '8px 20px', fontWeight: 600, color: theme.textMuted, fontSize: 11 }}>基金市值</th>
                <th style={{ textAlign: 'right', padding: '8px 20px', fontWeight: 600, color: theme.textMuted, fontSize: 11 }}>敞口金额</th>
              </tr>
            </thead>
            <tbody>
              {[...selectedStock.held_by_funds]
                .sort((a, b) => b.weight_pct - a.weight_pct)
                .map(f => {
                  const exposure = f.fund_value_cny * (f.weight_pct / 100)
                  return (
                    <tr key={f.fund_code}
                      style={{ borderBottom: `1px solid ${theme.borderSubtle}` }}>
                      <td style={{ padding: '10px 20px', color: theme.text }}>{f.fund_name}</td>
                      <td style={{ padding: '10px 20px', textAlign: 'right', fontWeight: 600, color: sectorColor }}>{f.weight_pct.toFixed(2)}%</td>
                      <td style={{ padding: '10px 20px', textAlign: 'right', color: theme.text }}>¥{f.fund_value_cny.toLocaleString()}</td>
                      <td style={{ padding: '10px 20px', textAlign: 'right', fontWeight: 600, color: theme.text }}>¥{exposure.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 0 })}</td>
                    </tr>
                  )
                })}
            </tbody>
          </table>
        </Card>
      </Card>
    )
  }

  return (
    <Card dark={dark} style={{ marginBottom: 20 }}>
      <div style={{ padding: '4px 0 16px' }}>
        <Text variant="heading3" as="h3">{t('portfolio.penetrationTitle', '股权穿透')}</Text>
        <Text variant="secondary" as="span" size="xs" style={{ marginTop: 2, display: 'block' }}>
          {equityCount} {t('portfolio.equityCount', '只权益基金')} · {uniqueStocks} {t('portfolio.uniqueStocks', '只底层股票')} · {t('portfolio.totalValue', '组合总市值')} ¥{totalValue.toLocaleString()}
        </Text>
        <Text variant="secondary" as="span" size="xs" style={{ marginTop: 2, display: 'block' }}>
          {t('portfolio.penetrationDesc', '色块面积 = 敞口金额 | 点击查看基金明细 | 覆盖中美股票行业')}
        </Text>
      </div>
      {loading
        ? placeholder(t('common.loading', '加载中…'), 'chart-loading')
        : error
          ? placeholder(error, 'chart-error')
          : !data.length
            ? placeholder(t('common.noData', '暂无数据'), 'chart-empty')
            : <div ref={chartRef} style={{ height: 520 }} data-testid="treemap-chart" />}
    </Card>
  )
}
