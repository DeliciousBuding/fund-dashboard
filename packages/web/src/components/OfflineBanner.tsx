import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'

/** Banner shown across the top of the page when the browser is offline. */
export default function OfflineBanner() {
  const { t } = useTranslation()
  const [offline, setOffline] = useState(!navigator.onLine)

  useEffect(() => {
    const goOffline = () => setOffline(true)
    const goOnline = () => setOffline(false)
    window.addEventListener('offline', goOffline)
    window.addEventListener('online', goOnline)
    return () => {
      window.removeEventListener('offline', goOffline)
      window.removeEventListener('online', goOnline)
    }
  }, [])

  if (!offline) return null

  return (
    <div
      role="alert"
      aria-live="polite"
      style={{
        position: 'fixed',
        top: 0,
        left: 0,
        right: 0,
        zIndex: 9999,
        background: '#d63649',
        color: '#fff',
        textAlign: 'center',
        padding: '8px 16px',
        fontSize: 14,
        fontWeight: 500,
        letterSpacing: 0.3,
        backdropFilter: 'blur(4px)',
      }}
    >
      {t('error.offline')}
    </div>
  )
}
