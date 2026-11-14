import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Checkbox,
  Form,
  Input,
  InputNumber,
  Modal,
  Radio,
  Select,
  Space,
  Tabs,
  message,
} from 'antd';
import { Controller, FormProvider, useForm, useWatch } from 'react-hook-form';
import { FinalMaskField, SniffingField } from '@/lib/xray/forms/fields';
import { FormField, rhfZodValidate } from '@/components/form/rhf';
import { JsonEditor } from '@/components/form';
import { Wireguard } from '@/utils';
import {
  formValuesToWirePayload,
  rawOutboundToFormValues,
} from '@/lib/xray/outbound-form-adapter';
import { parseOutboundLink } from '@/lib/xray/outbound-link-parser';
import { XMUX_FRESH_DEFAULTS } from '@/schemas/protocols/stream/xhttp';
import {
  OutboundFormBaseSchema,
  type OutboundFormValues,
} from '@/schemas/forms/outbound-form';
import {
  canEnableReality,
  canEnableStream,
  canEnableTls,
  canEnableTlsFlow,
} from '@/lib/xray/protocol-capabilities';

import {
  FLOW_OPTIONS,
  HYSTERIA_NETWORK_OPTION,
  NETWORK_OPTIONS,
  PROTOCOL_OPTIONS,
  SERVER_PROTOCOLS,
  TARGET_STRATEGY_OPTIONS,
} from './outbound-form-constants';
import {
  applyNetworkChange,
  buildAddModeValues,
  hysteriaStreamSlice,
  newStreamSlice,
} from './outbound-form-helpers';
import {
  BlackholeFields,
  DnsFields,
  FreedomFields,
  HttpFields,
  LoopbackFields,
  ServerTarget,
  ShadowsocksFields,
  SocksFields,
  TrojanFields,
  VlessFields,
  VmessFields,
  WireguardFields,
} from './protocols';
import {
  GrpcForm,
  HttpUpgradeForm,
  HysteriaForm,
  KcpForm,
  MuxForm,
  RawForm,
  SockoptForm,
  WsForm,
  XhttpForm,
} from './transport';
import { RealityForm, TlsForm } from './security';
import './OutboundFormModal.css';

type StreamValue = OutboundFormValues['streamSettings'];

interface OutboundFormModalProps {
  open: boolean;
  outbound: Record<string, unknown> | null;
  existingTags: string[];
  quickRouting?: boolean;
  quickRoutingReservedTags?: string[];
  dialerProxyTags?: string[];
  onClose: () => void;
  onConfirm: (outbound: Record<string, unknown>, quickRouting: boolean) => void;
}

