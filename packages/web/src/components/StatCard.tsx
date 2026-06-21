import { memo } from 'react'
import { LayerCard, Text } from '@cloudflare/kumo'

// 红涨绿跌（国内惯例）
interface Props { label: string; value: string; color?: 'up' | 'down'; sub?: string }

export default memo(function StatCard({ label, value, color, sub }: Props) {
  return (
    <LayerCard>
      <div style={{ padding: '16px 20px' }}>
        <Text variant="secondary" as="span" size="xs">{label}</Text>
        <div style={{ marginTop: 6 }}>
          <Text variant="heading2" as="span" style={color ? {
            color: color === 'up' ? '#d63649' : '#199c63',
            fontWeight: 700, fontSize: 20,
          } : undefined}>{value}</Text>
        </div>
        {sub && <div style={{ marginTop: 4 }}><Text variant="secondary" as="span" size="xs">{sub}</Text></div>}
      </div>
    </LayerCard>
  );
});
