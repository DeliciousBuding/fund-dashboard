import { useMemo, Suspense, lazy } from 'react'
import { useTranslation } from 'react-i18next'
import { Sidebar, Text, Button, Switch, Input } from '@cloudflare/kumo'
import { ChartBar, House, TrendUp, Funnel, MagnifyingGlassIcon, Sun, Moon, Building, Globe, Scales } from '@phosphor-icons/react'
import type { FundInfo, Portfolio, SecurityInfo, Market } from '../../api'
import { classify, classifySecurity, CATS, CAT_ORDER, STOCK_CATS, STOCK_CAT_ORDER } from '../../utils'
import { fmtShort } from '../../utils'
import LanguageSwitcher from '../LanguageSwitcher'

const PortfolioSwitcher = lazy(() => import('../PortfolioSwitcher'));

/** Human-readable market badge label (uses i18n keys) */
function marketLabel(m: Market): string {
  switch (m) {
    case 'sh': return '沪';
    case 'sz': return '深';
    case 'hk': return '港';
    case 'us': return '美';
    default: return m;
  }
}

/** Map internal category keys to i18n keys */
const CAT_I18N: Record<string, string> = {
  nasdaq: 'category.nasdaq',
  tech: 'category.tech',
  dividend: 'category.dividend',
  gold: 'category.gold',
  bond: 'category.bond',
  qdii: 'category.qdii',
  money: 'category.money',
  ashare: 'category.ashare',
  hkstock: 'category.hkstock',
  other: 'category.other',
  'stock-a': 'category.stockA',
  'stock-hk': 'category.stockHk',
  'stock-us': 'category.stockUs',
};

/** Icon for a security based on its type and category */
function securityIcon(cat: string, isStock: boolean) {
  if (isStock && (cat === 'stock-a' || cat === 'stock-hk')) return <Building size={18} />;
  if (isStock && cat === 'stock-us') return <Globe size={18} />;
  if (cat === 'nasdaq') return <Globe size={18} />;
  return <TrendUp size={18} />;
}

interface AppSidebarProps {
  funds: FundInfo[];
  securities: SecurityInfo[];
  activeCode: string;
  onSelect: (code: string) => void;
  heldOnly: boolean;
  onHeldToggle: () => void;
  searchQuery: string;
  onSearchChange: (q: string) => void;
  dark: boolean;
  onToggleDark: () => void;
  portfolio: Portfolio | null;
  portfolioId: number;
  onPortfolioChange: (id: number) => void;
}

