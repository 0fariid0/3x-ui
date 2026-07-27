import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Input, InputNumber, Modal, Select, Space, Switch, Tabs } from 'antd';
import {
  BarChartOutlined,
  ClockCircleOutlined,
  FileTextOutlined,
  ReloadOutlined,
  SettingOutlined,
} from '@ant-design/icons';

import { OutboundDomainStrategies } from '@/schemas/primitives';
import { HappyEyeballsSchema } from '@/schemas/protocols/stream/sockopt';
import { SettingListItem } from '@/components/ui';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { catTabLabel } from '@/pages/settings/catTabLabel';
import type { XraySettingsValue, SetTemplate, ScheduledRestartUnit } from '@/hooks/useXraySetting';
import './BasicsTab.css';

import {
  ACCESS_LOG,
  ERROR_LOG,
  LOG_LEVELS,
  MASK_ADDRESS,
  ROUTING_DOMAIN_STRATEGIES,
} from './constants';

interface BasicsTabProps {
  templateSettings: XraySettingsValue | null;
  setTemplateSettings: SetTemplate;
  outboundTestUrl: string;
  onChangeOutboundTestUrl: (v: string) => void;
  scheduledRestartEnable: boolean;
  onChangeScheduledRestartEnable: (v: boolean) => void;
  scheduledRestartInterval: number;
  onChangeScheduledRestartInterval: (v: number) => void;
  scheduledRestartUnit: ScheduledRestartUnit;
  onChangeScheduledRestartUnit: (v: ScheduledRestartUnit) => void;
  scheduledRestartPanel: boolean;
  onChangeScheduledRestartPanel: (v: boolean) => void;
  xrayHealthEnable: boolean;
  onChangeXrayHealthEnable: (v: boolean) => void;
  xrayHealthFailureThreshold: number;
  onChangeXrayHealthFailureThreshold: (v: number) => void;
  xrayHealthRestartCooldown: number;
  onChangeXrayHealthRestartCooldown: (v: number) => void;
  xrayHealthMaxRestarts: number;
  onChangeXrayHealthMaxRestarts: (v: number) => void;
  xrayHealthWindowMinutes: number;
  onChangeXrayHealthWindowMinutes: (v: number) => void;
  onResetDefault: () => void;
}

