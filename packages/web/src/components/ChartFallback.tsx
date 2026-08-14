import { Text } from '@cloudflare/kumo';
import { useTranslation } from 'react-i18next';
import { useAppStore } from '../stores/appStore';
import { getTheme, radius, skeleton, space } from '../styles/theme';
import { Card } from './ui/Card';

/** Shared loading shell for lazy chart/page chunks (#112/#116).
 *  Skeleton shape approximates ChartShell card to reduce CLS. */
export default function ChartFallback({
  labelKey = 'overview.loadingChart',
  height = skeleton.chartH,
}: {
  labelKey?: string;
  height?: number;
}) {
  const { t } = useTranslation();
  const dark = useAppStore((s) => s.dark);
  const theme = getTheme(dark);

  return (
    <Card dark={dark} glass style={{ marginBottom: space[5] }} aria-busy="true" aria-live="polite">
      <div style={{ padding: `${space[1]}px 0 ${space[4]}px` }}>
        <div
          style={{
            height: skeleton.barH.md,
            width: skeleton.barW.md,
            borderRadius: radius.sm,
            background: theme.surfaceHover,
            marginBottom: space[2],
          }}
        />
        <div
          style={{
            height: skeleton.barH.sm,
            width: skeleton.barW.xl,
            borderRadius: radius.sm,
            background: theme.borderSubtle,
          }}
        />
      </div>
      <div
        data-testid="chart-loading"
        className="fd-skeleton-bar"
        style={{
          height,
          borderRadius: radius.md,
          // Override global skeleton wash with theme-aware surface gradient.
          background: `linear-gradient(90deg, ${theme.surfaceHover} 0%, ${theme.borderSubtle} 50%, ${theme.surfaceHover} 100%)`,
          backgroundSize: '200% 100%',
          display: 'flex',
          alignItems: 'flex-end',
          justifyContent: 'center',
          paddingBottom: space[4],
        }}
      >
        <Text variant="secondary" as="span" size="sm">
          {t(labelKey)}
        </Text>
      </div>
    </Card>
  );
}
