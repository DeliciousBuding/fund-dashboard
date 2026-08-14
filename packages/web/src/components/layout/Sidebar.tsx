import { useMemo, Suspense, lazy, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useLocation } from 'react-router-dom'
import { Sidebar, Text, Button, Switch, Input } from '@cloudflare/kumo'
import { ChartBar, House, TrendUp, Funnel, MagnifyingGlassIcon, Sun, Moon, Building, Globe, Scales, X } from '@phosphor-icons/react'
import type { FundInfo, Portfolio, SecurityInfo } from '../../api'
import { classify, classifySecurity, CATS, CAT_ORDER, STOCK_CATS, STOCK_CAT_ORDER } from '../../services/classify'
import { fmtShort } from '../../services/format'
import LanguageSwitcher from '../LanguageSwitcher'
import EmptyState from '../feedback/EmptyState'
import { getTheme, space, radius, fontSize, fontWeight, lineHeight } from '../../styles/theme'

const PortfolioSwitcher = lazy(() => import('../PortfolioSwitcher'));

/** Short market badge i18n keys (category.沪/深/港/美) */
const MARKET_BADGE_KEYS: Record<string, string> = {
  sh: 'category.沪',
  sz: 'category.深',
  hk: 'category.港',
  us: 'category.美',
};

/** Icon for a security based on its type and category */
function securityIcon(cat: string, isStock: boolean) {
  if (isStock && (cat === 'stock-a' || cat === 'stock-hk')) return <Building size={18} />;
  if (isStock && cat === 'stock-us') return <Globe size={18} />;
  if (cat === 'nasdaq') return <Globe size={18} />;
  return <TrendUp size={18} />;
}

/** Map route paths back to the concept of "active code" for sidebar highlighting.
 *  /          → 'overview'
 *  /compare   → 'compare'
 *  /nasdaq    → 'nasdaq-overview'
 *  /fund/XXXX → 'XXXX'
 */
function activeCodeFromPath(pathname: string): string {
  if (pathname === '/') return 'overview';
  if (pathname === '/compare') return 'compare';
  if (pathname === '/nasdaq') return 'nasdaq-overview';
  const m = pathname.match(/^\/fund\/(.+)$/);
  if (m) return decodeURIComponent(m[1]);
  return 'overview';
}

function matchesQuery(name: string, code: string, q: string): boolean {
  if (!q) return true;
  const n = name.toLowerCase();
  const c = code.toLowerCase();
  const qq = q.toLowerCase();
  return n.includes(qq) || c.includes(qq);
}

