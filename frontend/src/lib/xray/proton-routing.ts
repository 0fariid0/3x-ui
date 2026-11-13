import type { XraySettingsValue } from '@/hooks/useXraySetting';
import type { RuleObject } from '@/schemas/routing';
import { propagateOutboundTagRename } from '@/pages/xray/basics/helpers';
import type { buildWireguardConf } from './proton-outbound';

export const PROTON_OUTBOUND_TAG = 'proton-free';
type Template = XraySettingsValue | null;

export function getProtonCandidates(template: Template): string[] {
  const rules = template?.routing?.rules ?? [];
  const outbounds = template?.outbounds ?? [];
  if (outbounds.some(outbound => outbound.tag === PROTON_OUTBOUND_TAG)) {
    const matches = outbounds.filter(outbound => outbound.tag === PROTON_OUTBOUND_TAG);
    return matches.length === 1 && matches[0].protocol === 'wireguard' ? [PROTON_OUTBOUND_TAG] : [];
  }
  return outbounds.flatMap(outbound => {
    const tag = outbound.tag;
    return outbound.protocol === 'wireguard' && tag && !/^(nord|warp)/i.test(tag)
      && (/^proton/i.test(tag) || rules.some(rule => rule.outboundTag === tag && rule.ruleTag?.startsWith('xui-proton-')))
      && outbounds.filter(item => item.tag === tag).length === 1
      ? [tag] : [];
  });
}

export function getProtonOutboundTag(template: Template): string | undefined {
  const candidates = getProtonCandidates(template);
  return candidates.length === 1 ? candidates[0] : undefined;
}

export function canImportProton(template: Template, reserved: readonly string[], legacyTag?: string): boolean {
  const outbounds = template?.outbounds ?? [];
  const candidates = getProtonCandidates(template);
  return outbounds.length > 0 && !reserved.includes(PROTON_OUTBOUND_TAG)
    && (!outbounds.some(outbound => outbound.tag === PROTON_OUTBOUND_TAG) || candidates[0] === PROTON_OUTBOUND_TAG)
    && (candidates.length < 2 || (legacyTag !== undefined && candidates.includes(legacyTag)));
}

export function saveProtonOutbound(
  template: XraySettingsValue, outbound: ReturnType<typeof buildWireguardConf>,
  reserved: readonly string[], legacyTag?: string,
): boolean {
  if (!canImportProton(template, reserved, legacyTag) || !template.outbounds) return false;
  const oldTag = getProtonOutboundTag(template) ?? legacyTag;
  const index = template.outbounds.findIndex(item => item.tag === oldTag && oldTag !== undefined);
  if (index < 0) template.outbounds.push(outbound);
  else {
    template.outbounds[index] = { ...template.outbounds[index], ...outbound };
    if (oldTag !== undefined && oldTag !== PROTON_OUTBOUND_TAG) {
      propagateOutboundTagRename(template, oldTag, PROTON_OUTBOUND_TAG);
      for (const rule of template.routing?.rules ?? []) {
        for (const kind of ['domain', 'ip'] as const) {
          if (rule.outboundTag === PROTON_OUTBOUND_TAG && rule.ruleTag === protonRuleTag(oldTag, kind)) {
            rule.ruleTag = protonRuleTag(PROTON_OUTBOUND_TAG, kind);
          }
        }
      }
    }
  }
  return true;
}

export const protonRuleTag = (tag: string, kind: 'domain' | 'ip') => `xui-proton-${kind}:${tag}`;

export function setProtonRules(template: XraySettingsValue, tag: string, domains: string[], ips?: string[]) {
  if (!template.outbounds?.some(outbound => outbound.tag === tag && outbound.protocol === 'wireguard')) return;
  template.routing ??= { rules: [] };
  const rules = [...(template.routing.rules ?? [])];
  for (const [kind, values] of [['domain', domains], ['ip', ips]] as const) {
    if (values === undefined) continue;
    const marker = protonRuleTag(tag, kind);
    const index = rules.findIndex(rule => rule.ruleTag?.startsWith(`xui-proton-${kind}:`) && rule.outboundTag === tag);
    const tokens = [...new Set(values)];
    if (!tokens.length) {
      if (index >= 0) rules.splice(index, 1);
      continue;
    }
    const rule: RuleObject = { type: 'field', ruleTag: marker, outboundTag: tag, [kind]: tokens };
    if (index >= 0) rules[index] = { ...rules[index], [kind]: tokens };
    else rules.push(rule);
  }
  template.routing.rules = rules;
}
