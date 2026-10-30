import { useQuery } from '@tanstack/react-query';
import { Alert, Button, Space, Spin, Table, Tag, Typography } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { HttpUtil } from '@/utils';
import { IPSharedClientsSchema } from '@/generated/zod';
import { usePanelDateTime } from '@/hooks/usePanelDateTime';

export default function IPSharedClientsPanel({ ip, nodeId, serverName, email }: { ip: string; nodeId: number; serverName: string; email?: string }) {
  const { t } = useTranslation();
  const dateTime = usePanelDateTime();
  const text = (key: string) => t(`pages.clients.ipManager.${key}`);
  const query = useQuery({
    queryKey: ['server', 'ip-shared-clients', nodeId, ip],
    retry: false,
    staleTime: 5000,
    refetchInterval: 15000,
    queryFn: async () => {
      const response = await HttpUtil.get(`/panel/api/server/ipBlocks/clients?nodeId=${nodeId}&ip=${encodeURIComponent(ip)}`, undefined, { silent: true });
      if (!response?.success) throw new Error(response?.msg || 'Shared IP lookup failed');
      return IPSharedClientsSchema.parse(response.obj);
    },
  });
  return <div className="ip-manager-shared">
    <Space wrap><Typography.Text strong>{text('sharedTitle')}</Typography.Text><bdi dir="ltr">{ip}</bdi><Tag><bdi dir="auto">{serverName}</bdi></Tag><Button size="small" icon={<ReloadOutlined />} loading={query.isFetching} onClick={() => { void query.refetch(); }}>{t('refresh')}</Button></Space>
    <Typography.Text type="secondary">{text('sharedScope')}</Typography.Text>
    {query.isError ? <Alert type="warning" showIcon title={text('sharedFailed')} description={<bdi dir="auto">{query.error.message}</bdi>} /> : query.isPending ? <Spin /> : <>
      <Typography.Text>{t('pages.clients.ipManager.sharedCount', { count: query.data.clients.length })}</Typography.Text>
      <Table size="small" rowKey="email" dataSource={query.data.clients} pagination={{ pageSize: 5, showSizeChanger: false }} scroll={{ x: 320 }} locale={{ emptyText: text('sharedEmpty') }} columns={[
        { title: text('client'), dataIndex: 'email', key: 'email', render: (value: string) => <Space wrap><bdi dir="auto">{value}</bdi>{value === email ? <Tag color="blue">{text('currentClient')}</Tag> : null}</Space> },
        { title: text('lastSeen'), dataIndex: 'lastSeen', key: 'lastSeen', render: (value: number) => <bdi dir="ltr">{dateTime.formatDateTime(value)}</bdi> },
      ]} />
    </>}
  </div>;
}
