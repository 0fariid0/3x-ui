import { fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';

import { buildAntdThemeConfig, ThemeProvider, useTheme } from '@/hooks/useTheme';

function AppearanceSwitch() {
  const { appearance, toggleAppearance } = useTheme();
  return <button type="button" onClick={toggleAppearance}>{appearance}</button>;
}

beforeEach(() => {
  localStorage.removeItem('panel-appearance');
  document.documentElement.removeAttribute('data-appearance');
});

describe('panel appearance switch', () => {
  it('switches between classic and GoGuard without a reload', () => {
    render(<ThemeProvider><AppearanceSwitch /></ThemeProvider>);

    expect(screen.getByRole('button').textContent).toBe('classic');
    fireEvent.click(screen.getByRole('button'));

    expect(screen.getByRole('button').textContent).toBe('goguard');
    expect(document.documentElement.getAttribute('data-appearance')).toBe('goguard');
    expect(localStorage.getItem('panel-appearance')).toBe('goguard');

    fireEvent.click(screen.getByRole('button'));
    expect(screen.getByRole('button').textContent).toBe('classic');
    expect(document.documentElement.hasAttribute('data-appearance')).toBe(false);
  });

  it('uses separate Ant Design tokens for GoGuard', () => {
    const classic = buildAntdThemeConfig(false, false, 'classic');
    const goguard = buildAntdThemeConfig(false, false, 'goguard');

    expect(goguard.token?.colorPrimary).toBe('#2563eb');
    expect(goguard.token).not.toEqual(classic.token);
  });
});
