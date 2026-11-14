import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { Button, Card } from 'antd';
import {
  CheckCircleOutlined,
  ClockCircleOutlined,
  CloseCircleOutlined,
  MinusCircleOutlined,
  TeamOutlined,
  WifiOutlined,
} from '@ant-design/icons';

import { keys } from '@/api/queryKeys';
import { ClientsSummarySchema } from '@/schemas/client';
import { HttpUtil } from '@/utils';

const STATS = [
  { key: 'total', label: 'clients', tone: 'total', Icon: TeamOutlined },
  { key: 'onlineCount', label: 'online', tone: 'online', Icon: WifiOutlined },
  { key: 'active', label: 'subscription.active', tone: 'active', Icon: CheckCircleOutlined },
  { key: 'expiringCount', label: 'depletingSoon', tone: 'warning', Icon: ClockCircleOutlined },
  { key: 'depletedCount', label: 'depleted', tone: 'error', Icon: CloseCircleOutlined },
  { key: 'deactiveCount', label: 'disabled', tone: 'disabled', Icon: MinusCircleOutlined },
] as const;

export default function ClientOverviewCard() {
  const { t, i18n } = useTranslation();
  const { data, isError, isFetching, refetch } = useQuery({
    queryKey: keys.clients.overview(),
    queryFn: async ({ signal }) => {
      // The paged endpoint returns global counts regardless of page size.
      const msg = await HttpUtil.get<{ summary?: unknown }>(
        '/panel/api/clients/list/paged', { page: 1, pageSize: 1 }, { signal, silent: true },
      );
      if (!msg.success) throw new Error(msg.msg);
      return ClientsSummarySchema.parse(msg.obj?.summary);
    },
    staleTime: 15_000,
    refetchInterval: 20_000,
    refetchIntervalInBackground: false,
  });
  const format = new Intl.NumberFormat(i18n.resolvedLanguage || i18n.language || 'en');

  return (
    <Card className="ov-client-card" styles={{ body: { padding: 0 } }}>
      <section dir={i18n.dir()} aria-label={t('clients')} aria-busy={isFetching && !data}>
        <div className="ov-client-head">
          <h2 className="ov-kicker">{t('clients')}</h2>
          <Link to="/clients">{t('pages.index.openClients')}</Link>
        </div>
        <dl className="ov-client-grid">
          {STATS.map(({ key, label, tone, Icon }) => (
            <div className="ov-client-stat" data-tone={tone} key={key}>
              <dt>
                <span className="ov-client-icon" aria-hidden="true"><Icon /></span>
                {t(label)}
              </dt>
              <dd><bdi>{data ? format.format(data[key]) : '—'}</bdi></dd>
            </div>
          ))}
        </dl>
        {isError && (
          <div className="ov-client-error" role="status">
            {t('somethingWentWrong')}
            <Button type="link" size="small" loading={isFetching} onClick={() => void refetch()}>
              {t('refresh')}
            </Button>
          </div>
        )}
      </section>
    </Card>
  );
}
