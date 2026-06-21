import { useState, useEffect, type CSSProperties } from 'react'
import { useTranslation } from 'react-i18next'
import { Text } from '@cloudflare/kumo'
import { Scales } from '@phosphor-icons/react'
import { fetchPortfolios, type PortfolioDefinition } from '../api'

interface PortfolioSwitcherProps {
  activeId: number;
  onChange: (id: number) => void;
}

/** Full-width portfolio selector chip.
 *  v3.0: rendered as a consistent bordered chip whether there is one or many
 *  portfolios, so the sidebar header row stays visually stable. Multi-portfolio
 *  opens a dropdown of styled items. */
export default function PortfolioSwitcher({ activeId, onChange }: PortfolioSwitcherProps) {
  const { t } = useTranslation();
  const [portfolios, setPortfolios] = useState<PortfolioDefinition[]>([]);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const ctrl = new AbortController();
    fetchPortfolios(ctrl.signal)
      .then(setPortfolios)
      .catch(() => {});
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
    display: 'flex', alignItems: 'center', gap: 6, width: '100%',
    padding: '6px 10px', borderRadius: 6,
    border: '1px solid var(--color-kumo-border)',
    background: 'var(--color-kumo-surface)',
    fontSize: 12, fontWeight: 500,
    color: 'var(--text-color-kumo)',
  };

  // Single portfolio → static chip (no dropdown, no interaction affordance).
  if (single) {
    return (
      <div style={{ ...chipStyle, cursor: 'default' }}>
        <Scales size={14} style={{ flexShrink: 0 }} />
        <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{displayName}</span>
      </div>
    );
  }

  return (
    <div data-portfolio-switcher style={{ position: 'relative', width: '100%' }}>
      <button
        onClick={() => setOpen(v => !v)}
        style={{ ...chipStyle, cursor: 'pointer', textAlign: 'left' }}
      >
        <Scales size={14} style={{ flexShrink: 0 }} />
        <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{displayName}</span>
        <span style={{ fontSize: 10, marginLeft: 2 }}>{open ? '▲' : '▼'}</span>
      </button>
      {open && (
        <div style={{
          position: 'absolute', top: '100%', left: 0, right: 0, marginTop: 4,
          background: 'var(--color-kumo-surface)',
          border: '1px solid var(--color-kumo-border)',
          borderRadius: 8, boxShadow: '0 4px 12px rgba(0,0,0,0.15)',
          zIndex: 200, overflow: 'hidden',
        }}>
          {portfolios.map(p => (
            <button
              key={p.id}
              onClick={() => { onChange(p.id); setOpen(false); }}
              style={{
                display: 'block', width: '100%', textAlign: 'left',
                padding: '8px 14px', border: 'none',
                background: p.id === activeId ? 'var(--color-kumo-canvas)' : 'transparent',
                cursor: 'pointer', fontSize: 13, fontWeight: p.id === activeId ? 600 : 400,
                color: 'var(--text-color-kumo)',
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