interface AppSidebarProps {
  funds: FundInfo[];
  securities: SecurityInfo[];
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
  funds, securities,
  heldOnly, onHeldToggle, searchQuery, onSearchChange,
  dark, onToggleDark, portfolio, portfolioId, onPortfolioChange,
}: AppSidebarProps) {
  const { t } = useTranslation();
  const theme = getTheme(dark);
  const navigate = useNavigate();
  const location = useLocation();
  const activeCode = activeCodeFromPath(location.pathname);
  const searchWrapRef = useRef<HTMLDivElement | null>(null);
  const focusSearch = () => {
    const el = searchWrapRef.current?.querySelector('input') as HTMLInputElement | null;
    el?.focus();
    el?.select?.();
  };
  const blurSearch = () => {
    const el = searchWrapRef.current?.querySelector('input') as HTMLInputElement | null;
    el?.blur();
  };
  const isSearchFocused = () => {
    const el = searchWrapRef.current?.querySelector('input');
    return !!el && document.activeElement === el;
  };

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

  const matchCount = useMemo(() => {
    let n = 0;
    const countItems = (items: Array<FundInfo | SecurityInfo>) => {
      for (const f of items) {
        if (heldOnly && !(f.held_shares > 0.001)) continue;
        if (!matchesQuery(f.name, f.code, searchQuery)) continue;
        n += 1;
      }
    };
    for (const items of Object.values(groups)) countItems(items);
    for (const items of Object.values(stockGroups)) countItems(items);
    return n;
  }, [groups, stockGroups, heldOnly, searchQuery]);

  const hasAnyVisible = matchCount > 0;

  // `/` focuses search when not typing in another field; Esc clears when focused.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null;
      const tag = target?.tagName?.toLowerCase();
      const editable = tag === 'input' || tag === 'textarea' || tag === 'select' || target?.isContentEditable;
      if (e.key === '/' && !editable && !e.metaKey && !e.ctrlKey && !e.altKey) {
        e.preventDefault();
        focusSearch();
        return;
      }
      if (e.key === 'Escape' && isSearchFocused()) {
        if (searchQuery) {
          e.preventDefault();
          onSearchChange('');
        } else {
          blurSearch();
        }
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onSearchChange, searchQuery]);

  /** Navigate to a fund/security detail or overview page */
  const handleSelect = (code: string) => {
    switch (code) {
      case 'overview': navigate('/'); break;
      case 'compare': navigate('/compare'); break;
      case 'nasdaq-overview': navigate('/nasdaq'); break;
      default: navigate(`/fund/${encodeURIComponent(code)}`); break;
    }
  };

  /** Render a category group in the sidebar */
  const renderCategoryGroup = (
    cat: string,
    items: Array<FundInfo | SecurityInfo>,
    nameKey: string,
  ) => {
    const localizedCatName = t(nameKey || cat);
    const filtered = (heldOnly ? items.filter(f => f.held_shares > 0.001) : items)
      .filter(f => matchesQuery(f.name, f.code, searchQuery))
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
            <span style={{ marginLeft: 8, fontSize: fontSize.sm, fontWeight: fontWeight.semibold, color: catPnl > 0 ? theme.up : theme.down }}>
              {catPnl > 0 ? '+' : ''}{catPnl.toFixed(0)} ({catPct > 0 ? '+' : ''}{catPct.toFixed(1)}%)
            </span>
          )}
        </Sidebar.GroupLabel>
        <Sidebar.Menu>
          {isNasdaq && (
            <Sidebar.MenuButton icon={<img src={dark ? "/ndaq-d.svg" : "/ndaq.svg"} width={18} height={18} style={{ borderRadius: radius.xs }} alt="" aria-hidden />} active={activeCode === 'nasdaq-overview'} onClick={() => handleSelect('nasdaq-overview')}>
              {t('nav.nasdaqOverview')} ({nasdaqFunds.filter(f => f.held_shares > 0.001).length} {t('tx.trades')})
            </Sidebar.MenuButton>
          )}
          {filtered.map(f => {
            const fp = f.unrealized_pnl ?? 0;
            const sec = f as SecurityInfo;
            const isStock = isStockCat && 'security_type' in f && sec.security_type === 'stock';
            const badgeKey = sec.market ? MARKET_BADGE_KEYS[sec.market] : undefined;
            return (
              <Sidebar.MenuButton key={f.code} icon={securityIcon(cat, isStock)} active={activeCode === f.code} onClick={() => handleSelect(f.code)}>
                <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{f.name}</span>
                {isStock && badgeKey && (
                  <span style={{
                    marginRight: space[1], fontSize: fontSize.xs, fontWeight: fontWeight.semibold, padding: `1px ${space[1] + 1}px`, borderRadius: radius.xs,
                    background: sec.market === 'hk' ? theme.amber : sec.market === 'us' ? theme.blue : theme.up,
                    color: theme.onAccent, lineHeight: lineHeight.badge,
                  }}>{t(badgeKey)}</span>
                )}
                <Sidebar.MenuBadge><span style={{ fontSize: fontSize.sm, fontWeight: fontWeight.semibold, color: fp > 0 ? theme.up : fp < 0 ? theme.down : 'var(--text-color-kumo-subtle)' }}>{fmtShort(fp)}</span></Sidebar.MenuBadge>
              </Sidebar.MenuButton>
            );
          })}
        </Sidebar.Menu>
      </Sidebar.Group>
    );
  };

  return (
    <Sidebar style={{ display: 'flex', flexDirection: 'column', height: '100%', overflow: 'hidden' }}>
      <Sidebar.Header style={{ height: 'auto', minHeight: 0, flexShrink: 0, overflow: 'visible', paddingBlock: 14 }}>
        <div style={{ display: 'flex', flexDirection: 'column', width: '100%', gap: space[3] }}>
          {/* Row 1 — brand + window controls (language / theme) */}
          <div style={{ display: 'flex', alignItems: 'center', gap: space[2] + 2, width: '100%' }}>
            <ChartBar size={22} weight="fill" style={{ flexShrink: 0 }} />
            <div style={{ flex: 1, minWidth: 0 }}>
              <Text variant="heading3" as="span" style={{ display: 'block', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{t('nav.title')}</Text>
              <Text variant="secondary" as="span" size="xs" style={{ display: 'block', marginTop: 2, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{t('nav.subtitle')}</Text>
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: space[1], flexShrink: 0 }}>
              <LanguageSwitcher />
              <Button type="button" variant="secondary" size="sm" onClick={onToggleDark} title={dark ? t('nav.lightMode') : t('nav.darkMode')} aria-label={dark ? t('nav.lightMode') : t('nav.darkMode')} style={{ padding: space[2] - 2, minWidth: 32 }}>
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
          <Sidebar.MenuButton icon={<House size={18} />} active={activeCode === 'overview'} onClick={() => handleSelect('overview')}>{t('nav.overview')}</Sidebar.MenuButton>
          <Sidebar.MenuButton icon={<Scales size={18} />} active={activeCode === 'compare'} onClick={() => handleSelect('compare')}>{t('nav.compare')}</Sidebar.MenuButton>
        </Sidebar.Menu>
        <Sidebar.Menu>
          <div style={{ display: 'flex', alignItems: 'center', gap: space[2] + 2, padding: `${space[2] - 2}px ${space[4]}px` }}>
            <Funnel size={16} />
            <Switch checked={heldOnly} onCheckedChange={onHeldToggle} />
            <Text variant="secondary" as="span" size="xs">{heldOnly ? t('nav.heldOnly') : t('nav.showAll')}</Text>
          </div>
        </Sidebar.Menu>
        <Sidebar.Menu>
          <div style={{ padding: `${space[1]}px ${space[4]}px` }}>
            <div ref={searchWrapRef} style={{ position: 'relative' }}>
              <Input
                placeholder={t('nav.search')}
                value={searchQuery}
                onChange={e => onSearchChange((e.target as HTMLInputElement).value)}
                prefix={<MagnifyingGlassIcon size={14} />}
                size="sm"
                aria-label={t('nav.search')}
              />
              {searchQuery ? (
                <button
                  type="button"
                  onClick={() => onSearchChange('')}
                  aria-label={t('empty.searchClear')}
                  style={{
                    position: 'absolute',
                    right: space[2],
                    top: '50%',
                    transform: 'translateY(-50%)',
                    border: 'none',
                    background: 'transparent',
                    cursor: 'pointer',
                    padding: space[1],
                    color: theme.textMuted,
                    display: 'inline-flex',
                    alignItems: 'center',
                  }}
                >
                  <X size={14} aria-hidden />
                </button>
              ) : null}
            </div>
            {searchQuery ? (
              <Text variant="secondary" as="span" size="xs" style={{ display: 'block', marginTop: space[1] }} className="fd-tabular-nums">
                {t('nav.searchMatches', { count: matchCount })}
              </Text>
            ) : null}
          </div>
        </Sidebar.Menu>

        {/* Fund categories */}
        {CAT_ORDER.map(cat => {
          const list = groups[cat]; if (!list?.length) return null;
          const cfg = CATS[cat];
          return renderCategoryGroup(cat, list, cfg?.nameKey || cat);
        })}

        {/* Stock categories */}
        {STOCK_CAT_ORDER.map(cat => {
          const list = stockGroups[cat]; if (!list?.length) return null;
          const cfg = STOCK_CATS[cat];
          return renderCategoryGroup(cat, list, cfg?.nameKey || cat);
        })}

        {!hasAnyVisible && (
          <div style={{ padding: `${space[3]}px ${space[3]}px` }}>
            {searchQuery ? (
              <EmptyState
                compact
                title={t('empty.searchTitle')}
                description={t('empty.searchDesc', { query: searchQuery })}
                actionLabel={t('empty.searchClear')}
                onAction={() => onSearchChange('')}
              />
            ) : heldOnly ? (
              <EmptyState
                compact
                title={t('empty.heldTitle')}
                description={t('empty.heldDesc')}
                actionLabel={t('empty.heldAction')}
                onAction={onHeldToggle}
              />
            ) : null}
          </div>
        )}
      </Sidebar.Content>
      <Sidebar.Footer>
        <Text variant="secondary" as="span" size="xs">{portfolio?.held_funds ?? '-'} {t('nav.holdingsFooter', { pnl: (pnl > 0 ? '+' : '') + pnl.toFixed(0) })}</Text>
      </Sidebar.Footer>
    </Sidebar>
  );
}
