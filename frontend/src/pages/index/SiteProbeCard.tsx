import { useState, type ReactNode } from 'react';
import { Button, Input, Modal, Space, Tooltip } from 'antd';
import {
  ApiOutlined, ArrowDownOutlined, ArrowUpOutlined, DeleteOutlined, EditOutlined,
  LoadingOutlined, PlusOutlined, RetweetOutlined, StarFilled, StarOutlined,
} from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { HttpUtil } from '@/utils';
import { usePanelDateTime } from '@/hooks/usePanelDateTime';
import './SiteProbeCard.css';

type Target = { url: string; name: string; starred: boolean };
type Path = { status: 'ok' | 'http' | 'failed' | 'timeout' | 'dns' | 'private' | 'unavailable'; code?: number; delay?: number };
type Result = { url: string; at: number; direct: Path; routed: Path };
type View = { targets: Target[]; results: Result[]; running: boolean };
const key = ['overview', 'site-probes'] as const;
const api = '/panel/api/server/siteProbes';

function Word({ path, fa, label }: { path?: Path; fa: boolean; label: string }) {
  const state = path?.status ?? 'idle';
  const text = state === 'ok' ? `${path?.delay} ms`
    : state === 'http' ? `HTTP ${path?.code}`
    : state === 'timeout' ? (fa ? 'مهلت تمام شد' : 'Timeout')
    : state === 'dns' ? 'DNS'
    : state === 'private' ? (fa ? 'آدرس غیرعمومی' : 'Non-public IP')
    : state === 'unavailable' ? (fa ? 'در دسترس نیست' : 'Unavailable')
    : state === 'failed' ? (fa ? 'ناموفق' : 'Failed') : '—';
  return (
    <span className={`site-probe-path site-probe-path--${state}`}>
      <span>{label}</span>
      <bdi title={path?.code ? `HTTP ${path.code} · ${path.delay} ms` : undefined}>{text}</bdi>
    </span>
  );
}

