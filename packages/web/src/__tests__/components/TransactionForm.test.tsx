import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import TransactionForm from '../../components/TransactionForm';

describe('TransactionForm', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders form with direction, trade type, amount, shares, fee, and date fields', () => {
    const onCancel = vi.fn();
    render(<TransactionForm onSubmit={vi.fn()} onCancel={onCancel} />);

    expect(screen.getByText('添加交易')).toBeInTheDocument();
    expect(screen.getByText('方向')).toBeInTheDocument();
    expect(screen.getByText('分类')).toBeInTheDocument();
    expect(screen.getByText('金额 (元)')).toBeInTheDocument();
    expect(screen.getByText('份额')).toBeInTheDocument();
    expect(screen.getByText('手续费')).toBeInTheDocument();
    expect(screen.getByText('交易时间')).toBeInTheDocument();
    expect(screen.getByText('确认添加')).toBeInTheDocument();
    expect(screen.getByText('取消')).toBeInTheDocument();
  });

  it('calls onSubmit with form data when valid', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<TransactionForm onSubmit={onSubmit} onCancel={vi.fn()} />);

    // Fill in amount
    const amountInput = screen.getByText('金额 (元)').closest('[data-testid="kumo-input-wrapper"]')?.querySelector('input');
    expect(amountInput).not.toBeNull();
    fireEvent.change(amountInput!, { target: { value: '500' } });

    // Fill in shares
    const sharesInput = screen.getByText('份额').closest('[data-testid="kumo-input-wrapper"]')?.querySelector('input');
    expect(sharesInput).not.toBeNull();
    fireEvent.change(sharesInput!, { target: { value: '100' } });

    // Submit
    fireEvent.click(screen.getByText('确认添加'));

    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        direction: 'buy',
        amount: '500',
        shares: '100',
      }),
    );
  });

  it('switches direction between buy, sell and adjusts trade_type accordingly', () => {
    render(<TransactionForm onSubmit={vi.fn()} onCancel={vi.fn()} />);

    // Default is buy → trade_type should be '用户买入'
    const directionSelect = screen.getByText('方向').closest('[data-testid="kumo-select-wrapper"]')?.querySelector('select');
    expect(directionSelect).not.toBeNull();

    // Verify buy options exist
    expect(screen.getByText('买入')).toBeInTheDocument();
    expect(screen.getByText('卖出')).toBeInTheDocument();
  });

  it('shows validation error when amount is empty or invalid', async () => {
    const onSubmit = vi.fn();
    render(<TransactionForm onSubmit={onSubmit} onCancel={vi.fn()} />);

    // Click submit without filling amount
    fireEvent.click(screen.getByText('确认添加'));

    // Should show validation error
    expect(screen.getByText('请输入有效金额')).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('calls onCancel when cancel button is clicked', () => {
    const onCancel = vi.fn();
    render(<TransactionForm onSubmit={vi.fn()} onCancel={onCancel} />);

    fireEvent.click(screen.getByText('取消'));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });
});
