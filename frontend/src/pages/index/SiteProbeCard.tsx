import { useState, type ReactNode } from 'react';
import { Button, Input, Modal, Radio, Select, Space, Tooltip } from 'antd';
import {
  ArrowDownOutlined, ArrowUpOutlined, DeleteOutlined, EditOutlined,
  PlusOutlined, RetweetOutlined, SettingOutlined, StarFilled, StarOutlined,
} from '@ant-design/icons';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { HttpUtil } from '@/utils';
import { usePanelDateTime } from '@/hooks/usePanelDateTime';
import './SiteProbeCard.css';

type Target = { url: string; name: string; starred: boolean };
type Path = { status: 'ok' | 'http' | 'failed' | 'timeout' | 'dns' | 'private' | 'unavailable'; code?: number; delay?: number };
type Result = { url: string; at: number; ping: Path };
type Mode = 'tcp' | 'http' | 'real';
type OutboundTarget = { tag: string; mode: Mode };
type OutboundResult = OutboundTarget & { at: number; ping: Path; effectiveMode?: Mode };
type OutboundOption = { tag: string; protocol: string };
type View = { targets: Target[]; results: Result[]; running: boolean; outboundTargets: OutboundTarget[]; outboundResults: OutboundResult[] };
const key = ['overview', 'site-probes'] as const;
const api = '/panel/api/server/siteProbes';

