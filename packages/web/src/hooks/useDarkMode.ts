import { useState, useEffect, useCallback } from 'react'

export function useDarkMode() {
  const [dark, setDark] = useState(() => {
    try {
      const stored = localStorage.getItem('fund-dark-mode')
      if (stored !== null) return stored === 'true'
    } catch {}
    return window.matchMedia('(prefers-color-scheme: dark)').matches
  })

  useEffect(() => {
    // data-theme is the DESIGN/index.css SSOT; keep data-mode for backward compat (#104).
    const mode = dark ? 'dark' : 'light'
    document.documentElement.setAttribute('data-theme', mode)
    document.documentElement.setAttribute('data-mode', mode)
    try { localStorage.setItem('fund-dark-mode', String(dark)) } catch {}
  }, [dark])

  const toggle = useCallback(() => setDark(d => !d), [])

  return { dark, toggle }
}
