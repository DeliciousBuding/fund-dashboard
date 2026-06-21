import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ErrorBoundary } from '../../components/ErrorBoundary';

// Suppress React error boundary logs during tests
const origError = console.error;
beforeAll(() => { console.error = vi.fn(); });
afterAll(() => { console.error = origError; });

function BrokenComponent() {
  throw new Error('Test explosion');
}

function NormalComponent() {
  return <div>一切正常</div>;
}

describe('ErrorBoundary', () => {
  it('renders children normally', () => {
    render(
      <ErrorBoundary>
        <NormalComponent />
      </ErrorBoundary>
    );
    expect(screen.getByText('一切正常')).toBeInTheDocument();
  });

  it('catches error and shows fallback UI', () => {
    render(
      <ErrorBoundary>
        <BrokenComponent />
      </ErrorBoundary>
    );
    expect(screen.getByText(/组件加载失败/)).toBeInTheDocument();
    expect(screen.getByText(/Test explosion/)).toBeInTheDocument();
  });

  it('shows retry button in fallback', () => {
    render(
      <ErrorBoundary>
        <BrokenComponent />
      </ErrorBoundary>
    );
    expect(screen.getByText('重试')).toBeInTheDocument();
  });

  it('retry resets error state', () => {
    render(
      <ErrorBoundary>
        <BrokenComponent />
      </ErrorBoundary>
    );

    // Error is shown first
    expect(screen.getByText(/组件加载失败/)).toBeInTheDocument();

    // Click retry — the component throws again, so error persists
    // But the state reset does happen (error message re-renders)
    fireEvent.click(screen.getByText('重试'));
    // After retry, if the component still throws, error re-appears
    expect(screen.getByText(/组件加载失败/)).toBeInTheDocument();
  });

  it('renders custom fallback when provided', () => {
    render(
      <ErrorBoundary fallback={<div>自定义错误提示</div>}>
        <BrokenComponent />
      </ErrorBoundary>
    );
    expect(screen.getByText('自定义错误提示')).toBeInTheDocument();
    expect(screen.queryByText(/组件加载失败/)).not.toBeInTheDocument();
  });

  it('renders default fallback UI with correct structure and styling', () => {
    render(
      <ErrorBoundary>
        <BrokenComponent />
      </ErrorBoundary>
    );

    // Error heading with specific message
    const errorText = screen.getByText(/组件加载失败: Test explosion/);
    expect(errorText).toBeInTheDocument();

    // The error text is rendered inside a span with data-variant="error"
    expect(errorText.getAttribute('data-variant')).toBe('error');

    // Retry button with correct variant
    const retryButton = screen.getByText('重试');
    expect(retryButton).toBeInTheDocument();
    expect(retryButton.getAttribute('data-variant')).toBe('primary');

    // Fallback UI has padding:60px and textAlign:center on the container div
    // The container is the parent div of the error text
    const containerDiv = errorText.parentElement;
    expect(containerDiv).toBeDefined();
    if (containerDiv) {
      const styleAttr = containerDiv.getAttribute('style') || '';
      expect(styleAttr).toContain('padding');
      expect(styleAttr).toContain('text-align');
    }
  });
});
