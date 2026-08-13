import { createContext, useCallback, useContext, useLayoutEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { theme as antdTheme } from 'antd';
import type { ThemeConfig } from 'antd';

const STORAGE_DARK = 'dark-mode';
const STORAGE_ULTRA = 'isUltraDarkThemeEnabled';

function readBool(key: string, fallback: boolean): boolean {
  const raw = localStorage.getItem(key);
  if (raw === null) return fallback;
  return raw === 'true';
}

function applyDom(isDark: boolean, isUltra: boolean) {
  document.body.classList.remove('dark', 'light');
  document.body.classList.add(isDark ? 'dark' : 'light');
  if (isUltra) {
    document.documentElement.setAttribute('data-theme', 'ultra-dark');
  } else {
    document.documentElement.removeAttribute('data-theme');
  }
  const msg = document.getElementById('message');
  if (msg) {
    msg.classList.remove('dark', 'light');
    msg.classList.add(isDark ? 'dark' : 'light');
  }
}

// module load so the document is in the right theme before React mounts.
const initialDark = readBool(STORAGE_DARK, true);
const initialUltra = readBool(STORAGE_ULTRA, false);
applyDom(initialDark, initialUltra);

const DARK_TOKENS = {
  colorPrimary: '#20d9ff',
  colorInfo: '#20d9ff',
  colorSuccess: '#20f6a9',
  colorWarning: '#f6c85f',
  colorError: '#ff5c7a',
  colorBgBase: '#020813',
  colorBgLayout: '#020813',
  colorBgContainer: 'rgba(9, 24, 43, 0.86)',
  colorBgElevated: 'rgba(12, 31, 55, 0.98)',
  colorBorder: 'rgba(87, 211, 255, 0.18)',
  colorBorderSecondary: 'rgba(87, 211, 255, 0.12)',
  colorText: 'rgba(240, 249, 255, 0.96)',
  colorTextSecondary: 'rgba(190, 220, 235, 0.72)',
  colorTextTertiary: 'rgba(148, 187, 206, 0.56)',
};
const ULTRA_DARK_TOKENS = {
  colorPrimary: '#20d9ff',
  colorInfo: '#20d9ff',
  colorSuccess: '#20f6a9',
  colorWarning: '#f6c85f',
  colorError: '#ff5c7a',
  colorBgBase: '#01040a',
  colorBgLayout: '#01040a',
  colorBgContainer: 'rgba(5, 16, 30, 0.9)',
  colorBgElevated: 'rgba(8, 21, 38, 0.98)',
  colorBorder: 'rgba(87, 211, 255, 0.18)',
  colorBorderSecondary: 'rgba(87, 211, 255, 0.10)',
  colorText: 'rgba(240, 249, 255, 0.96)',
  colorTextSecondary: 'rgba(190, 220, 235, 0.72)',
  colorTextTertiary: 'rgba(148, 187, 206, 0.56)',
};
const DARK_LAYOUT_TOKENS = {
  bodyBg: '#020813',
  headerBg: 'rgba(3, 11, 22, 0.86)',
  headerColor: '#ffffff',
  footerBg: '#020813',
  siderBg: 'rgba(3, 13, 26, 0.92)',
  triggerBg: 'rgba(11, 30, 52, 0.98)',
  triggerColor: '#ffffff',
};
const ULTRA_DARK_LAYOUT_TOKENS = {
  bodyBg: '#01040a',
  headerBg: 'rgba(1, 7, 14, 0.92)',
  headerColor: '#ffffff',
  footerBg: '#01040a',
  siderBg: 'rgba(1, 8, 17, 0.94)',
  triggerBg: 'rgba(8, 21, 38, 0.98)',
  triggerColor: '#ffffff',
};
const DARK_MENU_TOKENS = {
  darkItemBg: 'transparent',
  darkSubMenuItemBg: 'rgba(7, 22, 40, 0.5)',
  darkPopupBg: 'rgba(12, 31, 55, 0.98)',
  darkItemSelectedBg: 'rgba(32, 217, 255, 0.22)',
  darkItemSelectedColor: '#ffffff',
};
const ULTRA_DARK_MENU_TOKENS = {
  darkItemBg: 'transparent',
  darkSubMenuItemBg: 'rgba(4, 14, 28, 0.68)',
  darkPopupBg: 'rgba(8, 21, 38, 0.98)',
  darkItemSelectedBg: 'rgba(32, 217, 255, 0.22)',
  darkItemSelectedColor: '#ffffff',
};
const DARK_CARD_TOKENS = {
  colorBorderSecondary: 'rgba(87, 211, 255, 0.14)',
};
const ULTRA_DARK_CARD_TOKENS = {
  colorBorderSecondary: 'rgba(87, 211, 255, 0.12)',
};
const STATISTIC_TOKENS = {
  contentFontSize: 17,
  titleFontSize: 11,
};
const LIGHT_CONTRAST_TOKENS = {
  colorTextDescription: 'rgba(0, 0, 0, 0.58)',
  colorTextTertiary: 'rgba(0, 0, 0, 0.58)',
  colorTextPlaceholder: '#767676',
  colorError: '#cf1322',
  colorErrorText: '#cf1322',
  colorSuccessText: '#237804',
};
const LIGHT_BUTTON_TOKENS = {
  colorPrimary: '#0958d9',
  colorPrimaryHover: '#2468e5',
  colorPrimaryActive: '#073ea8',
};

// hashed:false drops the `:where(.css-<hash>)` wrapper antd puts around every
// rule. It costs nothing in specificity — `:where()` contributes zero, so the
// panel's own `.ant-*` overrides still win — and it removes roughly 5,700
// wrappers, 16% of the generated stylesheet, from what the browser has to parse.
//
// cssVar.key pins the CSS-variable scope. Every panel page mounts its own
// ConfigProvider (there is no root one), and without a fixed key each mints a
// fresh useId-derived scope, so navigating re-serialises and re-injects the whole
// token block under a new class instead of reusing the one already in the head.
const SHARED_STYLE_CONFIG = {
  hashed: false,
  cssVar: { key: 'xui' },
} as const;

export function buildAntdThemeConfig(isDark: boolean, isUltra: boolean): ThemeConfig {
  if (!isDark) {
    return {
      ...SHARED_STYLE_CONFIG,
      algorithm: antdTheme.defaultAlgorithm,
      token: LIGHT_CONTRAST_TOKENS,
      components: {
        Statistic: STATISTIC_TOKENS,
        Button: LIGHT_BUTTON_TOKENS,
      },
    };
  }
  return {
    ...SHARED_STYLE_CONFIG,
    algorithm: antdTheme.darkAlgorithm,
    token: isUltra ? ULTRA_DARK_TOKENS : DARK_TOKENS,
    components: {
      Layout: isUltra ? ULTRA_DARK_LAYOUT_TOKENS : DARK_LAYOUT_TOKENS,
      Menu: isUltra ? ULTRA_DARK_MENU_TOKENS : DARK_MENU_TOKENS,
      Card: isUltra ? ULTRA_DARK_CARD_TOKENS : DARK_CARD_TOKENS,
      Statistic: STATISTIC_TOKENS,
    },
  };
}

export function pauseAnimationsUntilLeave(elementId: string): void {
  document.documentElement.setAttribute('data-theme-animations', 'off');
  const el = document.getElementById(elementId);
  if (!el) return;
  const restore = () => {
    document.documentElement.removeAttribute('data-theme-animations');
    el.removeEventListener('mouseleave', restore);
    el.removeEventListener('touchend', restore);
  };
  el.addEventListener('mouseleave', restore);
  el.addEventListener('touchend', restore);
}

interface ThemeContextValue {
  isDark: boolean;
  isUltra: boolean;
  toggleTheme: () => void;
  toggleUltra: () => void;
  antdThemeConfig: ThemeConfig;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [isDark, setIsDark] = useState<boolean>(initialDark);
  const [isUltra, setIsUltra] = useState<boolean>(initialUltra);

  useLayoutEffect(() => {
    applyDom(isDark, isUltra);
    localStorage.setItem(STORAGE_DARK, String(isDark));
    localStorage.setItem(STORAGE_ULTRA, String(isUltra));
  }, [isDark, isUltra]);

  const toggleTheme = useCallback(() => setIsDark((v) => !v), []);
  const toggleUltra = useCallback(() => setIsUltra((v) => !v), []);

  const antdThemeConfig = useMemo(() => buildAntdThemeConfig(isDark, isUltra), [isDark, isUltra]);

  const value = useMemo<ThemeContextValue>(
    () => ({ isDark, isUltra, toggleTheme, toggleUltra, antdThemeConfig }),
    [isDark, isUltra, toggleTheme, toggleUltra, antdThemeConfig],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error('useTheme must be used inside <ThemeProvider>');
  return ctx;
}