function Word({ path, fa }: { path?: Path; fa: boolean }) {
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
  const [outboundTag, setOutboundTag] = useState<string>();
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
  const outboundOptions = useQuery({
    queryKey: [...key, 'outbound-options'],
    enabled: open,
    queryFn: async () => {
      const msg = await HttpUtil.get<OutboundOption[]>(`${api}/outboundOptions`, undefined, { silent: true });
      if (!msg.success || !msg.obj) throw new Error(msg.msg || 'Outbound list unavailable');
      return msg.obj;
    },
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
  const outbounds = view.data?.outboundTargets ?? [];
  const outboundResults = new Map(view.data?.outboundResults?.map(result => [result.tag, result]));
  const availableOutbounds = (outboundOptions.data ?? []).filter(row => !outbounds.some(o => o.tag === row.tag));
  const modeOptions = [
    { label: 'TCP', value: 'tcp' },
    { label: 'HTTP', value: 'http' },
    { label: t('pages.xray.outbound.modeRealDelay'), value: 'real' },
  ] as const;
  const modeLabel = (mode: Mode) => modeOptions.find(row => row.value === mode)?.label ?? mode;
  const running = !!view.data?.running;
  const pending = action.isPending;
  const testing = running || (pending && action.variables?.method === 'run');
  const save = (next: Target[]) => action.mutateAsync({ method: 'save', data: { targets: next } });
  const update = (next: Target[]) => void save(next).catch(() => {});
  const updateOutbounds = (next: OutboundTarget[]) => action.mutate({ method: 'outbounds', data: { targets: next } });
  const run = (url = '') => action.mutate({ method: 'run', data: { url } });
  const title = l('تست اتصال سایت‌ها', 'Site connectivity');
  const runLabel = l('تست همه', 'Test all');
  const settingsLabel = l('تنظیمات تست سایت‌ها', 'Site check settings');
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
    <section className="site-probe-card" aria-label={title} aria-busy={testing}>
      <div className="site-probe-bar">
        <div className="site-probe-favorites">
          {starred.map(target => {
            const result = results.get(target.url);
            return <div className="site-probe-chip" key={target.url}>
              <bdi className="site-probe-name">{target.name}</bdi>
              <Word path={result?.ping} fa={fa} />
            </div>;
          })}
          {outbounds.map(target => {
            const result = outboundResults.get(target.tag);
            return <div className="site-probe-chip site-probe-chip--outbound" key={target.tag}
              title={`${l('خروجی', 'Outbound')} · ${modeLabel(result?.effectiveMode ?? target.mode)}`}>
              <bdi className="site-probe-name">{target.tag}</bdi>
              <Word path={result?.ping} fa={fa} />
            </div>;
          })}
          {!starred.length && !outbounds.length && <small>{l('یک سایت را ستاره‌دار کن یا خروجی اضافه کن.', 'Star a site or add an outbound.')}</small>}
        </div>
        <div className="site-probe-actions">
          <Button size="small" color="blue" variant="solid" aria-label={runLabel}
            icon={<RetweetOutlined />} loading={testing} disabled={pending || testing || (!targets.length && !outbounds.length)}
            onClick={() => run()}>
            {runLabel}
          </Button>
          <Button size="small" color="red" variant="filled" aria-label={settingsLabel}
            icon={<SettingOutlined />} onClick={() => setOpen(true)}>
            {l('تنظیمات', 'Settings')}
          </Button>
        </div>
      </div>
      {!open && error && <div role="alert" className="site-probe-error">{error}</div>}

      <Modal title={settingsLabel} open={open} footer={null} width={780}
        onCancel={() => { setOpen(false); setEditor(undefined); setOutboundTag(undefined); setError(''); }}>
        <div className="site-probe-modal">
          <p title={l('تست از ورودی مخصوص پنل اجرا می‌شود؛ قوانین مختص یک ورودی یا کاربر دیگر در این تست اعمال نمی‌شوند.', 'Rules restricted to other inbounds or users are outside this test.')}>{l(
            'هر ۱۰ دقیقه از مسیر تعیین‌شده در Xray تست می‌شود. اگر قانونی جور نشود، اتصال مستقیم است. عدد کنار هر سایت زمان پاسخ وب است.',
            'Every 10 minutes, the check uses the matching Xray route, or connects directly if no rule matches. Each site shows one HTTP response time.',
          )}</p>
          <Space wrap>
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
                  <Word path={result?.ping} fa={fa} />
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
          <section className="site-probe-outbound-settings" aria-label={l('خروجی‌های تست', 'Outbound checks')}>
            <strong>{l('خروجی‌ها', 'Outbounds')}</strong>
            <p>{l('«تست همه» هر خروجی را با روش انتخاب‌شده بررسی می‌کند. برای خروجی‌های UDP، حالت TCP به HTTP تغییر می‌کند.',
              'Test all uses the saved mode for each outbound. TCP falls back to HTTP for UDP outbounds.')}</p>
            <div className="site-probe-outbound-add">
              <Select aria-label={l('انتخاب خروجی', 'Select outbound')} value={outboundTag}
                placeholder={l('یک خروجی انتخاب کن', 'Select an outbound')} loading={outboundOptions.isFetching}
                disabled={pending || outbounds.length >= 12}
                showSearch={{ optionFilterProp: 'label' }}
                onChange={setOutboundTag} options={availableOutbounds.map(({ tag, protocol }) => ({ value: tag, label: `${tag} · ${protocol}` }))} />
              <Button icon={<PlusOutlined />} disabled={pending || outbounds.length >= 12 || !availableOutbounds.some(o => o.tag === outboundTag)}
                onClick={() => {
                  if (!outboundTag) return;
                  updateOutbounds([...outbounds, { tag: outboundTag, mode: 'tcp' }]);
                  setOutboundTag(undefined);
                }}>{l('افزودن خروجی', 'Add outbound')}</Button>
            </div>
            {outboundOptions.isError && <div role="alert" className="site-probe-error">{outboundOptions.error.message}</div>}
            <div className="site-probe-list">
              {outbounds.map(target => {
                const result = outboundResults.get(target.tag);
                return <div className="site-probe-outbound-row" key={target.tag}>
                  <bdi className="site-probe-name" title={target.tag}>{target.tag}</bdi>
                  <Space size={6} wrap>
                    <Radio.Group size="small" buttonStyle="solid" optionType="button"
                      aria-label={`${l('نوع تست', 'Test mode')}: ${target.tag}`}
                      disabled={pending} value={target.mode} options={[...modeOptions]}
                      onChange={e => updateOutbounds(outbounds.map(row => row.tag === target.tag ? { ...row, mode: e.target.value as Mode } : row))} />
                    <Word path={result?.ping} fa={fa} />
                    {result?.effectiveMode && result.effectiveMode !== target.mode &&
                      <small title={l('روش اجراشده', 'Actual test mode')}>{modeLabel(result.effectiveMode)}</small>}
                    <Button type="text" size="small" danger icon={<DeleteOutlined />} disabled={pending}
                      aria-label={`${t('delete')}: ${target.tag}`}
                      onClick={() => updateOutbounds(outbounds.filter(row => row.tag !== target.tag))} />
                  </Space>
                </div>;
              })}
            </div>
          </section>
        </div>
      </Modal>
    </section>
  );
}
