import { useEffect, useId, useRef } from 'react'
import { Button, Text } from '@cloudflare/kumo'
import { getTheme, space, radius, fontSize, fontWeight, zIndex, glassSurfaceStyle } from '../styles/theme'
import { useAppStore } from '../stores/appStore'

interface ConfirmDialogProps {
  open: boolean
  title: string
  message: string
  confirmLabel: string
  cancelLabel: string
  destructive?: boolean
  onConfirm: () => void
  onCancel: () => void
}

/** Lightweight in-app confirm (replaces window.confirm for glass UI consistency) (#190/#200). */
export default function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel,
  cancelLabel,
  destructive = false,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const dark = useAppStore((s) => s.dark)
  const theme = getTheme(dark)
  const titleId = useId()
  const descId = useId()
  const panelRef = useRef<HTMLDivElement>(null)
  const prevFocusRef = useRef<HTMLElement | null>(null)

  useEffect(() => {
    if (!open) return
    prevFocusRef.current = document.activeElement as HTMLElement | null
    const panel = panelRef.current
    const focusable = () =>
      panel
        ? Array.from(
            panel.querySelectorAll<HTMLElement>(
              'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
            ),
          )
        : []

    // Prefer cancel for destructive, else last primary — safer default for deletes.
    const t = window.setTimeout(() => {
      const nodes = focusable()
      if (!nodes.length) return
      const target = destructive ? nodes[0] : nodes[nodes.length - 1]
      target.focus()
    }, 0)

    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        onCancel()
        return
      }
      if (e.key !== 'Tab' || !panel) return
      const nodes = focusable()
      if (!nodes.length) return
      const first = nodes[0]
      const last = nodes[nodes.length - 1]
      const active = document.activeElement as HTMLElement | null
      if (e.shiftKey) {
        if (active === first || !panel.contains(active)) {
          e.preventDefault()
          last.focus()
        }
      } else if (active === last || !panel.contains(active)) {
        e.preventDefault()
        first.focus()
      }
    }
    document.addEventListener('keydown', onKeyDown)
    return () => {
      window.clearTimeout(t)
      document.removeEventListener('keydown', onKeyDown)
      prevFocusRef.current?.focus?.()
    }
  }, [open, destructive, onCancel])

  if (!open) return null
  return (
    <div
      role="presentation"
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: zIndex.modal + 10,
        background: 'rgba(0,0,0,0.45)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: space[4],
      }}
      onClick={onCancel}
    >
      <div
        ref={panelRef}
        role="alertdialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={descId}
        style={{
          ...glassSurfaceStyle(theme, { borderRadius: radius.lg }),
          maxWidth: 400,
          width: '100%',
          padding: space[5],
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <Text id={titleId} variant="heading3" as="h2" style={{ marginBottom: space[2] }}>
          {title}
        </Text>
        <Text id={descId} variant="body" as="p" style={{ marginBottom: space[5], fontSize: fontSize.base }}>
          {message}
        </Text>
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: space[2] }}>
          <Button type="button" variant="secondary" size="sm" onClick={onCancel}>
            {cancelLabel}
          </Button>
          <Button
            type="button"
            variant="primary"
            size="sm"
            onClick={onConfirm}
            style={
              destructive
                ? { background: theme.critical, borderColor: theme.critical, fontWeight: fontWeight.semibold }
                : undefined
            }
          >
            {confirmLabel}
          </Button>
        </div>
      </div>
    </div>
  )
}