export default function BasicsTab({
  templateSettings,
  setTemplateSettings,
  outboundTestUrl,
  onChangeOutboundTestUrl,
  scheduledRestartEnable,
  onChangeScheduledRestartEnable,
  scheduledRestartInterval,
  onChangeScheduledRestartInterval,
  scheduledRestartUnit,
  onChangeScheduledRestartUnit,
  scheduledRestartPanel,
  onChangeScheduledRestartPanel,
  xrayHealthEnable,
  onChangeXrayHealthEnable,
  xrayHealthFailureThreshold,
  onChangeXrayHealthFailureThreshold,
  xrayHealthRestartCooldown,
  onChangeXrayHealthRestartCooldown,
  xrayHealthMaxRestarts,
  onChangeXrayHealthMaxRestarts,
  xrayHealthWindowMinutes,
  onChangeXrayHealthWindowMinutes,
  onResetDefault,
}: BasicsTabProps) {
  const { t } = useTranslation();
  const { isMobile } = useMediaQuery();
  const [modal, modalContextHolder] = Modal.useModal();

  const mutate = useCallback(
    (mutator: (next: XraySettingsValue) => void) => {
      setTemplateSettings((prev) => {
        if (!prev) return prev;
        const clone = JSON.parse(JSON.stringify(prev)) as XraySettingsValue;
        mutator(clone);
        return clone;
      });
    },
    [setTemplateSettings],
  );

  const setLevel0 = useCallback(
    (field: string, value: number | null) => mutate((tt) => {
      if (!tt.policy) tt.policy = {};
      if (!tt.policy.levels) tt.policy.levels = {};
      if (!tt.policy.levels['0']) tt.policy.levels['0'] = {};
      if (value === null || value === undefined) {
        delete tt.policy.levels['0'][field];
      } else {
        tt.policy.levels['0'][field] = value;
      }
    }),
    [mutate],
  );

  const metricsCfg = (templateSettings as { metrics?: { tag?: string; listen?: string } } | null)?.metrics;

  const setMetrics = useCallback(
    (field: 'tag' | 'listen', value: string) => mutate((tt) => {
      const node = tt as { metrics?: { tag?: string; listen?: string }; stats?: Record<string, unknown> };
      const m: { tag?: string; listen?: string } = { ...(node.metrics ?? {}) };
      if (value.trim() === '') {
        delete m[field];
      } else {
        m[field] = value.trim();
      }
      if (!m.listen && !m.tag) {
        delete node.metrics;
      } else {
        node.metrics = m;
        // xray-core's metrics handler needs a stats object to populate.
        if (!node.stats) node.stats = {};
      }
    }),
    [mutate],
  );

  function confirmResetDefault() {
    modal.confirm({
      title: t('pages.settings.resetDefaultConfig'),
      okText: t('reset'),
      okType: 'danger',
      cancelText: t('cancel'),
      onOk: () => onResetDefault(),
    });
  }

  const freedomStrategy =
    (templateSettings?.outbounds?.find((o) => o?.protocol === 'freedom' && o?.tag === 'direct')?.settings as
      | { domainStrategy?: string }
      | undefined)?.domainStrategy ?? 'AsIs';

  const directFreedomOutbound = templateSettings?.outbounds?.find(
    (o) => o?.protocol === 'freedom' && o?.tag === 'direct',
  );
  const directHappyEyeballs = (() => {
    const sockopt = (directFreedomOutbound?.streamSettings as { sockopt?: { happyEyeballs?: unknown } } | undefined)
      ?.sockopt;
    const raw = sockopt?.happyEyeballs;
    if (raw == null || typeof raw !== 'object') return null;
    const parsed = HappyEyeballsSchema.safeParse(raw);
    return parsed.success ? parsed.data : null;
  })();

  const setDirectHappyEyeballs = useCallback(
    (next: ReturnType<typeof HappyEyeballsSchema.parse> | null) => {
      mutate((tt) => {
        if (!tt.outbounds) tt.outbounds = [];
        let idx = tt.outbounds.findIndex((o) => o?.protocol === 'freedom' && o?.tag === 'direct');
        if (idx < 0) {
          tt.outbounds.push({ protocol: 'freedom', tag: 'direct', settings: {} });
          idx = tt.outbounds.length - 1;
        }
        const ob = tt.outbounds[idx];
        const stream = (ob.streamSettings ?? {}) as Record<string, unknown>;
        const sockopt = (stream.sockopt ?? {}) as Record<string, unknown>;
        if (next == null) {
          delete sockopt.happyEyeballs;
        } else {
          sockopt.happyEyeballs = next;
        }
        if (Object.keys(sockopt).length === 0) {
          delete stream.sockopt;
        } else {
          stream.sockopt = sockopt;
        }
        if (Object.keys(stream).length === 0) {
          delete ob.streamSettings;
        } else {
          ob.streamSettings = stream;
        }
      });
    },
    [mutate],
  );

  const routingStrategy = templateSettings?.routing?.domainStrategy ?? 'AsIs';
  const log = (templateSettings?.log || {}) as Record<string, unknown>;
  const policy = (templateSettings?.policy?.system || {}) as Record<string, boolean>;
  const level0 = (templateSettings?.policy?.levels?.['0'] || {}) as Record<string, unknown>;

  const items = [
    {
      key: '1',
      label: catTabLabel(<SettingOutlined />, t('pages.xray.generalConfigs'), isMobile),
      children: (
        <>
          <Alert
            type="warning"
            showIcon
            className="mb-12 hint-alert"
            title={t('pages.xray.generalConfigsDesc')}
          />
          <SettingListItem
            title={t('pages.xray.FreedomStrategy')}
            description={t('pages.xray.FreedomStrategyDesc')}
            paddings="small"
            control={
              <Select
                value={freedomStrategy}
                style={{ width: '100%' }}
                options={OutboundDomainStrategies.map((s) => ({ value: s, label: s }))}
                onChange={(next) => mutate((tt) => {
                  if (!tt.outbounds) tt.outbounds = [];
                  const idx = tt.outbounds.findIndex((o) => o?.protocol === 'freedom' && o?.tag === 'direct');
                  if (idx < 0) {
                    tt.outbounds.push({ protocol: 'freedom', tag: 'direct', settings: { domainStrategy: next } });
                  } else {
                    const ob = tt.outbounds[idx];
                    ob.settings = (ob.settings || {}) as Record<string, unknown>;
                    (ob.settings as Record<string, unknown>).domainStrategy = next;
                  }
                })}
              />
            }
          />
          <SettingListItem
            title={t('pages.xray.FreedomHappyEyeballs')}
            description={t('pages.xray.FreedomHappyEyeballsDesc')}
            paddings="small"
            control={
              <Switch
                checked={directHappyEyeballs != null}
                onChange={(checked) => {
                  setDirectHappyEyeballs(checked ? HappyEyeballsSchema.parse({}) : null);
                }}
              />
            }
          />
          {directHappyEyeballs != null && (
            <>
              <SettingListItem
                title={t('pages.inbounds.form.tryDelayMs')}
                description={t('pages.xray.FreedomHappyEyeballsTryDelayDesc')}
                paddings="small"
                control={
                  <InputNumber
                    min={0}
                    style={{ width: '100%' }}
                    value={directHappyEyeballs.tryDelayMs}
                    placeholder="150"
                    onChange={(v) => setDirectHappyEyeballs({
                      ...directHappyEyeballs,
                      tryDelayMs: typeof v === 'number' ? v : 0,
                    })}
                  />
                }
              />
              <SettingListItem
                title={t('pages.inbounds.form.prioritizeIPv6')}
                paddings="small"
                control={
                  <Switch
                    checked={directHappyEyeballs.prioritizeIPv6}
                    onChange={(checked) => setDirectHappyEyeballs({
                      ...directHappyEyeballs,
                      prioritizeIPv6: checked,
                    })}
                  />
                }
              />
            </>
          )}
          <SettingListItem
            title={t('pages.xray.RoutingStrategy')}
            description={t('pages.xray.RoutingStrategyDesc')}
            paddings="small"
            control={
              <Select
                value={routingStrategy}
                style={{ width: '100%' }}
                options={ROUTING_DOMAIN_STRATEGIES.map((s) => ({ value: s, label: s }))}
                onChange={(next) => mutate((tt) => {
                  if (tt.routing) tt.routing.domainStrategy = next;
                })}
              />
            }
          />
          <SettingListItem
            title={t('pages.xray.outboundTestUrl')}
            description={t('pages.xray.outboundTestUrlDesc')}
            paddings="small"
            control={
              <Input
                value={outboundTestUrl}
                onChange={(e) => onChangeOutboundTestUrl(e.target.value)}
                placeholder="https://www.google.com/generate_204"
              />
            }
          />
          <SettingListItem
            title={t('pages.xray.scheduledRestart')}
            description={t('pages.xray.scheduledRestartDesc')}
            paddings="small"
            control={
              <Switch
                checked={scheduledRestartEnable}
                onChange={onChangeScheduledRestartEnable}
              />
            }
          />
          {scheduledRestartEnable && (
            <>
              <SettingListItem
                title={t('pages.xray.scheduledRestartInterval')}
                description={t('pages.xray.scheduledRestartIntervalDesc')}
                paddings="small"
                control={
                  <Space.Compact block>
                    <InputNumber
                      min={1}
                      value={scheduledRestartInterval}
                      style={{ width: '60%' }}
                      onChange={(value) => onChangeScheduledRestartInterval(Math.max(1, Number(value) || 1))}
                    />
                    <Select
                      value={scheduledRestartUnit}
                      style={{ width: '40%' }}
                      options={[
                        { value: 'minutes', label: t('pages.xray.restartUnitMinutes') },
                        { value: 'hours', label: t('pages.xray.restartUnitHours') },
                        { value: 'days', label: t('pages.xray.restartUnitDays') },
                      ]}
                      onChange={(value) => onChangeScheduledRestartUnit(value as ScheduledRestartUnit)}
                    />
                  </Space.Compact>
                }
              />
              <SettingListItem
                title={t('pages.xray.scheduledRestartPanel')}
                description={t('pages.xray.scheduledRestartPanelDesc')}
                paddings="small"
                control={
                  <Switch
                    checked={scheduledRestartPanel}
                    onChange={onChangeScheduledRestartPanel}
                  />
                }
              />
              <Alert
                type="info"
                showIcon
                className="mb-12 hint-alert"
                title={scheduledRestartPanel
                  ? t('pages.xray.scheduledRestartPanelHint')
                  : t('pages.xray.scheduledRestartXrayHint')}
              />
            </>
          )}
          <SettingListItem
            title={t('pages.xray.xrayHealthMonitor')}
            description={t('pages.xray.xrayHealthMonitorDesc')}
            paddings="small"
            control={<Switch checked={xrayHealthEnable} onChange={onChangeXrayHealthEnable} />}
          />
          {xrayHealthEnable && (
            <>
              <SettingListItem
                title={t('pages.xray.xrayHealthFailureThreshold')}
                description={t('pages.xray.xrayHealthFailureThresholdDesc')}
                paddings="small"
                control={
                  <InputNumber
                    min={1}
                    max={60}
                    value={xrayHealthFailureThreshold}
                    onChange={(value) => onChangeXrayHealthFailureThreshold(Math.max(1, Number(value) || 1))}
                  />
                }
              />
              <SettingListItem
                title={t('pages.xray.xrayHealthRestartCooldown')}
                description={t('pages.xray.xrayHealthRestartCooldownDesc')}
                paddings="small"
                control={
                  <InputNumber
                    min={1}
                    max={1440}
                    addonAfter={t('pages.xray.restartUnitMinutes')}
                    value={xrayHealthRestartCooldown}
                    onChange={(value) => onChangeXrayHealthRestartCooldown(Math.max(1, Number(value) || 1))}
                  />
                }
              />
              <SettingListItem
                title={t('pages.xray.xrayHealthMaxRestarts')}
                description={t('pages.xray.xrayHealthMaxRestartsDesc')}
                paddings="small"
                control={
                  <InputNumber
                    min={1}
                    max={100}
                    value={xrayHealthMaxRestarts}
                    onChange={(value) => onChangeXrayHealthMaxRestarts(Math.max(1, Number(value) || 1))}
                  />
                }
              />
              <SettingListItem
                title={t('pages.xray.xrayHealthWindowMinutes')}
                description={t('pages.xray.xrayHealthWindowMinutesDesc')}
                paddings="small"
                control={
                  <InputNumber
                    min={1}
                    max={1440}
                    addonAfter={t('pages.xray.restartUnitMinutes')}
                    value={xrayHealthWindowMinutes}
                    onChange={(value) => onChangeXrayHealthWindowMinutes(Math.max(1, Number(value) || 1))}
                  />
                }
              />
              <Alert type="info" showIcon className="mb-12 hint-alert" title={t('pages.xray.xrayHealthLoopProtectionHint')} />
            </>
          )}
        </>
      ),
    },
    {
      key: '2',
      label: catTabLabel(<BarChartOutlined />, t('pages.xray.statistics'), isMobile),
      children: (
        <>
          {[
            ['statsInboundUplink', t('pages.xray.statsInboundUplink')],
            ['statsInboundDownlink', t('pages.xray.statsInboundDownlink')],
            ['statsOutboundUplink', t('pages.xray.statsOutboundUplink')],
            ['statsOutboundDownlink', t('pages.xray.statsOutboundDownlink')],
          ].map(([field, label]) => (
            <SettingListItem
              key={field}
              title={label}
              paddings="small"
              control={
                <Switch
                  checked={!!policy[field]}
                  onChange={(checked) => mutate((tt) => {
                    if (!tt.policy) tt.policy = {};
                    if (!tt.policy.system) tt.policy.system = {};
                    tt.policy.system[field] = checked;
                  })}
                />
              }
            />
          ))}
          <SettingListItem
            title={t('pages.xray.metricsListen')}
            description={t('pages.xray.metricsListenDesc')}
            paddings="small"
            control={
              <Input
                value={metricsCfg?.listen ?? ''}
                onChange={(e) => setMetrics('listen', e.target.value)}
                placeholder="127.0.0.1:11111"
              />
            }
          />
          <SettingListItem
            title={t('pages.xray.metricsTag')}
            paddings="small"
            control={
              <Input
                value={metricsCfg?.tag ?? ''}
                onChange={(e) => setMetrics('tag', e.target.value)}
                placeholder="metrics_out"
              />
            }
          />
        </>
      ),
    },
    {
      key: 'connection',
      label: catTabLabel(<ClockCircleOutlined />, t('pages.xray.connectionLimits'), isMobile),
      children: (
        <>
          <Alert
            type="warning"
            showIcon
            className="mb-12 hint-alert"
            title={t('pages.xray.connectionLimitsDesc')}
          />
          <SettingListItem
            title={t('pages.xray.connIdle')}
            description={t('pages.xray.connIdleDesc')}
            paddings="small"
            control={
              <InputNumber
                value={typeof level0.connIdle === 'number' ? level0.connIdle : undefined}
                min={0}
                style={{ width: '100%' }}
                placeholder="300"
                suffix={t('pages.xray.seconds')}
                onChange={(v) => setLevel0('connIdle', v as number | null)}
              />
            }
          />
          <SettingListItem
            title={t('pages.xray.bufferSize')}
            description={t('pages.xray.bufferSizeDesc')}
            paddings="small"
            control={
              <InputNumber
                value={typeof level0.bufferSize === 'number' ? level0.bufferSize : undefined}
                min={0}
                style={{ width: '100%' }}
                placeholder={t('pages.xray.bufferSizePlaceholder')}
                suffix="KB"
                onChange={(v) => setLevel0('bufferSize', v as number | null)}
              />
            }
          />
        </>
      ),
    },
    {
      key: '3',
      label: catTabLabel(<FileTextOutlined />, t('pages.xray.logConfigs'), isMobile),
      children: (
        <>
          <Alert
            type="warning"
            showIcon
            className="mb-12 hint-alert"
            title={t('pages.xray.logConfigsDesc')}
          />
          <SettingListItem
            title={t('pages.xray.logLevel')}
            description={t('pages.xray.logLevelDesc')}
            paddings="small"
            control={
              <Select
                value={(log.loglevel as string) || 'warning'}
                style={{ width: '100%' }}
                options={LOG_LEVELS.map((s) => ({ value: s, label: s }))}
                onChange={(v) => mutate((tt) => { if (tt.log) tt.log.loglevel = v; })}
              />
            }
          />
          <SettingListItem
            title={t('pages.xray.accessLog')}
            description={t('pages.xray.accessLogDesc')}
            paddings="small"
            control={
              <Select
                value={(log.access as string) || ''}
                style={{ width: '100%' }}
                options={ACCESS_LOG.map((s) => ({ value: s, label: s }))}
                onChange={(v) => mutate((tt) => { if (tt.log) tt.log.access = v; })}
              />
            }
          />
          <SettingListItem
            title={t('pages.xray.errorLog')}
            description={t('pages.xray.errorLogDesc')}
            paddings="small"
            control={
              <Select
                value={(log.error as string) || ''}
                style={{ width: '100%' }}
                options={[{ value: '', label: t('empty') }, ...ERROR_LOG.map((s) => ({ value: s, label: s }))]}
                onChange={(v) => mutate((tt) => { if (tt.log) tt.log.error = v; })}
              />
            }
          />
          <SettingListItem
            title={t('pages.xray.maskAddress')}
            description={t('pages.xray.maskAddressDesc')}
            paddings="small"
            control={
              <Select
                value={(log.maskAddress as string) || ''}
                style={{ width: '100%' }}
                options={[{ value: '', label: t('empty') }, ...MASK_ADDRESS.map((s) => ({ value: s, label: s }))]}
                onChange={(v) => mutate((tt) => { if (tt.log) tt.log.maskAddress = v; })}
              />
            }
          />
          <SettingListItem
            title={t('pages.xray.dnsLog')}
            description={t('pages.xray.dnsLogDesc')}
            paddings="small"
            control={
              <Switch
                checked={!!log.dnsLog}
                onChange={(v) => mutate((tt) => { if (tt.log) tt.log.dnsLog = v; })}
              />
            }
          />
        </>
      ),
    },
    {
      key: 'reset',
      label: catTabLabel(<ReloadOutlined />, t('pages.settings.resetDefaultConfig'), isMobile),
      children: (
        <Space style={{ padding: '0 20px' }}>
          <Button type="primary" danger icon={<ReloadOutlined />} onClick={confirmResetDefault}>
            {t('pages.settings.resetDefaultConfig')}
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <>
      {modalContextHolder}
      <Tabs defaultActiveKey="1" items={items} />
    </>
  );
}