export default function AppSidebar({
  funds, securities, activeCode, onSelect,
  heldOnly, onHeldToggle, searchQuery, onSearchChange,
  dark, onToggleDark, portfolio, portfolioId, onPortfolioChange,
}: AppSidebarProps) {
  const { t } = useTranslation();
  const groups = useMemo(() => {
    const g: Record<string, FundInfo[]> = {};
    for (const f of funds) { const cat = classify(f); if (!g[cat]) g[cat] = []; g[cat].push(f); }
    return g;
  }, [funds]);

  const stockGroups = useMemo(() => {
    const g: Record<string, SecurityInfo[]> = {};
    for (const s of securities) {
      const cat = classifySecurity(s);
      if (!g[cat]) g[cat] = [];
      g[cat].push(s);
    }
    return g;
  }, [securities]);

  const nasdaqFunds = useMemo(() => groups['nasdaq'] || [], [groups]);
  const pnl = portfolio?.unrealized_pnl ?? 0;

  /** Render a category group in the sidebar */
  const renderCategoryGroup = (
    cat: string,
    items: Array<FundInfo | SecurityInfo>,
    catName: string,
  ) => {
    const localizedCatName = t(CAT_I18N[cat] || cat, catName);
    const filtered = (heldOnly ? items.filter(f => f.held_shares > 0.001) : items)
      .filter(f => !searchQuery || f.name.includes(searchQuery) || f.code.includes(searchQuery))
      .sort((a, b) => (b.unrealized_pnl ?? 0) - (a.unrealized_pnl ?? 0));
    if (!filtered.length) return null;

    const catPnl = filtered.reduce((s, f) => s + (f.unrealized_pnl || 0), 0);
    const catCost = filtered.reduce((s, f) => s + Math.abs((f.current_value || 0) - (f.unrealized_pnl || 0)), 0);
    const catPct = catCost > 0 ? (catPnl / catCost * 100) : 0;
    const isStockCat = STOCK_CAT_ORDER.includes(cat);
    const isNasdaq = cat === 'nasdaq';

    return (
      <Sidebar.Group key={cat}>
        <Sidebar.GroupLabel>
          {localizedCatName} ({filtered.length})
          {catPnl !== 0 && (
            <span style={{ marginLeft: 8, fontSize: 11, fontWeight: 600, color: catPnl > 0 ? '#d63649' : '#199c63' }}>
              {catPnl > 0 ? '+' : ''}{catPnl.toFixed(0)} ({catPct > 0 ? '+' : ''}{catPct.toFixed(1)}%)
            </span>
          )}
        </Sidebar.GroupLabel>
        <Sidebar.Menu>
          {isNasdaq && (
            <Sidebar.MenuButton icon={<img src={dark ? "/ndaq-d.svg" : "/ndaq.svg"} width={18} height={18} style={{ borderRadius: 2 }} />} active={activeCode === 'nasdaq-overview'} onClick={() => onSelect('nasdaq-overview')}>
              {t('nav.nasdaqOverview')} ({nasdaqFunds.filter(f => f.held_shares > 0.001).length} {t('tx.trades')})
            </Sidebar.MenuButton>
          )}
          {filtered.map(f => {
            const fp = f.unrealized_pnl ?? 0;
            const sec = f as SecurityInfo;
            const isStock = isStockCat && 'security_type' in f && sec.security_type === 'stock';
            return (
              <Sidebar.MenuButton key={f.code} icon={securityIcon(cat, isStock)} active={activeCode === f.code} onClick={() => onSelect(f.code)}>
                <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{f.name}</span>
                {isStock && sec.market && (
                  <span style={{
                    marginRight: 4, fontSize: 10, fontWeight: 600, padding: '1px 5px', borderRadius: 3,
                    background: sec.market === 'hk' ? '#e05206' : sec.market === 'us' ? '#3172d9' : '#d63649',
                    color: '#fff', lineHeight: '16px',
                  }}>{marketLabel(sec.market)}</span>
                )}
                <Sidebar.MenuBadge><span style={{ fontSize: 11, fontWeight: 600, color: fp > 0 ? '#d63649' : fp < 0 ? '#199c63' : 'var(--text-color-kumo-subtle)' }}>{fmtShort(fp)}</span></Sidebar.MenuBadge>
              </Sidebar.MenuButton>
            );
          })}
        </Sidebar.Menu>
      </Sidebar.Group>
    );
  };

  return (
    <Sidebar style={{ display: 'flex', flexDirection: 'column', height: '100%', overflow: 'hidden' }}>
      {/* Sidebar.Header defaults to h-[58px] + overflow-hidden (Kumo), which
          clipped the two-row layout into a cramped blob. Override to auto height
          + visible overflow so the title row and portfolio chip each get room. */}
      <Sidebar.Header style={{ height: 'auto', minHeight: 0, flexShrink: 0, overflow: 'visible', paddingBlock: 14 }}>
        <div style={{ display: 'flex', flexDirection: 'column', width: '100%', gap: 12 }}>
          {/* Row 1 — brand + window controls (language / theme) */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, width: '100%' }}>
            <ChartBar size={22} weight="fill" style={{ flexShrink: 0 }} />
            <div style={{ flex: 1, minWidth: 0 }}>
              <Text variant="heading3" as="span" style={{ display: 'block', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{t('nav.title')}</Text>
              <Text variant="secondary" as="span" size="xs" style={{ display: 'block', marginTop: 2, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{t('nav.subtitle')}</Text>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 4, flexShrink: 0 }}>
              <LanguageSwitcher />
              <Button variant="secondary" size="sm" onClick={onToggleDark} title={dark ? t('nav.lightMode') : t('nav.darkMode')} aria-label={dark ? t('nav.lightMode') : t('nav.darkMode')} style={{ padding: 6, minWidth: 32 }}>
                {dark ? <Sun size={18} weight="bold" /> : <Moon size={18} weight="bold" />}
              </Button>
            </div>
          </div>
          {/* Row 2 — active portfolio (full-width chip) */}
          <Suspense fallback={null}>
            <PortfolioSwitcher activeId={portfolioId} onChange={onPortfolioChange} />
          </Suspense>
        </div>
      </Sidebar.Header>
      <Sidebar.Content style={{ flex: 1, overflowY: 'auto', overflowX: 'hidden' }}>
        <Sidebar.Menu>
          <Sidebar.MenuButton icon={<House size={18} />} active={activeCode === 'overview'} onClick={() => onSelect('overview')}>{t('nav.overview')}</Sidebar.MenuButton>
          <Sidebar.MenuButton icon={<Scales size={18} />} active={activeCode === 'compare'} onClick={() => onSelect('compare')}>{t('nav.compare')}</Sidebar.MenuButton>
        </Sidebar.Menu>
        <Sidebar.Menu>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '6px 16px' }}>
            <Funnel size={16} />
            <Switch checked={heldOnly} onCheckedChange={onHeldToggle} />
            <Text variant="secondary" as="span" size="xs">{heldOnly ? t('nav.heldOnly') : t('nav.showAll')}</Text>
          </div>
        </Sidebar.Menu>
        <Sidebar.Menu>
          <div style={{ padding: '4px 16px' }}>
            <Input placeholder={t('nav.search')} value={searchQuery} onChange={e => onSearchChange((e.target as HTMLInputElement).value)}
              prefix={<MagnifyingGlassIcon size={14} />} size="sm" />
          </div>
        </Sidebar.Menu>

        {/* Fund categories */}
        {CAT_ORDER.map(cat => {
          const list = groups[cat]; if (!list?.length) return null;
          const cfg = CATS[cat];
          return renderCategoryGroup(cat, list, cfg?.name || cat);
        })}

        {/* Stock categories */}
        {STOCK_CAT_ORDER.map(cat => {
          const list = stockGroups[cat]; if (!list?.length) return null;
          const cfg = STOCK_CATS[cat];
          return renderCategoryGroup(cat, list, cfg?.name || cat);
        })}
      </Sidebar.Content>
      <Sidebar.Footer>
        <Text variant="secondary" as="span" size="xs">{portfolio?.held_funds ?? '-'} {t('nav.holdingsFooter', { pnl: (pnl > 0 ? '+' : '') + pnl.toFixed(0) })}</Text>
      </Sidebar.Footer>
    </Sidebar>
  );
}