export default function OutboundFormModal({
  open,
  outbound: outboundProp,
  existingTags,
  quickRouting = false,
  quickRoutingReservedTags = [],
  dialerProxyTags,
  onClose,
  onConfirm,
}: OutboundFormModalProps) {
  const { t, i18n } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const methods = useForm<OutboundFormValues>({ defaultValues: buildAddModeValues() });
  const [activeKey, setActiveKey] = useState('1');
  const [jsonText, setJsonText] = useState('');
  const [jsonDirty, setJsonDirty] = useState(false);
  const [formSnapshot, setFormSnapshot] = useState('');
  const [linkInput, setLinkInput] = useState('');
  const [showInBasicRouting, setShowInBasicRouting] = useState(quickRouting);

  const isEdit = outboundProp != null;
  const jsonOnly = isEdit && !PROTOCOL_OPTIONS.some(option => option.value === outboundProp?.protocol);
  const fa = i18n.language?.startsWith('fa');
  const title = isEdit ? `${t('edit')} ${t('pages.xray.Outbounds')}` : `+ ${t('pages.xray.Outbounds')}`;
  const okText = isEdit ? t('pages.clients.submitEdit') : t('create');

  const tag = (useWatch({ control: methods.control, name: 'tag' }) ?? '') as string;
  const protocol = (useWatch({ control: methods.control, name: 'protocol' }) ?? 'vless') as string;
  const network = (useWatch({ control: methods.control, name: 'streamSettings.network' }) ?? '') as string;
  const security = (useWatch({ control: methods.control, name: 'streamSettings.security' }) ?? 'none') as string;
  const flow = (useWatch({ control: methods.control, name: 'settings.flow' }) ?? '') as string;
  const reverseTag = useWatch({ control: methods.control, name: 'settings.reverseTag' });
  const wgSecretKey = useWatch({ control: methods.control, name: 'settings.secretKey' }) as string | undefined;

  const streamAllowed = canEnableStream({ protocol });
  const tlsAllowed = canEnableTls({ protocol, streamSettings: { network, security } });
  const realityAllowed = canEnableReality({ protocol, streamSettings: { network, security } });
  const tlsFlowAllowed = canEnableTlsFlow({ protocol, streamSettings: { network, security } });

  /*
   * Parse a share link (vmess:// / vless:// / trojan:// / ss:// /
   * hysteria2:// / wireguard://) and replace form state with the result.
   * The current tag is preserved when the parsed link doesn't carry one.
   */
  function importLink() {
    const link = linkInput.trim();
    if (!link) return;
    const parsed = parseOutboundLink(link);
    if (!parsed) {
      messageApi.error('Wrong Link!');
      return;
    }
    const currentTag = methods.getValues('tag');
    if (!parsed.tag && currentTag) parsed.tag = currentTag;
    const next = rawOutboundToFormValues(parsed);
    methods.reset(next);
    setFormSnapshot(JSON.stringify(formValuesToWirePayload(next)));
    setJsonText(JSON.stringify(formValuesToWirePayload(next), null, 2));
    setJsonDirty(false);
    setLinkInput('');
    messageApi.success('Link imported successfully');
    if (!jsonOnly) switchTab('1');
  }

  useEffect(() => {
    if (!open) return;
    const initial = outboundProp
      ? rawOutboundToFormValues(outboundProp)
      : buildAddModeValues();
    methods.reset(initial);
    setFormSnapshot(JSON.stringify(formValuesToWirePayload(initial)));
    setActiveKey(jsonOnly ? '2' : '1');
    setJsonText(JSON.stringify(outboundProp ?? formValuesToWirePayload(initial), null, 2));
    setLinkInput('');
    setJsonDirty(false);
    setShowInBasicRouting(quickRouting);
  }, [open, outboundProp, jsonOnly, methods, quickRouting]);

  useEffect(() => {
    if (!streamAllowed) return;
    /*
     * Wireguard dials its own UDP — only finalmask/sockopt apply, never a
     * transport. Don't seed network 'tcp'; clear a leftover one (from a
     * protocol switch) so the transmission/security blocks stay hidden.
     */
    if (protocol === 'wireguard') {
      if (network) methods.setValue('streamSettings', { security: 'none' } as StreamValue);
      return;
    }
    if (network) return;
    methods.setValue('streamSettings', { ...newStreamSlice('tcp'), security: 'none' } as StreamValue);
  }, [streamAllowed, network, protocol, methods]);

  useEffect(() => {
    if (protocol !== 'hysteria') return;
    if (network === 'hysteria' && security === 'tls') return;
    const existing = (methods.getValues('streamSettings') ?? {}) as Record<string, unknown>;
    const slice = hysteriaStreamSlice();
    if (existing.hysteriaSettings) slice.hysteriaSettings = existing.hysteriaSettings;
    if (existing.tlsSettings) slice.tlsSettings = existing.tlsSettings;
    methods.setValue('streamSettings', slice as StreamValue);
  }, [protocol, network, security, methods]);

  useEffect(() => {
    if (protocol !== 'wireguard') return;
    const sk = (wgSecretKey ?? '').trim();
    if (!sk) {
      methods.setValue('settings.pubKey', '');
      return;
    }
    try {
      const { publicKey } = Wireguard.generateKeypair(sk);
      methods.setValue('settings.pubKey', publicKey);
    } catch {
      methods.setValue('settings.pubKey', '');
    }
  }, [protocol, wgSecretKey, methods]);

  useEffect(() => {
    /* eslint-disable-next-line react-hooks/incompatible-library */
    const sub = methods.watch((_value, { name, type }) => {
      if (name !== 'protocol' || type !== 'change') return;
      const nextProtocol = methods.getValues('protocol');
      const next = rawOutboundToFormValues({ protocol: nextProtocol });
      methods.setValue('settings', next.settings);
      if (nextProtocol === 'hysteria') {
        methods.setValue('streamSettings', hysteriaStreamSlice() as StreamValue);
      } else if ((methods.getValues('streamSettings.network') ?? '') === 'hysteria') {
        methods.setValue('streamSettings', { ...newStreamSlice('tcp'), security: 'none' } as StreamValue);
      }
    });
    return () => sub.unsubscribe();
  }, [methods]);

  function onSecurityChange(next: string) {
    const stream = (methods.getValues('streamSettings') ?? {}) as Record<string, unknown>;
    const cleaned = { ...stream };
    delete cleaned.tlsSettings;
    delete cleaned.realitySettings;
    if (next === 'tls') {
      cleaned.tlsSettings = {
        serverName: '',
        alpn: [],
        fingerprint: '',
        echConfigList: '',
        verifyPeerCertByName: '',
        pinnedPeerCertSha256: '',
      };
    } else if (next === 'reality') {
      cleaned.realitySettings = {
        publicKey: '',
        fingerprint: 'chrome',
        serverName: '',
        shortId: '',
        spiderX: '',
        mldsa65Verify: '',
      };
    }
    cleaned.security = next;
    methods.setValue('streamSettings', cleaned as StreamValue);
  }

  /*
   * Network change cascade: swap the per-network sub-key (tcpSettings,
   * wsSettings, etc.) so the DU branch matches. Preserve security if
   * the new network supports it, otherwise force back to 'none'.
   */
  function onNetworkChange(next: string) {
    const stream = (methods.getValues('streamSettings') ?? {}) as Record<string, unknown>;
    methods.setValue('streamSettings', applyNetworkChange(protocol, stream, next) as StreamValue);
  }

  function onXmuxToggle(checked: boolean) {
    if (!checked) return;
    const existing = methods.getValues('streamSettings.xhttpSettings.xmux');
    const hasValues = existing && typeof existing === 'object' && Object.keys(existing).length > 0;
    if (hasValues) return;
    methods.setValue('streamSettings.xhttpSettings.xmux', { ...XMUX_FRESH_DEFAULTS });
  }

  const duplicateTag = useMemo(() => {
    const myTag = tag.trim();
    if (!myTag) return false;
    if (isEdit && (outboundProp?.tag as string | undefined) === myTag) return false;
    return (existingTags || []).includes(myTag);
  }, [tag, existingTags, isEdit, outboundProp]);

  /*
   * Bridge form <-> JSON tab: when leaving the JSON tab back to Basic, push
   * any edits into form state. When entering JSON tab, snapshot current
   * form values so the user sees the live shape.
   */
  function applyJsonToForm(): boolean {
    if (!jsonDirty) return true;
    const raw = jsonText.trim();
    if (!raw) return true;
    let parsed: Record<string, unknown>;
    try {
      parsed = JSON.parse(raw) as Record<string, unknown>;
    } catch (e) {
      messageApi.error(`JSON: ${(e as Error).message}`);
      return false;
    }
    if (!parsed || !PROTOCOL_OPTIONS.some(option => option.value === parsed.protocol)) {
      messageApi.error(i18n.language?.startsWith('fa')
        ? 'این پروتکل فقط از تب JSON قابل ویرایش است.'
        : 'This protocol can only be edited in the JSON tab.');
      return false;
    }
    const next = rawOutboundToFormValues(parsed);
    methods.reset(next);
    setFormSnapshot(JSON.stringify(formValuesToWirePayload(next)));
    setJsonDirty(false);
    return true;
  }

  function switchTab(key: string) {
    if (typeof document !== 'undefined') {
      (document.activeElement as HTMLElement | null)?.blur?.();
    }
    setActiveKey(key);
  }

  function onTabChange(key: string) {
    if (key === '2') {
      const payload = formValuesToWirePayload(methods.getValues());
      if (JSON.stringify(payload) !== formSnapshot) {
        setJsonText(JSON.stringify(payload, null, 2));
        setFormSnapshot(JSON.stringify(payload));
      }
      setJsonDirty(false);
      switchTab(key);
      return;
    }
    if (key === '1' && activeKey === '2') {
      if (!applyJsonToForm()) return;
    }
    switchTab(key);
  }

  async function onOk() {
    let payload: Record<string, unknown>;
    if (activeKey === '2') {
      try {
        const parsed: unknown = JSON.parse(jsonText);
        if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
          throw new Error(t('pages.xray.importInvalidJson'));
        }
        payload = parsed as Record<string, unknown>;
        if (typeof payload.protocol !== 'string' || !payload.protocol.trim()) {
          messageApi.error(i18n.language?.startsWith('fa') ? 'پروتکل را در JSON وارد کن.' : 'Enter an outbound protocol in the JSON.');
          return;
        }
        payload.protocol = payload.protocol.trim();
      } catch (e) {
        messageApi.error(`JSON: ${(e as Error).message}`);
        return;
      }
    } else {
      if (!(await methods.trigger())) return;
      payload = formValuesToWirePayload(methods.getValues());
      if (outboundProp && JSON.stringify(payload) === JSON.stringify(formValuesToWirePayload(rawOutboundToFormValues(outboundProp)))) {
        payload = outboundProp;
      }
    }
    const tagValue = typeof payload.tag === 'string' ? payload.tag.trim() : '';
    if (!tagValue) {
      messageApi.error(t('pages.xray.outboundForm.tagRequired'));
      return;
    }
    if (tagValue.startsWith('_bl_')) {
      messageApi.error(t('pages.xray.balancer.reservedPrefix'));
      return;
    }
    const isDuplicateTag = (existingTags || []).includes(tagValue)
      && !(isEdit && (outboundProp?.tag as string | undefined) === tagValue);
    if (isDuplicateTag) {
      messageApi.error('Tag already used by another outbound');
      return;
    }
    onConfirm({ ...payload, tag: tagValue }, showInBasicRouting && !quickRoutingReservedTags.includes(tagValue));
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        className="outbound-config-modal"
        title={title}
        okText={okText}
        cancelText={t('close')}
        mask={{ closable: false }}
        width={780}
        onOk={onOk}
        onCancel={onClose}
        destroyOnHidden
      >
        {(!quickRoutingReservedTags.includes(tag.trim()) || activeKey === '2') && (
          <Checkbox checked={showInBasicRouting} onChange={e => setShowInBasicRouting(e.target.checked)}>
            {fa ? 'نمایش در مسیریابی ساده' : 'Show in Basic Routing'}
          </Checkbox>
        )}
        <FormProvider {...methods}>
          <Form
            colon={false}
            labelCol={{ md: { span: 8 } }}
            wrapperCol={{ md: { span: 14 } }}
            labelWrap
          >
            <Tabs
              activeKey={activeKey}
              onChange={onTabChange}
              items={[
                {
                  key: '1',
                  disabled: jsonOnly,
                  label: t('pages.xray.basicTemplate'),
                  children: (
                    <>
                      <FormField
                        label={t('protocol')}
                        name="protocol"
                        rules={{ validate: rhfZodValidate(OutboundFormBaseSchema.shape.tag) }}
                      >
                        <Select id="protocol" options={PROTOCOL_OPTIONS} />
                      </FormField>

                      <Controller
                        control={methods.control}
                        name="tag"
                        rules={{ required: 'pages.xray.outboundForm.tagRequired' }}
                        render={({ field, fieldState }) => {
                          const errorMessage = fieldState.error?.message
                            ? t(fieldState.error.message, { defaultValue: fieldState.error.message })
                            : '';
                          return (
                            <Form.Item
                              label={t('pages.xray.outbound.tag')}
                              required
                              validateStatus={errorMessage ? 'error' : duplicateTag ? 'warning' : undefined}
                              help={errorMessage || (duplicateTag ? t('pages.xray.outboundForm.tagDuplicate') : undefined)}
                            >
                              <Input
                                value={field.value}
                                onChange={(e) => field.onChange(e.target.value)}
                                onBlur={field.onBlur}
                                ref={field.ref}
                                placeholder={t('pages.xray.outboundForm.tagPlaceholder')}
                              />
                            </Form.Item>
                          );
                        }}
                      />

                      <FormField label={t('pages.xray.outbound.sendThrough')} name="sendThrough">
                        <Input placeholder={t('pages.xray.outboundForm.localIpPlaceholder')} />
                      </FormField>

                      <FormField
                        label={t('pages.xray.outbound.targetStrategy')}
                        name="targetStrategy"
                        tooltip={t('pages.xray.outboundForm.targetStrategyHint')}
                      >
                        <Select allowClear placeholder="AsIs" options={TARGET_STRATEGY_OPTIONS} />
                      </FormField>

                      {SERVER_PROTOCOLS.has(protocol) && <ServerTarget />}
                      {protocol === 'vmess' && <VmessFields />}
                      {protocol === 'vless' && <VlessFields />}
                      {protocol === 'trojan' && <TrojanFields />}
                      {protocol === 'shadowsocks' && <ShadowsocksFields />}
                      {protocol === 'http' && <HttpFields />}
                      {protocol === 'socks' && <SocksFields />}

                      {protocol === 'loopback' && <LoopbackFields />}
                      {protocol === 'blackhole' && <BlackholeFields />}
                      {protocol === 'dns' && <DnsFields />}

                      {protocol === 'freedom' && <FreedomFields />}

                      {protocol === 'vless' && reverseTag && (
                        <Controller
                          control={methods.control}
                          name="settings.reverseSniffing"
                          render={({ field }) => (
                            <SniffingField
                              value={field.value}
                              onChange={field.onChange}
                              enableLabel={t('pages.xray.outboundForm.reverseSniffing')}
                            />
                          )}
                        />
                      )}

                      {protocol === 'wireguard' && <WireguardFields />}

                      {streamAllowed && network && (
                        <>
                          <Form.Item label={t('transmission')}>
                            <Select
                              value={network}
                              onChange={onNetworkChange}
                              options={
                                protocol === 'hysteria'
                                  ? [HYSTERIA_NETWORK_OPTION]
                                  : NETWORK_OPTIONS
                              }
                            />
                          </Form.Item>

                          {network === 'tcp' && <RawForm />}

                          {network === 'kcp' && <KcpForm />}

                          {network === 'ws' && <WsForm />}

                          {network === 'grpc' && <GrpcForm />}

                          {network === 'httpupgrade' && <HttpUpgradeForm />}

                          {network === 'xhttp' && <XhttpForm onXmuxToggle={onXmuxToggle} />}

                          {network === 'hysteria' && <HysteriaForm />}
                        </>
                      )}

                      {tlsFlowAllowed && (
                        <FormField label={t('pages.clients.flow')} name={['settings', 'flow']}>
                          <Select
                            allowClear
                            placeholder={t('none')}
                            options={[{ value: '', label: t('none') }, ...FLOW_OPTIONS]}
                          />
                        </FormField>
                      )}

                      {/* Vision seed knobs only meaningful for the exact
                          xtls-rprx-vision flow, on TCP+(tls|reality). */}
                      {tlsFlowAllowed && flow === 'xtls-rprx-vision' && (
                        <>
                          <FormField label={t('pages.xray.outboundForm.visionTestpre')} name={['settings', 'testpre']}>
                            <InputNumber min={0} style={{ width: '100%' }} />
                          </FormField>
                          <Form.Item label={t('pages.inbounds.form.visionTestseed')}>
                            <Space.Compact block>
                              {[0, 1, 2, 3].map((i) => (
                                <FormField key={i} name={['settings', 'testseed', i]} noStyle>
                                  <InputNumber min={1} style={{ width: '25%' }} />
                                </FormField>
                              ))}
                            </Space.Compact>
                          </Form.Item>
                        </>
                      )}

                      {streamAllowed && network && (
                        <Form.Item label={t('security')}>
                          <Radio.Group
                            value={security}
                            buttonStyle="solid"
                            onChange={(e) => onSecurityChange(e.target.value as string)}
                          >
                            {network !== 'hysteria' && <Radio.Button value="none">{t('none')}</Radio.Button>}
                            {tlsAllowed && <Radio.Button value="tls">TLS</Radio.Button>}
                            {realityAllowed && <Radio.Button value="reality">Reality</Radio.Button>}
                          </Radio.Group>
                        </Form.Item>
                      )}

                      {security === 'tls' && tlsAllowed && <TlsForm />}

                      {security === 'reality' && realityAllowed && <RealityForm />}

                      {((streamAllowed && network) || !streamAllowed || protocol === 'wireguard') && (
                        <SockoptForm outboundTags={dialerProxyTags ?? existingTags} />
                      )}

                      <Controller
                        control={methods.control}
                        name="streamSettings.finalmask"
                        render={({ field }) => (
                          <FinalMaskField
                            key={`${protocol}:${network}`}
                            value={field.value}
                            onChange={field.onChange}
                            network={network}
                            protocol={protocol}
                          />
                        )}
                      />

                      <MuxForm protocol={protocol} network={network} />
                    </>
                  ),
                },
                {
                  key: '2',
                  label: 'JSON',
                  children: (
                    <Space orientation="vertical" size={10} style={{ width: '100%', marginTop: 10 }}>
                      <Input.Search
                        value={linkInput}
                        placeholder="vmess:// vless:// trojan:// ss:// hysteria2:// wireguard://"
                        enterButton="Import"
                        onChange={(e) => setLinkInput(e.target.value)}
                        onSearch={importLink}
                      />
                      <JsonEditor
                        value={jsonText}
                        onChange={(next) => {
                          setJsonText(next);
                          setJsonDirty(true);
                        }}
                        minHeight="360px"
                        maxHeight="600px"
                      />
                    </Space>
                  ),
                },
              ]}
            />
          </Form>
        </FormProvider>
      </Modal>
    </>
  );
}
