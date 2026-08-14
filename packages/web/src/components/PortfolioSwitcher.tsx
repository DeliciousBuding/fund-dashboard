import { useState, useEffect, type CSSProperties } from 'react'
import { useTranslation } from 'react-i18next'
import { Text } from '@cloudflare/kumo'
import { Scales } from '@phosphor-icons/react'
import { fetchPortfolios, type PortfolioDefinition } from '../api'
import { getTheme, glassSurfaceStyle, radius, space, fontSize, fontWeight, zIndex } from '../styles/theme'
import { useAppStore } from '../stores/appStore'

interface PortfolioSwitcherProps {
  activeId: number;
  onChange: (id: number) => void;
}

/** Full-width portfolio selector chip.
 *  v3.0: rendered as a consistent bordered chip whether there is one or many
 *  portfolios, so the sidebar header row stays visually stable. Multi-portfolio
 *  opens a dropdown of styled items. URL sync is owned by App via usePortfolioDeepLink. */
export default function PortfolioSwitcher({ activeId, onChange }: PortfolioSwitcherProps) {
  const { t } = useTranslation();
  const dark = useAppStore((s) => s.dark);
  const theme = getTheme(dark);
  const [portfolios, setPortfolios] = useState<PortfolioDefinition[]>([]);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const ctrl = new AbortController();
    fetchPortfolios(ctrl.signal)
      .then(setPortfolios)
      .catch((e: any) => { if (e?.name !== 'AbortError') console.warn('[portfolioSwitcher]', e); });
    return () => ctrl.abort();
  }, []);

  // Close on outside click
  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      if (!target.closest('[data-portfolio-switcher]')) setOpen(false);
    };
    document.addEventListener('click', handler);
    return () => document.removeEventListener('click', handler);
  }, [open]);

  const active = portfolios.find(p => p.id === activeId);
  const displayName = active?.name || t('portfolio.default');
  const single = portfolios.length <= 1;

  const chipStyle: CSSProperties = {
    display: 'flex', alignItems: 'center', gap: space[2], width: '100%',
    padding: `${space[2] - 2}px ${space[3] - 2}px`,
    ...glassSurfaceStyle(theme, { borderRadius: radius.sm }),
    fontSize: fontSize.md, fontWeight: fontWeight.medium,
    color: theme.text,
  };

  // Single portfolio → static chip (no dropdown, no interaction affordance).
  if (single) {
    return (
      <div style={{ ...chipStyle, cursor: 'default' }} aria-label={displayName}>
        <Scales size={14} style={{ flexShrink: 0 }} aria-hidden />
        <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{displayName}</span>
      </div>
    );
  }

  return (
    <div data-portfolio-switcher style={{ position: 'relative', width: '100%' }}>
      <button
        type="button"
        aria-haspopup="listbox"
        aria-expanded={open}
        onClick={() => setOpen(v => !v)}
        style={{ ...chipStyle, cursor: 'pointer', textAlign: 'left' }}
      >
        <Scales size={14} style={{ flexShrink: 0 }} aria-hidden />
        <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{displayName}</span>
        <span style={{ fontSize: fontSize.xs, marginLeft: 2 }} aria-hidden>{open ? '▲' : '▼'}</span>
      </button>
      {open && (
        <div
          role="listbox"
          aria-label={t('portfolio.default')}
          style={{
            position: 'absolute', top: '100%', left: 0, right: 0, marginTop: space[1],
            ...glassSurfaceStyle(theme, { borderRadius: radius.md }),
            zIndex: zIndex.sticky, overflow: 'hidden',
          }}
        >
          {portfolios.map(p => (
            <button
              type="button"
              key={p.id}
              role="option"
              aria-selected={p.id === activeId}
              onClick={() => { onChange(p.id); setOpen(false); }}
              style={{
                display: 'block', width: '100%', textAlign: 'left',
                padding: `${space[2]}px ${space[4] - 2}px`, border: 'none',
                background: p.id === activeId ? 'var(--color-kumo-canvas)' : 'transparent',
                cursor: 'pointer', fontSize: fontSize.base, fontWeight: p.id === activeId ? fontWeight.semibold : fontWeight.regular,
                color: theme.text,
                borderBottom: '1px solid var(--color-kumo-border)',
              }}
            >
              {p.name}
              {p.description && (
                <Text variant="secondary" as="span" size="xs" style={{ display: 'block', marginTop: 1 }}>
                  {p.description}
                </Text>
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
