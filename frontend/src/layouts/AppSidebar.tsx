import { useCallback, useMemo, useState } from 'react';
import type { ComponentType } from 'react';
import { useLocation, useNavigate } from 'react-router';
import { useTranslation } from 'react-i18next';
import { Drawer, Layout, Menu } from 'antd';
import type { MenuProps } from 'antd';
import {
  ApiOutlined,
  AppstoreOutlined,
  CloseOutlined,
  CloudServerOutlined,
  ClockCircleOutlined,
  ClusterOutlined,
  CodeOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  ExportOutlined,
  GithubOutlined,
  GlobalOutlined,
  ImportOutlined,
  LogoutOutlined,
  MailOutlined,
  MenuOutlined,
  MessageOutlined,
  MoonFilled,
  MoonOutlined,
  SafetyOutlined,
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
import { useAllSettings } from '@/api/queries/useAllSettings';
import { useServerClock } from '@/hooks/useServerClock';
import './AppSidebar.css';

const REPO_URL = 'https://github.com/0fariid0/3x-ui';
const LOGOUT_KEY = '__logout__';

type IconName = 'dashboard' | 'inbound' | 'team' | 'groups' | 'setting' | 'tool' | 'cluster' | 'hosts' | 'logout' | 'apidocs' | 'outbound' | 'routing';

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

function BrandMark({ compact = false }: { compact?: boolean }) {
  return (
    <div className={`fara-brand ${compact ? 'is-compact' : ''}`}>
      <span className="fara-brand-mark">F</span>
      {!compact && (
        <span className="fara-brand-copy">
          <strong>Fara Xray</strong>
          <small>فرا ایکس‌ری</small>
        </span>
      )}
    </div>
  );
}

function VersionBadge({ version }: { version: string }) {
  if (!version) return null;
  const label = formatPanelVersion(version);
  return (
    <a href={REPO_URL} target="_blank" rel="noopener noreferrer" className="sider-version" title={label}>
      <GithubOutlined />
      <span>{label}</span>
    </a>
  );
}

function ThemeCycleButton({ id, isDark, isUltra, onCycle, ariaLabel }: {
  id: string;
  isDark: boolean;
  isUltra: boolean;
  onCycle: () => void;
  ariaLabel: string;
}) {
  const icon = !isDark ? <SunOutlined /> : !isUltra ? <MoonOutlined /> : <MoonFilled />;
  return (
    <button id={id} type="button" className="fara-icon-btn" aria-label={ariaLabel} title={ariaLabel} onClick={onCycle}>
      {icon}
    </button>
  );
}

export default function AppSidebar() {
  const { t } = useTranslation();
  const { isDark, isUltra, toggleTheme, toggleUltra } = useTheme();
  const navigate = useNavigate();
  const { pathname, hash } = useLocation();
  const { allSetting } = useAllSettings();
  const serverClock = useServerClock();
  const showSubFormats = !!(allSetting.subJsonEnable || allSetting.subClashEnable);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [openKeys, setOpenKeys] = useState<string[]>([]);

  const currentTheme: 'light' | 'dark' = isDark ? 'dark' : 'light';
  const panelVersion = window.X_UI_CUR_VER || '';

  const tabs = useMemo<{ key: string; icon: IconName; title: string }[]>(() => [
    { key: '/', icon: 'dashboard', title: t('menu.dashboard') },
    { key: '/inbounds', icon: 'inbound', title: t('menu.inbounds') },
    { key: '/clients', icon: 'team', title: t('menu.clients') },
    { key: '/groups', icon: 'groups', title: t('menu.groups') },
    { key: '/nodes', icon: 'cluster', title: t('menu.nodes') },
    { key: '/hosts', icon: 'hosts', title: t('menu.hosts') },
    { key: '/outbound', icon: 'outbound', title: t('menu.outbounds') },
    { key: '/routing', icon: 'routing', title: t('menu.routing') },
    { key: '/settings', icon: 'setting', title: t('menu.settings') },
    { key: '/xray', icon: 'tool', title: t('menu.xray') },
    { key: '/api-docs', icon: 'apidocs', title: t('menu.apiDocs') },
    { key: LOGOUT_KEY, icon: 'logout', title: t('logout') },
  ], [t]);

  const settingsChildren = useMemo<NonNullable<MenuProps['items']>>(() => {
    const children: NonNullable<MenuProps['items']> = [
      { key: '/settings#general', icon: <SettingOutlined />, label: t('pages.settings.panelSettings') },
      { key: '/settings#security', icon: <SafetyOutlined />, label: t('pages.settings.securitySettings') },
      { key: '/settings#telegram', icon: <MessageOutlined />, label: t('pages.settings.TGBotSettings') },
      { key: '/settings#email', icon: <MailOutlined />, label: t('pages.settings.emailSettings') },
      { key: '/settings#subscription', icon: <CloudServerOutlined />, label: t('pages.settings.subSettings') },
    ];
    if (showSubFormats) children.push({ key: '/settings#subscription-formats', icon: <CodeOutlined />, label: 'Sub Formats' });
    return children;
  }, [t, showSubFormats]);

  const xrayChildren = useMemo<NonNullable<MenuProps['items']>>(() => [
    { key: '/xray#basic', icon: <SettingOutlined />, label: t('pages.xray.basicTemplate') },
    { key: '/xray#balancer', icon: <ClusterOutlined />, label: t('pages.xray.Balancers') },
    { key: '/xray#dns', icon: <DatabaseOutlined />, label: 'DNS' },
    { key: '/xray#advanced', icon: <CodeOutlined />, label: t('pages.xray.advancedTemplate') },
  ], [t]);

  const selectedKey = pathname === '/settings'
    ? `/settings${hash || '#general'}`
    : pathname === '/xray'
      ? `/xray${hash || '#basic'}`
      : (pathname || '/');

  const toMenuItems = useCallback((items: typeof tabs): MenuProps['items'] => items.map((tab) => {
    const Icon = iconByName[tab.icon];
    if (tab.key === '/settings') return { key: tab.key, icon: <Icon />, label: tab.title, children: settingsChildren };
    if (tab.key === '/xray') return { key: tab.key, icon: <Icon />, label: tab.title, children: xrayChildren };
    return { key: tab.key, icon: <Icon />, label: tab.title };
  }), [settingsChildren, xrayChildren]);

  const openLink = useCallback(async (key: string) => {
    if (key === LOGOUT_KEY) {
      await HttpUtil.post('/logout');
      window.location.href = window.X_UI_BASE_PATH || '/';
      return;
    }
    navigate(key);
    setDrawerOpen(false);
  }, [navigate]);

  const onMenuClick = useCallback<NonNullable<MenuProps['onClick']>>(({ key }) => openLink(String(key)), [openLink]);

  const cycleTheme = useCallback((id: string) => {
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
  }, [isDark, isUltra, toggleTheme, toggleUltra]);

  const mobileItems = tabs.filter((item) => ['/', '/inbounds', '/clients', '/hosts'].includes(item.key));

  return (
    <>
      <div className="fara-desktop-sidebar">
        <Layout.Sider theme={currentTheme} width={256}>
          <div className="sider-brand"><BrandMark /></div>
          <div className="sider-section-label">CONTROL CENTER</div>
          <Menu
            theme={currentTheme}
            mode="inline"
            selectedKeys={[selectedKey]}
            openKeys={openKeys}
            onOpenChange={(keys) => setOpenKeys(keys as string[])}
            className="sider-nav"
            items={toMenuItems(tabs.filter((tab) => tab.key !== LOGOUT_KEY))}
            onClick={onMenuClick}
          />
          <div className="sider-bottom">
            <div className="sider-clock-card" title={serverClock.dateTimeText}>
              <ClockCircleOutlined />
              <div><strong>{serverClock.clockText}</strong><small>{serverClock.data?.timezoneLabel || ''}</small></div>
            </div>
            <div className="sider-actions-row">
              <ThemeCycleButton id="theme-cycle" isDark={isDark} isUltra={isUltra} onCycle={() => cycleTheme('theme-cycle')} ariaLabel={t('menu.theme')} />
              <VersionBadge version={panelVersion} />
              <button className="fara-icon-btn danger" type="button" title={t('logout')} onClick={() => openLink(LOGOUT_KEY)}><LogoutOutlined /></button>
            </div>
          </div>
        </Layout.Sider>
      </div>

      <header className="fara-mobile-topbar">
        <button className="fara-icon-btn" type="button" aria-label={t('menu.openMenu')} onClick={() => setDrawerOpen(true)}><MenuOutlined /></button>
        <BrandMark />
        <ThemeCycleButton id="theme-cycle-mobile" isDark={isDark} isUltra={isUltra} onCycle={() => cycleTheme('theme-cycle-mobile')} ariaLabel={t('menu.theme')} />
      </header>

      <nav className="fara-mobile-bottom-nav" aria-label="Primary">
        {mobileItems.map((item) => {
          const Icon = iconByName[item.icon];
          const active = pathname === item.key || (item.key === '/' && pathname === '');
          return (
            <button key={item.key} type="button" className={active ? 'active' : ''} onClick={() => openLink(item.key)}>
              <Icon /><span>{item.title}</span>
            </button>
          );
        })}
        <button type="button" className={drawerOpen ? 'active' : ''} onClick={() => setDrawerOpen(true)}>
          <AppstoreOutlined /><span>{t('more')}</span>
        </button>
      </nav>

      <Drawer
        placement="left"
        closable={false}
        open={drawerOpen}
        rootClassName={currentTheme}
        width="min(88vw, 360px)"
        styles={{ body: { padding: 0, display: 'flex', flexDirection: 'column', height: '100%' }, header: { display: 'none' } }}
        onClose={() => setDrawerOpen(false)}
      >
        <div className="drawer-header">
          <BrandMark />
          <button className="fara-icon-btn" type="button" aria-label={t('close')} onClick={() => setDrawerOpen(false)}><CloseOutlined /></button>
        </div>
        <div className="drawer-clock"><ClockCircleOutlined /><span>{serverClock.clockText}</span><small>{serverClock.data?.timezoneLabel || ''}</small></div>
        <Menu
          theme={currentTheme}
          mode="inline"
          selectedKeys={[selectedKey]}
          openKeys={openKeys}
          onOpenChange={(keys) => setOpenKeys(keys as string[])}
          className="drawer-menu"
          items={toMenuItems(tabs)}
          onClick={onMenuClick}
        />
        <div className="drawer-footer"><VersionBadge version={panelVersion} /></div>
      </Drawer>
    </>
  );
}
