import { useCallback, useEffect, useMemo, useState } from 'react';
import type { ComponentType } from 'react';
import { useLocation, useNavigate } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Drawer, Input, Modal, Tooltip } from 'antd';
import {
  ApiOutlined,
  AppstoreOutlined,
  ArrowRightOutlined,
  ClockCircleOutlined,
  CloseOutlined,
  ClusterOutlined,
  DashboardOutlined,
  ExportOutlined,
  GlobalOutlined,
  ImportOutlined,
  LogoutOutlined,
  MenuOutlined,
  MoonFilled,
  MoonOutlined,
  SearchOutlined,
  SettingOutlined,
  SunOutlined,
  SwapOutlined,
  TagsOutlined,
  TeamOutlined,
  ToolOutlined,
} from '@ant-design/icons';
import { HttpUtil } from '@/utils';
import { formatPanelVersion } from '@/lib/panel-version';
import { pauseAnimationsUntilLeave, useTheme } from '@/hooks/useTheme';
import { useServerClock } from '@/hooks/useServerClock';
import './AppSidebar.css';

const REPO_URL = 'https://github.com/0fariid0/3x-ui';
const LOGOUT_KEY = '__logout__';

type IconName =
  | 'dashboard'
  | 'inbound'
  | 'team'
  | 'groups'
  | 'setting'
  | 'tool'
  | 'cluster'
  | 'hosts'
  | 'logout'
  | 'apidocs'
  | 'outbound'
  | 'routing';

type NavGroup = 'workspace' | 'network' | 'system';

interface NavItem {
  key: string;
  icon: IconName;
  title: string;
  group: NavGroup;
  hint: string;
}

const iconByName: Record<IconName, ComponentType> = {
  dashboard: DashboardOutlined,
  inbound: ImportOutlined,
  team: TeamOutlined,
  groups: TagsOutlined,
  setting: SettingOutlined,
  tool: ToolOutlined,
  cluster: ClusterOutlined,
  hosts: GlobalOutlined,
  logout: LogoutOutlined,
  apidocs: ApiOutlined,
  outbound: ExportOutlined,
  routing: SwapOutlined,
};

function isFa(): boolean {
  if (typeof document === 'undefined') return false;
  return document.documentElement.dir === 'rtl' || (document.documentElement.lang || '').startsWith('fa');
}

function BrandMark({ compact = false }: { compact?: boolean }) {
  return (
    <div className={`fx-brand${compact ? ' is-compact' : ''}`}>
      <span className="fx-brand-symbol" aria-hidden="true">
        <i />
        <i />
        <i />
      </span>
      {!compact && (
        <span className="fx-brand-copy">
          <strong>Fara Xray</strong>
          <small>CONTROL CENTER</small>
        </span>
      )}
    </div>
  );
}

function ThemeCycleButton({
  id,
  isDark,
  isUltra,
  onCycle,
  ariaLabel,
}: {
  id: string;
  isDark: boolean;
  isUltra: boolean;
  onCycle: () => void;
  ariaLabel: string;
}) {
  const icon = !isDark ? <SunOutlined /> : !isUltra ? <MoonOutlined /> : <MoonFilled />;
  return (
    <button
      id={id}
      type="button"
      className="fx-icon-button"
      aria-label={ariaLabel}
      title={ariaLabel}
      onClick={onCycle}
    >
      {icon}
    </button>
  );
}

