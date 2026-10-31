import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Input, Modal, Popconfirm, Select, Space, Table, Tabs, Tag, Tooltip, Typography, message } from 'antd';
import { CopyOutlined, LockOutlined, ReloadOutlined, UnlockOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';

import { ClipboardManager, HttpUtil } from '@/utils';
import { usePanelDateTime } from '@/hooks/usePanelDateTime';
import { useIPBlocks } from '@/api/queries/useIPBlocks';
import { keys } from '@/api/queryKeys';
import { NodeListSchema } from '@/schemas/node';
import { isTunnelLocalIP, canonicalBlockIP, type IPBlockMutation } from '@/schemas/api/ip-blocks';
import type { ClientIpInfo } from '@/lib/clients/ip-log';
import type { IPBlockEntry } from '@/generated/types';
import IPSharedClientsPanel from './IPSharedClientsPanel';
import './ClientIpManagerModal.css';

interface ClientIpManagerModalProps {
  open: boolean;
  email?: string;
  ips: ClientIpInfo[];
  loading: boolean;
  clearing: boolean;
  zIndex?: number;
  onRefresh: () => void | Promise<void>;
  onClear: () => void | Promise<void>;
  onClose: () => void;
}

export default function ClientIpManagerModal({ open, email, ips, loading, clearing, zIndex, onRefresh, onClear, onClose }: ClientIpManagerModalProps) {
  const { t } = useTranslation();
  const dateTime = usePanelDateTime();
  const [messageApi, contextHolder] = message.useMessage();
  const [nodeId, setNodeId] = useState(0);
  const [search, setSearch] = useState('');
  const [manualIP, setManualIP] = useState('');
  const [sharedTarget, setSharedTarget] = useState<{ ip: string; nodeId: number; serverName: string } | null>(null);
  const [confirmation, setConfirmation] = useState<(IPBlockMutation & { serverName: string }) | null>(null);
  const { query, mutation } = useIPBlocks(open, nodeId);
  const nodesQuery = useQuery({
    queryKey: keys.nodes.list(),
    enabled: open,
    queryFn: async () => {
      const response = await HttpUtil.get('/panel/api/nodes/list', undefined, { silent: true });
      if (!response?.success) throw new Error(response?.msg || 'Could not load servers');
      return NodeListSchema.parse(response.obj);
    },
  });
  const nodes = nodesQuery.data ?? [];
  const nodeName = nodes.find((node) => node.id === nodeId)?.name || '';
  const serverName = nodeId === 0 ? t('pages.clients.ipManager.localServer') : nodeName || String(nodeId);
  const ready = query.data?.available === true && !query.isError;
  const busy = mutation.isPending || query.isFetching;
  const active = useMemo(() => new Map((query.data?.blocks ?? []).map((row) => [row.ip, row])), [query.data?.blocks]);
  const searchText = search.trim().toLowerCase();
  const clientIPs = ips.filter((row) => !isTunnelLocalIP(row.ip));
  const localIPs = ips.filter((row) => isTunnelLocalIP(row.ip));
  const visibleIPs = clientIPs.filter((row) => `${row.ip} ${row.node}`.toLowerCase().includes(searchText));
  const visibleBlocks = (query.data?.blocks ?? []).filter((row) => `${row.ip} ${row.email} ${row.source}`.toLowerCase().includes(searchText));
  const text = (key: string) => t(`pages.clients.ipManager.${key}`);

  async function changeIP(request: IPBlockMutation) {
    try {
      const result = await mutation.mutateAsync(request);
      messageApi.success(text(request.action === 'ban' ? 'banSuccess' : 'unbanSuccess'));
      if (result.auditWarning) messageApi.warning(text('auditWarning'));
      if (request.action === 'ban') setManualIP('');
      setConfirmation(null);
    } catch {
      messageApi.error(text('operationFailed'));
    }
  }

  function actionButton(ip: string, blocked: boolean, owner: string, wrongServer = false) {
    if (isTunnelLocalIP(ip)) return <Tag>{text('localTunnel')}</Tag>;
    return (
      <Tooltip title={wrongServer ? text('selectObservedServer') : undefined}>
          <Button size="small" danger={!blocked} disabled={!ready || busy || !ip || wrongServer} loading={mutation.isPending && mutation.variables?.ip === ip} icon={blocked ? <UnlockOutlined /> : <LockOutlined />} onClick={() => setConfirmation({ nodeId, ip, action: blocked ? 'unban' : 'ban', email: owner, serverName })}>
            {text(blocked ? 'unban' : 'ban')}
          </Button>
      </Tooltip>
    );
  }

  function ipText(ip: string) {
    return <Space size={4}><bdi dir="ltr" className="ip-manager-address">{ip}</bdi><Button size="small" type="text" aria-label={text('copyIP')} icon={<CopyOutlined />} onClick={async () => { if (await ClipboardManager.copyText(ip)) messageApi.success(t('copied')); }} /></Space>;
  }

  function sourceTag(source: string) {
    const kind = source === 'manual' || source === 'automatic' ? source : 'unknown';
    return <Tag color={kind === 'manual' ? 'orange' : kind === 'automatic' ? 'purple' : 'default'}>{text(kind)}</Tag>;
  }

  const blockColumns = [
    { title: text('ip'), dataIndex: 'ip', key: 'ip', render: ipText },
    { title: text('sharedTitle'), key: 'shared', render: (_value: unknown, row: IPBlockEntry) => <Button size="small" onClick={() => setSharedTarget({ ip: row.ip, nodeId, serverName })}>{text('viewClients')}</Button> },
    { title: text('source'), dataIndex: 'source', key: 'source', render: sourceTag },
    { title: text('client'), dataIndex: 'email', key: 'email', render: (value: string) => <bdi dir="auto">{value || '—'}</bdi> },
    { title: text('bannedAt'), dataIndex: 'bannedAt', key: 'bannedAt', render: (value: number) => <bdi dir="ltr">{value > 0 ? dateTime.formatDateTime(value) : '—'}</bdi> },
    { title: text('expiresAt'), key: 'expiresAt', render: (_value: unknown, row: IPBlockEntry) => <bdi dir="ltr">{row.permanent ? text('permanent') : row.expiresAt > 0 ? dateTime.formatDateTime(row.expiresAt) : text('unknownTime')}</bdi> },
    { title: text('action'), key: 'action', render: (_value: unknown, row: IPBlockEntry) => actionButton(row.ip, true, row.email) },
  ];

  const observedColumns = [
    { title: text('ip'), dataIndex: 'ip', key: 'ip', render: ipText },
    { title: text('server'), key: 'node', render: (_value: unknown, row: ClientIpInfo) => <span dir="auto">{row.node || (row.nodeId === 0 ? text('localServer') : text('unknown'))}</span> },
    { title: text('sharedTitle'), key: 'shared', render: (_value: unknown, row: ClientIpInfo) => <Button size="small" disabled={row.nodeId === undefined || !canonicalBlockIP(row.ip)} onClick={() => { if (row.nodeId !== undefined) setSharedTarget({ ip: canonicalBlockIP(row.ip)!, nodeId: row.nodeId, serverName: row.node || (row.nodeId === 0 ? text('localServer') : text('unknown')) }); }}>{text('viewClients')}</Button> },
    { title: text('lastSeen'), key: 'time', render: (_value: unknown, row: ClientIpInfo) => <bdi dir="ltr">{row.timestamp ? dateTime.formatDateTime(row.timestamp) : row.time || '—'}</bdi> },
    { title: text('status'), key: 'status', render: (_value: unknown, row: ClientIpInfo) => {
      const sameServer = row.nodeId === nodeId;
      const ip = canonicalBlockIP(row.ip);
      return <Tag color={ready && sameServer && ip ? active.has(ip) ? 'red' : 'green' : 'default'}>{!ready || !sameServer || !ip ? text('unknown') : active.has(ip) ? text('blocked') : text('notBlocked')}</Tag>;
    } },
    { title: text('action'), key: 'action', render: (_value: unknown, row: ClientIpInfo) => {
      const ip = canonicalBlockIP(row.ip) || '';
      const sameServer = row.nodeId === nodeId;
      return actionButton(ip, active.has(ip), email || '', !sameServer);
    } },
  ];

  const banDuration = (query.data?.banSeconds ?? 0) < 0 ? text('permanent') : (query.data?.banSeconds ?? 0) > 0 ? t('pages.clients.ipManager.durationSeconds', { seconds: query.data?.banSeconds }) : text('unknownTime');

  return (
    <Modal open={open} title={<span>{text('title')}{email ? <> — <bdi dir="auto">{email}</bdi></> : null}</span>} width={1050} className="ip-manager-modal" zIndex={zIndex} onCancel={() => { setConfirmation(null); onClose(); }} footer={<Space wrap>
      <Button icon={<ReloadOutlined />} loading={loading || query.isFetching} disabled={mutation.isPending} onClick={() => { void onRefresh(); void query.refetch(); }}>{t('refresh')}</Button>
      <Popconfirm title={text('confirmClear')} description={text('clearOnlyLog')} onConfirm={onClear} zIndex={zIndex ? zIndex + 20 : undefined} okText={t('confirm')} cancelText={t('cancel')}><Button danger loading={clearing} disabled={ips.length === 0 || mutation.isPending}>{t('pages.clients.clearAll')}</Button></Popconfirm>
      <Button type="primary" onClick={() => { setConfirmation(null); onClose(); }}>{t('close')}</Button>
    </Space>}>
      {contextHolder}
      <Modal width={640} open={open && confirmation !== null} title={text(confirmation?.action === 'unban' ? 'confirmUnban' : 'confirmBan')} zIndex={(zIndex ?? 1000) + 30} onCancel={() => setConfirmation(null)} onOk={() => confirmation && changeIP(confirmation)} confirmLoading={mutation.isPending} cancelButtonProps={{ disabled: mutation.isPending }} closable={!mutation.isPending} keyboard={!mutation.isPending} mask={{ closable: !mutation.isPending }} okButtonProps={{ danger: confirmation?.action === 'ban' }} okText={text(confirmation?.action === 'unban' ? 'unban' : 'ban')} cancelText={t('cancel')}>
        {mutation.isError ? <Alert type="error" showIcon title={text('operationFailed')} description={<bdi dir="auto">{mutation.error.message}</bdi>} /> : null}
        <div className="ip-manager-confirm"><bdi dir="ltr">{confirmation?.ip}</bdi><span dir="auto">{confirmation?.serverName}</span><span>{text('serverScope')}</span></div>
        {open && confirmation ? <IPSharedClientsPanel key={`${confirmation.nodeId}:${confirmation.ip}`} ip={confirmation.ip} nodeId={confirmation.nodeId} serverName={confirmation.serverName} email={email} /> : null}
      </Modal>
      <div className="ip-manager">
        {localIPs.length > 0 ? <div className="ip-manager-local"><Typography.Text type="secondary">{text('localConnected')} ({localIPs.length})</Typography.Text><Space size={[4, 4]} wrap>{localIPs.map((row) => <Tooltip key={`${row.nodeId ?? row.node}:${row.ip}`} title={<span>{text('localExcluded')} — <bdi dir="auto">{row.node || (row.nodeId === 0 ? text('localServer') : text('unknown'))}</bdi>{row.timestamp ? <> — <bdi dir="ltr">{dateTime.formatDateTime(row.timestamp)}</bdi></> : null}</span>}><Tag><bdi dir="ltr">{row.ip}</bdi></Tag></Tooltip>)}</Space></div> : null}
        <Tooltip title={text('scopeDescription')}><Typography.Text type="secondary" className="ip-manager-scope">{text('scopeCompact')}</Typography.Text></Tooltip>
        {nodesQuery.isError ? <Alert type="warning" title={text('nodesFailed')} /> : null}
        <div className="ip-manager-toolbar">
          <Select size="small" aria-label={text('server')} value={nodeId} disabled={mutation.isPending} onChange={(value) => { setNodeId(value); setSharedTarget(null); mutation.reset(); }} options={[{ value: 0, label: text('localServer') }, ...nodes.map((node) => ({ value: node.id, label: node.name || String(node.id), disabled: node.enable === false }))]} />
          <Input.Search aria-label={text('search')} placeholder={text('search')} size="small" allowClear value={search} onChange={(event) => setSearch(event.target.value)} />
        </div>
        {query.isError ? <Alert type="error" showIcon title={text('loadFailed')} description={<span dir="auto">{query.error.message}</span>} /> : null}
        {mutation.isError ? <Alert type="error" showIcon title={text('operationFailed')} description={<span dir="auto">{mutation.error.message}</span>} /> : null}
        {query.data && !query.data.available ? <Alert type="warning" showIcon title={text('unavailable')} description={text(`reason_${query.data.reason}`)} /> : null}
        <div className="ip-manager-toolbar">
          <Input size="small" aria-label={text('manualIP')} placeholder={text('manualIP')} dir="ltr" value={manualIP} maxLength={128} onChange={(event) => setManualIP(event.target.value)} />
          {actionButton(canonicalBlockIP(manualIP) || '', active.has(canonicalBlockIP(manualIP) || ''), email || '')}
          <Button size="small" disabled={!canonicalBlockIP(manualIP) || isTunnelLocalIP(manualIP)} onClick={() => setSharedTarget({ ip: canonicalBlockIP(manualIP)!, nodeId, serverName })}>{text('viewClients')}</Button>
          <Typography.Text type="secondary">{text('defaultDuration')}: <bdi dir="auto">{banDuration}</bdi></Typography.Text>
        </div>
        {open && sharedTarget ? <div><IPSharedClientsPanel key={`${sharedTarget.nodeId}:${sharedTarget.ip}`} {...sharedTarget} email={email} /><Button size="small" onClick={() => setSharedTarget(null)}>{t('close')}</Button></div> : null}
        <Tabs items={[
          { key: 'observed', label: `${text('observed')} (${clientIPs.length})`, children: <Table size="small" loading={loading} columns={observedColumns} dataSource={visibleIPs} rowKey={(row) => `${row.node}:${row.ip}`} pagination={{ pageSize: 10, showSizeChanger: false }} scroll={{ x: 900 }} /> },
          { key: 'blocked', label: `${text('allBlocked')} (${ready ? active.size : '—'})`, children: <Table size="small" loading={query.isFetching} columns={blockColumns} dataSource={ready ? visibleBlocks : []} rowKey="ip" pagination={{ pageSize: 10, showSizeChanger: false }} scroll={{ x: 900 }} /> },
        ]} />
      </div>
    </Modal>
  );
}
