import type { XraySettingsValue } from '@/hooks/useXraySetting';
import type { RuleObject } from '@/schemas/routing';

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