export default function AppSidebar() {
  const { t } = useTranslation();
  const { isDark, isUltra, toggleTheme, toggleUltra } = useTheme();
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const serverClock = useServerClock();
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [commandOpen, setCommandOpen] = useState(false);
  const [query, setQuery] = useState('');
  const panelVersion = window.X_UI_CUR_VER || '';
  const persian = isFa();

  const tabs = useMemo<NavItem[]>(
    () => [
      {
        key: '/',
        icon: 'dashboard',
        title: t('menu.dashboard'),
        group: 'workspace',
        hint: persian ? 'نمای کلی و کنترل سریع' : 'Overview and quick control',
      },
      {
        key: '/clients',
        icon: 'team',
        title: t('menu.clients'),
        group: 'workspace',
        hint: persian ? 'کاربران و اشتراک‌ها' : 'Users and subscriptions',
      },
      {
        key: '/groups',
        icon: 'groups',
        title: t('menu.groups'),
        group: 'workspace',
        hint: persian ? 'مدیریت دسترسی گروهی' : 'Grouped access control',
      },
      {
        key: '/inbounds',
        icon: 'inbound',
        title: t('menu.inbounds'),
        group: 'network',
        hint: persian ? 'ورودی‌ها و پروتکل‌ها' : 'Ingress and protocols',
      },
      {
        key: '/hosts',
        icon: 'hosts',
        title: t('menu.hosts'),
        group: 'network',
        hint: persian ? 'هاست‌های لینک اشتراک' : 'Subscription endpoints',
      },
      {
        key: '/nodes',
        icon: 'cluster',
        title: t('menu.nodes'),
        group: 'network',
        hint: persian ? 'سرورهای متصل' : 'Connected servers',
      },
      {
        key: '/outbound',
        icon: 'outbound',
        title: t('menu.outbounds'),
        group: 'network',
        hint: persian ? 'مسیرهای خروجی' : 'Egress paths',
      },
      {
        key: '/routing',
        icon: 'routing',
        title: t('menu.routing'),
        group: 'network',
        hint: persian ? 'قوانین هدایت ترافیک' : 'Traffic policies',
      },
      {
        key: '/settings',
        icon: 'setting',
        title: t('menu.settings'),
        group: 'system',
        hint: persian ? 'پنل، امنیت و اشتراک' : 'Panel and security',
      },
      {
        key: '/xray',
        icon: 'tool',
        title: t('menu.xray'),
        group: 'system',
        hint: persian ? 'موتور و تنظیمات پیشرفته' : 'Engine configuration',
      },
      {
        key: '/api-docs',
        icon: 'apidocs',
        title: t('menu.apiDocs'),
        group: 'system',
        hint: persian ? 'مستندات توسعه‌دهنده' : 'Developer reference',
      },
    ],
    [persian, t],
  );

  const current = useMemo(() => {
    if (pathname === '/') return tabs[0];
    return tabs.find((tab) => pathname === tab.key || pathname.startsWith(`${tab.key}/`)) || tabs[0];
  }, [pathname, tabs]);

  const filteredTabs = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase();
    if (!needle) return tabs;
    return tabs.filter((item) => `${item.title} ${item.hint}`.toLocaleLowerCase().includes(needle));
  }, [query, tabs]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        setCommandOpen((open) => !open);
      }
      if (event.key === 'Escape') {
        setCommandOpen(false);
        setQuery('');
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  const openLink = useCallback(
    async (key: string) => {
      if (key === LOGOUT_KEY) {
        await HttpUtil.post('/logout');
        window.location.href = window.X_UI_BASE_PATH || '/';
        return;
      }
      navigate(key);
      setDrawerOpen(false);
      setCommandOpen(false);
      setQuery('');
    },
    [navigate],
  );

  const cycleTheme = useCallback(
    (id: string) => {
      pauseAnimationsUntilLeave(id);
      if (!isDark) {
        toggleTheme();
        if (isUltra) toggleUltra();
      } else if (!isUltra) {
        toggleUltra();
      } else {
        toggleUltra();
        toggleTheme();
      }
    },
    [isDark, isUltra, toggleTheme, toggleUltra],
  );

  const groups: { key: NavGroup; label: string }[] = [
    { key: 'workspace', label: persian ? 'مدیریت' : 'Workspace' },
    { key: 'network', label: persian ? 'شبکه' : 'Network' },
    { key: 'system', label: persian ? 'سیستم' : 'System' },
  ];

  const renderNavItem = (item: NavItem, compact = false) => {
    const Icon = iconByName[item.icon];
    const active = current.key === item.key;
    return (
      <button
        key={item.key}
        type="button"
        className={`fx-nav-item${active ? ' is-active' : ''}${compact ? ' is-compact' : ''}`}
        onClick={() => openLink(item.key)}
        title={compact ? item.title : undefined}
      >
        <span className="fx-nav-icon"><Icon /></span>
        <span className="fx-nav-copy">
          <strong>{item.title}</strong>
          <small>{item.hint}</small>
        </span>
        <ArrowRightOutlined className="fx-nav-arrow" />
      </button>
    );
  };

  const mobileItems = tabs.filter((item) => ['/', '/inbounds', '/clients', '/hosts'].includes(item.key));
  const CurrentIcon = iconByName[current.icon];

  return (
    <>
      <aside className="fx-sidebar" aria-label="Primary navigation">
        <div className="fx-sidebar-top">
          <BrandMark />
          <span className="fx-online-dot" title={persian ? 'پنل فعال است' : 'Panel online'} />
        </div>

        <button type="button" className="fx-search-button" onClick={() => setCommandOpen(true)}>
          <SearchOutlined />
          <span>{persian ? 'جست‌وجوی سریع' : 'Quick search'}</span>
          <kbd>⌘ K</kbd>
        </button>

        <div className="fx-sidebar-scroll">
          {groups.map((group) => (
            <section className="fx-nav-group" key={group.key}>
              <div className="fx-nav-group-title">{group.label}</div>
              <div className="fx-nav-list">
                {tabs.filter((item) => item.group === group.key).map((item) => renderNavItem(item))}
              </div>
            </section>
          ))}
        </div>

        <div className="fx-sidebar-footer">
          <div className="fx-server-card" title={serverClock.dateTimeText}>
            <span className="fx-server-card-icon"><ClockCircleOutlined /></span>
            <span className="fx-server-card-copy">
              <strong>{serverClock.clockText}</strong>
              <small>{serverClock.data?.timezoneLabel || (persian ? 'زمان سرور' : 'Server time')}</small>
            </span>
          </div>
          <div className="fx-footer-actions">
            <ThemeCycleButton
              id="theme-cycle"
              isDark={isDark}
              isUltra={isUltra}
              onCycle={() => cycleTheme('theme-cycle')}
              ariaLabel={t('menu.theme')}
            />
            <a className="fx-version-pill" href={REPO_URL} target="_blank" rel="noopener noreferrer">
              {formatPanelVersion(panelVersion)}
            </a>
            <button
              className="fx-icon-button is-danger"
              type="button"
              title={t('logout')}
              onClick={() => openLink(LOGOUT_KEY)}
            >
              <LogoutOutlined />
            </button>
          </div>
        </div>
      </aside>

      <header className="fx-desktop-header">
        <div className="fx-header-page">
          <span className="fx-header-page-icon"><CurrentIcon /></span>
          <div>
            <small>{persian ? 'صفحه فعلی' : 'Current page'}</small>
            <strong>{current.title}</strong>
          </div>
        </div>
        <div className="fx-header-actions">
          <Tooltip title={serverClock.dateTimeText}>
            <span className="fx-header-clock"><ClockCircleOutlined />{serverClock.clockText}</span>
          </Tooltip>
          <button type="button" className="fx-header-search" onClick={() => setCommandOpen(true)}>
            <SearchOutlined />
            <span>{persian ? 'جست‌وجو' : 'Search'}</span>
          </button>
          <ThemeCycleButton
            id="theme-cycle-top"
            isDark={isDark}
            isUltra={isUltra}
            onCycle={() => cycleTheme('theme-cycle-top')}
            ariaLabel={t('menu.theme')}
          />
        </div>
      </header>

      <header className="fx-mobile-bar">
        <button className="fx-icon-button" type="button" aria-label={t('menu.openMenu')} onClick={() => setDrawerOpen(true)}>
          <MenuOutlined />
        </button>
        <div className="fx-mobile-title">
          <BrandMark compact />
          <strong>{current.title}</strong>
        </div>
        <button className="fx-icon-button" type="button" aria-label="Search" onClick={() => setCommandOpen(true)}>
          <SearchOutlined />
        </button>
      </header>

      <nav className="fx-mobile-dock" aria-label="Primary">
        {mobileItems.map((item) => {
          const Icon = iconByName[item.icon];
          const active = current.key === item.key;
          return (
            <button key={item.key} type="button" className={active ? 'is-active' : ''} onClick={() => openLink(item.key)}>
              <span><Icon /></span>
              <small>{item.title}</small>
            </button>
          );
        })}
        <button type="button" className={drawerOpen ? 'is-active' : ''} onClick={() => setDrawerOpen(true)}>
          <span><AppstoreOutlined /></span>
          <small>{t('more')}</small>
        </button>
      </nav>

      <Drawer
        placement={persian ? 'right' : 'left'}
        closable={false}
        open={drawerOpen}
        rootClassName="fx-mobile-drawer"
        width="min(92vw, 390px)"
        styles={{ body: { padding: 0 } }}
        onClose={() => setDrawerOpen(false)}
      >
        <div className="fx-drawer-head">
          <BrandMark />
          <button className="fx-icon-button" type="button" aria-label={t('close')} onClick={() => setDrawerOpen(false)}>
            <CloseOutlined />
          </button>
        </div>
        <div className="fx-drawer-current">
          <span><CurrentIcon /></span>
          <div><small>{persian ? 'صفحه فعلی' : 'Current page'}</small><strong>{current.title}</strong></div>
        </div>
        <div className="fx-drawer-nav">
          {groups.map((group) => (
            <section key={group.key}>
              <div className="fx-nav-group-title">{group.label}</div>
              <div className="fx-drawer-grid">
                {tabs.filter((item) => item.group === group.key).map((item) => renderNavItem(item, true))}
              </div>
            </section>
          ))}
        </div>
        <div className="fx-drawer-bottom">
          <div className="fx-drawer-time"><ClockCircleOutlined /><strong>{serverClock.clockText}</strong><span>{serverClock.data?.timezoneLabel || ''}</span></div>
          <div className="fx-drawer-actions">
            <ThemeCycleButton
              id="theme-cycle-mobile"
              isDark={isDark}
              isUltra={isUltra}
              onCycle={() => cycleTheme('theme-cycle-mobile')}
              ariaLabel={t('menu.theme')}
            />
            <a className="fx-version-pill" href={REPO_URL} target="_blank" rel="noopener noreferrer">
              {formatPanelVersion(panelVersion)}
            </a>
            <button className="fx-drawer-logout" type="button" onClick={() => openLink(LOGOUT_KEY)}>
              <LogoutOutlined />{t('logout')}
            </button>
          </div>
        </div>
      </Drawer>

      <Modal
        open={commandOpen}
        footer={null}
        closable={false}
        width={620}
        rootClassName="fx-command-modal"
        onCancel={() => { setCommandOpen(false); setQuery(''); }}
      >
        <div className="fx-command-head">
          <SearchOutlined />
          <Input
            autoFocus
            variant="borderless"
            value={query}
            placeholder={persian ? 'نام بخش یا ابزار را بنویسید…' : 'Type a section or tool…'}
            onChange={(event) => setQuery(event.target.value)}
            onPressEnter={() => filteredTabs[0] && openLink(filteredTabs[0].key)}
          />
          <button type="button" onClick={() => { setCommandOpen(false); setQuery(''); }}>ESC</button>
        </div>
        <div className="fx-command-results">
          {filteredTabs.map((item) => {
            const Icon = iconByName[item.icon];
            return (
              <button key={item.key} type="button" onClick={() => openLink(item.key)}>
                <span className="fx-command-icon"><Icon /></span>
                <span><strong>{item.title}</strong><small>{item.hint}</small></span>
                <ArrowRightOutlined />
              </button>
            );
          })}
          {filteredTabs.length === 0 && (
            <div className="fx-command-empty">{persian ? 'نتیجه‌ای پیدا نشد' : 'No matching section'}</div>
          )}
        </div>
      </Modal>
    </>
  );
}
