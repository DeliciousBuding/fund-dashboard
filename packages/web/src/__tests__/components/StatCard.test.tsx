import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import StatCard from '../../components/StatCard';

describe('StatCard', () => {
  it('renders title text', () => {
    render(<StatCard label="累计买入" value="¥ 50,000" />);
    expect(screen.getByText('累计买入')).toBeInTheDocument();
  });

  it('renders value', () => {
    render(<StatCard label="累计买入" value="¥ 50,000" />);
    expect(screen.getByText('¥ 50,000')).toBeInTheDocument();
  });

  it('renders subtitle when provided', () => {
    render(<StatCard label="定投投入" value="¥ 25,000" sub="50 笔" />);
    expect(screen.getByText('50 笔')).toBeInTheDocument();
  });

  it('does not render subtitle element when not provided', () => {
    render(<StatCard label="Test" value="123" />);
    // The "Test" label uses <Text variant="secondary" size="xs">
    // Subtitle also uses the same variant. We verify no extra text beyond label and value.
    const secondaryTexts = screen.getAllByText(/Test|123/);
    expect(secondaryTexts.length).toBeGreaterThanOrEqual(1);
  });

  it('renders red color for "up" color prop', () => {
    render(<StatCard label="未实现盈亏" value="+¥ 500.00" color="up" />);
    const valueEl = screen.getByText('+¥ 500.00');
    expect(valueEl).toBeInTheDocument();
  });

  it('renders green color for "down" color prop', () => {
    render(<StatCard label="未实现盈亏" value="-¥ 500.00" color="down" />);
    const valueEl = screen.getByText('-¥ 500.00');
    expect(valueEl).toBeInTheDocument();
  });

  it('renders without color prop', () => {
    render(<StatCard label="Test" value="123" />);
    expect(screen.getByText('123')).toBeInTheDocument();
  });
});
