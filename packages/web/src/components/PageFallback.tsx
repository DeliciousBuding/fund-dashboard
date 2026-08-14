import { Text, Grid } from '@cloudflare/kumo';
import { useTranslation } from 'react-i18next';
import { useAppStore } from '../stores/appStore';
import { getTheme, radius, skeleton, space } from '../styles/theme';
import { Card } from './ui/Card';
import ChartFallback from './ChartFallback';

/** Page-level loading shell: 4-up stat skeletons + chart skeleton (#116). */
export default function PageFallback({ labelKey = 'overview.loading' }: { labelKey?: string }) {
  const { t } = useTranslation();
  const dark = useAppStore((s) => s.dark);
  const theme = getTheme(dark);

  return (
    <div aria-busy="true" aria-live="polite">
      <div style={{ marginBottom: space[5] }}>
        <div
          className="fd-skeleton-bar"
          style={{
            height: skeleton.barH.lg,
            width: skeleton.barW.lg,
            borderRadius: radius.sm,
            marginBottom: space[3],
          }}
        />
        <Text variant="secondary" as="span" size="sm">{t(labelKey)}</Text>
      </div>
      <Grid variant="4up" gap="base" style={{ marginBottom: space[5] }}>
        {Array.from({ length: 4 }).map((_, i) => (
          <Card key={i} dark={dark} glass padded={false}>
            <div style={{ padding: `${space[4]}px ${space[5]}px`, minHeight: skeleton.statMinH }}>
              <div
                className="fd-skeleton-bar"
                style={{
                  height: skeleton.barH.sm,
                  width: skeleton.barW.xs,
                  borderRadius: radius.sm,
                }}
              />
              <div
                className="fd-skeleton-bar"
                style={{
                  marginTop: space[3],
                  height: skeleton.barH.lg,
                  width: skeleton.barW.sm,
                  borderRadius: radius.sm,
                }}
              />
            </div>
          </Card>
        ))}
      </Grid>
      <ChartFallback />
    </div>
  );
}
