import type { ReactNode } from 'react';
import {
  ApiOutlined,
  CloudServerOutlined,
  ClusterOutlined,
  DashboardOutlined,
  GlobalOutlined,
  ImportOutlined,
  SettingOutlined,
  SwapOutlined,
  TagsOutlined,
  TeamOutlined,
  ToolOutlined,
} from '@ant-design/icons';
import './FaraPageHeader.css';

export type FaraSection =
  | 'dashboard'
  | 'inbounds'
  | 'clients'
  | 'groups'
  | 'nodes'
  | 'hosts'
  | 'settings'
  | 'xray'
  | 'outbounds'
  | 'routing'
  | 'api';

interface Props {
  section: FaraSection;
  title: ReactNode;
  actions?: ReactNode;
  meta?: ReactNode;
  compact?: boolean;
}

const copy: Record<FaraSection, { en: string; fa: string; eyebrow: string; icon: ReactNode }> = {
  dashboard: { en: 'Live system health, traffic and Xray operations in one workspace.', fa: 'سلامت زنده سرور، ترافیک و کنترل‌های Xray در یک فضای کاری.', eyebrow: 'OPERATIONS', icon: <DashboardOutlined /> },
  inbounds: { en: 'Create, inspect and operate inbound services without losing context.', fa: 'ساخت، بررسی و مدیریت ورودی‌ها بدون گم‌شدن میان تنظیمات.', eyebrow: 'ACCESS', icon: <ImportOutlined /> },
  clients: { en: 'A focused workspace for users, traffic, subscriptions and lifecycle actions.', fa: 'فضای متمرکز برای کاربران، ترافیک، اشتراک‌ها و عملیات مدیریتی.', eyebrow: 'CUSTOMERS', icon: <TeamOutlined /> },
  groups: { en: 'Organize client access and bulk operations with clear group boundaries.', fa: 'مدیریت دسترسی کاربران و عملیات گروهی با ساختار روشن.', eyebrow: 'GROUPS', icon: <TagsOutlined /> },
  nodes: { en: 'Monitor remote nodes, latency, versions and availability from one place.', fa: 'پایش نودها، تأخیر، نسخه و وضعیت دسترسی از یک صفحه.', eyebrow: 'NODES', icon: <ClusterOutlined /> },
  hosts: { en: 'Control subscription endpoints, visibility and per-user host delivery.', fa: 'کنترل هاست‌های اشتراک، نمایش و دسترسی اختصاصی هر کاربر.', eyebrow: 'HOSTS', icon: <GlobalOutlined /> },
  settings: { en: 'Panel, security, messaging and subscription settings arranged by task.', fa: 'تنظیمات پنل، امنیت، پیام‌رسان و اشتراک براساس نوع کار.', eyebrow: 'SETTINGS', icon: <SettingOutlined /> },
  xray: { en: 'Configure the Xray engine with guided sections and safer advanced access.', fa: 'پیکربندی موتور Xray با بخش‌بندی روشن و دسترسی امن‌تر به تنظیمات پیشرفته.', eyebrow: 'XRAY', icon: <ToolOutlined /> },
  outbounds: { en: 'Manage egress paths, subscriptions and connectivity tests.', fa: 'مدیریت مسیرهای خروجی، اشتراک‌های خروجی و تست اتصال.', eyebrow: 'EGRESS', icon: <CloudServerOutlined /> },
  routing: { en: 'Build and verify routing decisions with a clearer rule workflow.', fa: 'ساخت و بررسی قوانین مسیریابی با روندی روشن‌تر.', eyebrow: 'ROUTING', icon: <SwapOutlined /> },
  api: { en: 'Explore authenticated API operations and request schemas.', fa: 'بررسی عملیات API و ساختار درخواست‌ها در محیط مستندات.', eyebrow: 'API', icon: <ApiOutlined /> },
};

function isPersian(): boolean {
  if (typeof document === 'undefined') return false;
  const lang = document.documentElement.lang || navigator.language || '';
  return lang.toLowerCase().startsWith('fa') || document.documentElement.dir === 'rtl';
}

export default function FaraPageHeader({ section, title, actions, meta, compact = false }: Props) {
  const item = copy[section];
  const description = isPersian() ? item.fa : item.en;
  return (
    <section className={`fx-page-hero${compact ? ' is-compact' : ''}`}>
      <div className="fx-page-hero-main">
        <span className="fx-page-hero-icon" aria-hidden="true">{item.icon}</span>
        <div className="fx-page-hero-copy">
          <div className="fx-page-eyebrow"><i />{item.eyebrow}</div>
          <h1>{title}</h1>
          <p>{description}</p>
          {meta && <div className="fx-page-meta">{meta}</div>}
        </div>
      </div>
      {actions && <div className="fx-page-actions">{actions}</div>}
    </section>
  );
}
