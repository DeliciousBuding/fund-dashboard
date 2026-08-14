import { useState, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { useQueryClient } from '@tanstack/react-query'
import { useAppStore } from '../stores/appStore'
import { useToast } from './feedback/Toast'
import { getTheme, space, radius, fontSize, fontWeight, zIndex, letterSpacing, glassSurfaceStyle } from '../styles/theme'

/** Banner shown across the top of the page when the browser is offline. */
export default function OfflineBanner() {
  const { t } = useTranslation()
  const dark = useAppStore((s) => s.dark)
  const theme = getTheme(dark)
  const toast = useToast()
  const queryClient = useQueryClient()
  const [offline, setOffline] = useState(!navigator.onLine)
  const wasOffline = useRef(offline)

  useEffect(() => {
    const goOffline = () => {
      setOffline(true)
      wasOffline.current = true
    }
    const goOnline = () => {
      setOffline(false)
      if (wasOffline.current) {
        toast.success(t('toast.online'))
        void queryClient.invalidateQueries()
      }
      wasOffline.current = false
    }
    window.addEventListener('offline', goOffline)
    window.addEventListener('online', goOnline)
    return () => {
      window.removeEventListener('offline', goOffline)
      window.removeEventListener('online', goOnline)
    }
  }, [toast, t, queryClient])

  if (!offline) return null

  return (
    <div
      role="alert"
      aria-live="assertive"
      className="fd-offline-banner"
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        zIndex: zIndex.banner,
        ...glassSurfaceStyle(theme, { borderRadius: radius.none, material: 'thick' }),
        background: `color-mix(in srgb, ${theme.critical} 88%, transparent)`,
        color: theme.onAccent,
        borderBottom: `1px solid ${theme.critical}`,
        textAlign: 'center',
        padding: `${space[2]}px ${space[4]}px`,
        paddingTop: 'calc(8px + env(safe-area-inset-top, 0px))',
        fontSize: fontSize.lg,
        fontWeight: fontWeight.medium,
        letterSpacing: letterSpacing.wide,
      }}
    >
      {t('error.offline', { defaultValue: t('toast.offline') })}
    </div>
  )
}
