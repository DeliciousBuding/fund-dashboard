import { useTranslation } from 'react-i18next'
import { CurrencyDollar } from '@phosphor-icons/react'
import { getTheme, radius, space, fontSize } from '../styles/theme'
import { useAppStore } from '../stores/appStore'
import type { ExchangeRate } from '../api'

/** Shared USD/CNY chip used on overview/detail/compare/nasdaq headers (#108/#186). */
export default function ExchangeRateBadge({ exchangeRate }: { exchangeRate: ExchangeRate | null }) {
  const { t } = useTranslation()
  const dark = useAppStore((s) => s.dark)
  const theme = getTheme(dark)
  if (!exchangeRate) return null
  return (
    <span
      style={{
        fontSize: fontSize.sm,
        color: theme.textSubtle,
        display: 'inline-flex',
        alignItems: 'center',
        gap: space[1] / 2,
        padding: `2px ${space[2]}px`,
        borderRadius: radius.sm,
        background: theme.surfaceHover,
        whiteSpace: 'nowrap',
      }}
    >
      <CurrencyDollar size={12} weight="bold" aria-hidden />
      {t('common.usdCny')} {exchangeRate.rate.toFixed(4)}
    </span>
  )
}
