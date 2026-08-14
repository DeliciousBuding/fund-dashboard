import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import './i18n'
import App from './App'
import './index.css'
import { ToastProvider } from './components/feedback/Toast'

const CHUNK_RELOAD_KEY = 'fund-dashboard:chunk-reload-attempted'
const CHUNK_ERROR_PATTERNS = [
  'Failed to fetch dynamically imported module',
  'Importing a module script failed',
  'error loading dynamically imported module',
]

function isChunkLoadError(message: string): boolean {
  return CHUNK_ERROR_PATTERNS.some((pattern) => message.includes(pattern))
}

async function reloadAfterChunkError() {
  const lastAttempt = Number(sessionStorage.getItem(CHUNK_RELOAD_KEY) || '0')
  if (Date.now() - lastAttempt < 60_000) return
  sessionStorage.setItem(CHUNK_RELOAD_KEY, String(Date.now()))

  if ('serviceWorker' in navigator) {
    const registrations = await navigator.serviceWorker.getRegistrations().catch(() => [])
    await Promise.all(registrations.map((registration) => registration.update().catch(() => undefined)))
  }

  window.location.reload()
}

window.addEventListener('vite:preloadError', (event) => {
  event.preventDefault()
  void reloadAfterChunkError()
})

window.addEventListener('unhandledrejection', (event) => {
  const reason = event.reason
  const message = reason instanceof Error ? reason.message : String(reason ?? '')
  if (isChunkLoadError(message)) {
    event.preventDefault()
    void reloadAfterChunkError()
  }
})

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 2,
      refetchOnWindowFocus: false,
    },
  },
})

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <ToastProvider>
          <App />
        </ToastProvider>
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>,
)
