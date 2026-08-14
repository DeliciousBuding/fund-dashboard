import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import DataFreshnessBar from '../../components/DataFreshnessBar'

describe('DataFreshnessBar', () => {
  it('shows pending label when no NAV date', () => {
    render(<DataFreshnessBar onRefresh={() => {}} />)
    expect(screen.getByText('净值待更新')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '刷新数据' })).toBeInTheDocument()
    const bar = document.querySelector('.fd-freshness-bar')
    expect(bar?.getAttribute('data-stale-level')).toBe('unknown')
  })

  it('marks fresh NAV as ok with today label', () => {
    render(
      <DataFreshnessBar
        lastNavDate="2026-07-19"
        staleDays={0}
        onRefresh={() => {}}
      />,
    )
    expect(screen.getByText(/净值更新于/)).toBeInTheDocument()
    expect(screen.getByText(/今日/)).toBeInTheDocument()
    expect(document.querySelector('.fd-freshness-bar')?.getAttribute('data-stale-level')).toBe('ok')
  })

  it('warns when stale 3–7 days', () => {
    render(
      <DataFreshnessBar
        lastNavDate="2026-07-14"
        staleDays={5}
        onRefresh={() => {}}
      />,
    )
    expect(screen.getByText(/5天前/)).toBeInTheDocument()
    expect(document.querySelector('.fd-freshness-bar')?.getAttribute('data-stale-level')).toBe('warn')
  })

  it('is critical when stale > 7 days', () => {
    render(
      <DataFreshnessBar
        lastNavDate="2026-07-01"
        staleDays={18}
        onRefresh={() => {}}
      />,
    )
    expect(document.querySelector('.fd-freshness-bar')?.getAttribute('data-stale-level')).toBe('critical')
  })

  it('disables refresh while fetching and shows busy state', () => {
    render(
      <DataFreshnessBar
        lastNavDate="2026-07-19"
        staleDays={0}
        isFetching
        onRefresh={() => {}}
      />,
    )
    const btn = screen.getByRole('button', { name: '刷新数据' })
    expect(btn).toBeDisabled()
    expect(btn).toHaveAttribute('aria-busy', 'true')
    expect(screen.getByText('刷新中…')).toBeInTheDocument()
  })

  it('invokes onRefresh when button clicked', async () => {
    const user = userEvent.setup()
    const onRefresh = vi.fn()
    render(
      <DataFreshnessBar
        lastNavDate="2026-07-19"
        staleDays={1}
        onRefresh={onRefresh}
      />,
    )
    await user.click(screen.getByRole('button', { name: '刷新数据' }))
    expect(onRefresh).toHaveBeenCalledTimes(1)
  })
})