export default function SiteProbeCard() {
  const { t, i18n } = useTranslation();
  const fa = i18n.language?.startsWith('fa') ?? false;
  const l = (f: string, e: string) => fa ? f : e;
  const dateTime = usePanelDateTime();
  const cache = useQueryClient();
  const [open, setOpen] = useState(false);
  const [editor, setEditor] = useState<{ index?: number; name: string; url: string }>();
  const [error, setError] = useState('');
  const view = useQuery({
    queryKey: key,
    queryFn: async () => {
      const msg = await HttpUtil.get<View>(api, undefined, { silent: true });
      if (!msg.success || !msg.obj) throw new Error(msg.msg || 'Site checks unavailable');
      return msg.obj;
    },
    refetchInterval: q => q.state.data?.running ? 1500 : 30_000,
    refetchIntervalInBackground: false,
  });
  const action = useMutation({
    mutationFn: async ({ method, data }: { method: string; data: object }) => {
      const msg = await HttpUtil.post<View>(`${api}/${method}`, data, { silent: true, headers: { 'Content-Type': 'application/json' } });
      if (!msg.success || !msg.obj) throw new Error(msg.msg || 'Request failed');
      return msg.obj;
    },
    onMutate: () => setError(''),
    onSuccess: data => cache.setQueryData(key, data),
    onError: err => setError(err.message),
  });
  const targets = view.data?.targets ?? [];
  const results = new Map(view.data?.results.map(result => [result.url, result]));
  const starred = targets.filter(row => row.starred);
  const running = !!view.data?.running;
  const pending = action.isPending;
  const save = (next: Target[]) => action.mutateAsync({ method: 'save', data: { targets: next } });
  const update = (next: Target[]) => void save(next).catch(() => {});
  const run = (url = '') => action.mutate({ method: 'run', data: { url } });
  const direct = l('مستقیم', 'Direct');
  const routed = l('با روت', 'Xray route');
  const title = l('تست اتصال سایت‌ها', 'Site connectivity');
  const runLabel = l('پینگ گرفتن', 'Run checks');
  const nameLabel = l('نام سایت', 'Site name');
  const addressLabel = l('آدرس سایت', 'Site address');

  function move(index: number, offset: number) {
    const next = targets.slice();
    [next[index], next[index + offset]] = [next[index + offset], next[index]];
    update(next);
  }

  async function submitEditor() {
    if (!editor?.url.trim()) return;
    const next = targets.slice();
    if (editor.index === undefined) next.push({ url: editor.url, name: editor.name, starred: false });
    else next[editor.index] = { ...next[editor.index], url: editor.url, name: editor.name };
    try {
      await save(next);
      setEditor(undefined);
    } catch {
      // Keep the form open so the address can be corrected.
    }
  }

  return (
    <section className="site-probe-card" aria-label={title} aria-busy={running}>
      <div className="site-probe-bar">
        <strong><ApiOutlined /> {title}</strong>
        <div className="site-probe-favorites">
          {starred.map(target => {
            const result = results.get(target.url);
            return <div className="site-probe-chip" key={target.url}>
              <bdi className="site-probe-name">{target.name}</bdi>
              <Word label={direct} path={result?.direct} fa={fa} />
              <Word label={routed} path={result?.routed} fa={fa} />
            </div>;
          })}
          {!starred.length && <small>{l('برای نمایش، کنار سایت ستاره بزن.', 'Star sites to show them here.')}</small>}
        </div>
        <Button size="small" aria-label={runLabel} icon={running ? <LoadingOutlined /> : <RetweetOutlined />} onClick={() => setOpen(true)}>
          {runLabel}
        </Button>
      </div>

      <Modal title={title} open={open} footer={null} width={780}
        onCancel={() => { setOpen(false); setEditor(undefined); setError(''); }}>
        <div className="site-probe-modal">
          <p>{l(
            'هر ۱۰ دقیقه خودکار چک می‌شود. عددها زمان پاسخ HTTP هستند. تست «با روت» از قوانین روشن Xray روی یک ورودی تست می‌گذرد؛ قوانین مخصوص ورودی یا کاربر واقعی را عیناً شبیه‌سازی نمی‌کند.',
            'Checks run every 10 minutes. Times are HTTP response times. The routed test uses the live Xray rules through a test inbound; it does not simulate rules specific to a real inbound or user.',
          )}</p>
          <Space wrap>
            <Button type="primary" icon={<RetweetOutlined />} loading={running} disabled={pending || !targets.length} onClick={() => run()}>
              {l('تست همه', 'Test all')}
            </Button>
            <Button icon={<PlusOutlined />} disabled={pending || targets.length >= 12} onClick={() => { setError(''); setEditor({ url: '', name: '' }); }}>
              {l('افزودن سایت', 'Add site')}
            </Button>
            <small>{l('تا ۱۲ آدرس', 'Up to 12 sites')}</small>
          </Space>
          {(error || view.isError) && <div role="alert" className="site-probe-error">{error || view.error?.message}</div>}
          {editor && <form className="site-probe-editor" onSubmit={ev => { ev.preventDefault(); void submitEditor(); }}>
            <Input aria-label={nameLabel} placeholder={nameLabel} maxLength={255} value={editor.name}
              onChange={ev => setEditor({ ...editor, name: ev.target.value })} />
            <Input aria-label={addressLabel} placeholder="https://example.com/" required dir="ltr" maxLength={2048} value={editor.url}
              onChange={ev => setEditor({ ...editor, url: ev.target.value })} />
            <Space>
              <Button type="primary" htmlType="submit" loading={pending}>{t('save')}</Button>
              <Button onClick={() => { setEditor(undefined); setError(''); }}>{t('cancel')}</Button>
            </Space>
          </form>}
          <div className="site-probe-list">
            {targets.map((target, i) => {
              const result = results.get(target.url);
              const control = (label: string, icon: ReactNode, onClick: () => void, disabled = false) => (
                <Tooltip title={label}>
                  <Button type="text" size="small" aria-label={`${label}: ${target.name}`}
                    disabled={disabled || pending} icon={icon} onClick={onClick} />
                </Tooltip>
              );
              return <article className="site-probe-row" key={target.url}>
                <div className="site-probe-site">
                  <strong><bdi>{target.name}</bdi></strong>
                  <small dir="ltr" title={target.url}>{target.url}</small>
                </div>
                <div className="site-probe-results">
                  <Word label={direct} path={result?.direct} fa={fa} />
                  <Word label={routed} path={result?.routed} fa={fa} />
                  {result && <small>{dateTime.formatDateTime(result.at)}</small>}
                </div>
                <Space size={1} wrap className="site-probe-controls">
                  {control(target.starred ? l('برداشتن ستاره', 'Unstar') : l('ستاره‌دار کردن', 'Star'),
                    target.starred ? <StarFilled className="site-probe-star" /> : <StarOutlined />,
                    () => update(targets.map((item, j) => j === i ? { ...item, starred: !item.starred } : item)))}
                  {control(l('تست این سایت', 'Test site'), <RetweetOutlined />, () => run(target.url), running)}
                  {control(t('edit'), <EditOutlined />, () => { setError(''); setEditor({ index: i, name: target.name, url: target.url }); })}
                  {control(l('انتقال به بالا', 'Move up'), <ArrowUpOutlined />, () => move(i, -1), i === 0 || !!editor)}
                  {control(l('انتقال به پایین', 'Move down'), <ArrowDownOutlined />, () => move(i, 1), i === targets.length - 1 || !!editor)}
                  {control(t('delete'), <DeleteOutlined />, () => update(targets.filter((_, j) => j !== i)), !!editor)}
                </Space>
              </article>;
            })}
          </div>
        </div>
      </Modal>
    </section>
  );
}
