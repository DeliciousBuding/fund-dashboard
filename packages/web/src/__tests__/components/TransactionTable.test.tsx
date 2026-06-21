import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import TransactionTable from '../../components/TransactionTable'
import type { Transaction } from '../../api'

// ── Mock transaction data ──────────────────────────────────────

const mockTransactions: Transaction[] = [
  {
    seq: 1,
    trade_time: '2024-01-03T09:30:00',
    confirm_date: '2024-01-04',
    trade_type: '用户买入',
    direction: 'buy',
    amount: 1000,
    shares: 990.10,
    fee: 1.50,
    nav: 1.0100,
    inferred_nav: null,
    settlement_days: 1,
    order_id: 'ord_001',
    anomaly: null,
  },
  {
    seq: 2,
    trade_time: '2024-01-10T10:00:00',
    confirm_date: '2024-01-11',
    trade_type: '定投买入',
    direction: 'buy',
    amount: 2000,
    shares: 1923.08,
    fee: 1.50,
    nav: 1.0400,
    inferred_nav: null,
    settlement_days: 1,
    order_id: 'ord_002',
    anomaly: null,
  },
  {
    seq: 3,
    trade_time: '2024-01-20T14:00:00',
    confirm_date: '2024-01-22',
    trade_type: '用户卖出',
    direction: 'sell',
    amount: -500,
    shares: -462.96,
    fee: 0.75,
    nav: 1.0800,
    inferred_nav: null,
    settlement_days: 1,
    order_id: 'ord_003',
    anomaly: null,
  },
  {
    seq: 4,
    trade_time: '2024-03-01T00:00:00',
    confirm_date: '2024-03-02',
    trade_type: '分红',
    direction: 'dividend',
    amount: 150,
    shares: 0,
    fee: 0,
    nav: null,
    inferred_nav: null,
    settlement_days: 0,
    order_id: 'div_001',
    anomaly: null,
  },
]

const noop = vi.fn()

describe('TransactionTable', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders buy/sell/dividend rows with correct badges', () => {
    render(
      <TransactionTable
        transactions={mockTransactions}
        onToggleType={noop}
        onDelete={noop}
        onAdd={noop}
        deleting={null}
      />,
    )

    // Direction badges - there are 2 buy rows, so use getAllByText
    expect(screen.getAllByText('买入')).toHaveLength(2)
    expect(screen.getByText('卖出')).toBeInTheDocument()
    expect(screen.getByText('分红')).toBeInTheDocument()

    // Auto-buy badge (only one 定投 transaction)
    expect(screen.getByText('定投')).toBeInTheDocument()
    // Manual badge appears on both manual buy and manual sell (2 occurrences)
    expect(screen.getAllByText('手动')).toHaveLength(2)

    // Amounts rendered
    expect(screen.getByText('¥ 1000.00')).toBeInTheDocument()
    expect(screen.getByText('¥ 2000.00')).toBeInTheDocument()
    expect(screen.getByText('¥ 150.00')).toBeInTheDocument()

    // Shares rendered
    expect(screen.getByText('990.10')).toBeInTheDocument()
    expect(screen.getByText('1923.08')).toBeInTheDocument()
  })

  it('renders search input and filters transactions', async () => {
    render(
      <TransactionTable
        transactions={mockTransactions}
        onToggleType={noop}
        onDelete={noop}
        onAdd={noop}
        deleting={null}
      />,
    )

    const searchInput = screen.getByPlaceholderText('搜索交易记录（日期/类型/金额）...')
    expect(searchInput).toBeInTheDocument()

    // Type "分红" to filter to only the dividend transaction
    await userEvent.type(searchInput, '分红')

    // Only the dividend row should remain
    expect(screen.getByText('分红')).toBeInTheDocument()
    // Buy/sell should be filtered out
    expect(screen.queryByText('买入')).not.toBeInTheDocument()
    expect(screen.queryByText('卖出')).not.toBeInTheDocument()
  })

  it('supports column sort toggle by clicking headers', async () => {
    render(
      <TransactionTable
        transactions={mockTransactions}
        onToggleType={noop}
        onDelete={noop}
        onAdd={noop}
        deleting={null}
      />,
    )

    const amountHeader = screen.getByText('金额')
    expect(amountHeader).toBeInTheDocument()

    // Click "金额" header to sort by amount (ascending first? actually default desc toggle)
    fireEvent.click(amountHeader)

    // The sort indicator should appear
    expect(amountHeader.textContent).toContain('▼')

    // Click again to reverse sort
    fireEvent.click(amountHeader)
    expect(amountHeader.textContent).toContain('▲')
  })

  it('shows empty state when no transactions match search', async () => {
    render(
      <TransactionTable
        transactions={mockTransactions}
        onToggleType={noop}
        onDelete={noop}
        onAdd={noop}
        deleting={null}
      />,
    )

    const searchInput = screen.getByPlaceholderText('搜索交易记录（日期/类型/金额）...')

    // Type something that matches nothing
    await userEvent.type(searchInput, 'ZZZZZ_NOMATCH')

    expect(screen.getByText('无匹配交易')).toBeInTheDocument()
  })

  it('renders empty state with no transactions', () => {
    render(
      <TransactionTable
        transactions={[]}
        onToggleType={noop}
        onDelete={noop}
        onAdd={noop}
        deleting={null}
      />,
    )

    expect(screen.getByText('无匹配交易')).toBeInTheDocument()
  })

  it('calls onAdd when "添加交易" button is clicked', async () => {
    const onAdd = vi.fn()
    render(
      <TransactionTable
        transactions={mockTransactions}
        onToggleType={noop}
        onDelete={noop}
        onAdd={onAdd}
        deleting={null}
      />,
    )

    const addButton = screen.getByText('添加交易')
    fireEvent.click(addButton)
    expect(onAdd).toHaveBeenCalledTimes(1)
  })

  it('calls onDelete when delete button is clicked', async () => {
    const onDelete = vi.fn()
    render(
      <TransactionTable
        transactions={[mockTransactions[0]]}
        onToggleType={noop}
        onDelete={onDelete}
        onAdd={noop}
        deleting={null}
      />,
    )

    // Find delete button (TrashIcon with title "删除此交易")
    const deleteButton = screen.getByTitle('删除此交易')
    fireEvent.click(deleteButton)
    expect(onDelete).toHaveBeenCalledWith(1)
  })

  it('shows reduced opacity for the deleting row', () => {
    render(
      <TransactionTable
        transactions={mockTransactions}
        onToggleType={noop}
        onDelete={noop}
        onAdd={noop}
        deleting={1}
      />,
    )

    // Row with seq=1 should have reduced opacity
    const rows = document.querySelectorAll('tr')
    const deletingRow = Array.from(rows).find(r => r.style.opacity === '0.4')
    expect(deletingRow).toBeDefined()
  })

  it('renders settlement days as T+N format', () => {
    render(
      <TransactionTable
        transactions={[mockTransactions[0]]}
        onToggleType={noop}
        onDelete={noop}
        onAdd={noop}
        deleting={null}
      />,
    )

    expect(screen.getByText('T+1')).toBeInTheDocument()
  })
})
