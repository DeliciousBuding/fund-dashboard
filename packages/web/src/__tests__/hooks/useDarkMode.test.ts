import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useDarkMode } from '../../hooks/useDarkMode';

describe('useDarkMode', () => {
  beforeEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute('data-mode');
  });

  afterEach(() => {
    localStorage.clear();
    document.documentElement.removeAttribute('data-mode');
  });

  it('defaults to light mode when no localStorage value is set and system prefers light', () => {
    // Mock matchMedia to return light preference
    window.matchMedia = vi.fn().mockImplementation((query) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));

    const { result } = renderHook(() => useDarkMode());
    expect(result.current.dark).toBe(false);
  });

  it('reads dark mode from localStorage when stored as "true"', () => {
    localStorage.setItem('fund-dark-mode', 'true');
    const { result } = renderHook(() => useDarkMode());
    expect(result.current.dark).toBe(true);
  });

  it('reads light mode from localStorage when stored as "false"', () => {
    localStorage.setItem('fund-dark-mode', 'false');
    const { result } = renderHook(() => useDarkMode());
    expect(result.current.dark).toBe(false);
  });

  it('toggle switches from light to dark', () => {
    localStorage.setItem('fund-dark-mode', 'false');
    const { result } = renderHook(() => useDarkMode());

    act(() => {
      result.current.toggle();
    });

    expect(result.current.dark).toBe(true);
    expect(localStorage.getItem('fund-dark-mode')).toBe('true');
  });

  it('toggle switches from dark to light', () => {
    localStorage.setItem('fund-dark-mode', 'true');
    const { result } = renderHook(() => useDarkMode());

    act(() => {
      result.current.toggle();
    });

    expect(result.current.dark).toBe(false);
    expect(localStorage.getItem('fund-dark-mode')).toBe('false');
  });

  it('sets data-mode attribute on document element', () => {
    const { result } = renderHook(() => useDarkMode());

    act(() => {
      result.current.toggle();
    });

    expect(document.documentElement.getAttribute('data-mode')).toBe('dark');

    act(() => {
      result.current.toggle();
    });

    expect(document.documentElement.getAttribute('data-mode')).toBe('light');
  });

  it('defaults to system preference (dark) when no localStorage', () => {
    window.matchMedia = vi.fn().mockImplementation((query) => ({
      matches: true,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));

    const { result } = renderHook(() => useDarkMode());
    expect(result.current.dark).toBe(true);
  });
});
