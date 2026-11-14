import type { XraySettingsValue } from '@/hooks/useXraySetting';
import type { RuleObject } from '@/schemas/routing';

type Match = 'domain' | 'ip';
const marker = (kind: Match) => `xui-custom-outbound-${kind}:`;

// Match by destination separately from the marker so a standard outbound
// rename can update references without losing the quick editor's rules.
function isManagedRule(rule: RuleObject, outboundTag: string, kind: Match) {
  return rule.outboundTag === outboundTag && rule.ruleTag?.startsWith(marker(kind));
}

export function customRouteValues(template: XraySettingsValue | null, outboundTag: string, kind: Match): string[] {
  return (template?.routing?.rules ?? []).find(rule => isManagedRule(rule, outboundTag, kind))?.[kind] ?? [];
}

export function setCustomRoute(template: XraySettingsValue, outboundTag: string, kind: Match, values: string[]) {
  if (!outboundTag || !template.outbounds?.some(outbound => outbound.tag === outboundTag)) return;
  template.routing ??= {};
  const rules = [...(template.routing.rules ?? [])];
  const index = rules.findIndex(rule => isManagedRule(rule, outboundTag, kind));
  const terms = [...new Set(values.map(value => value.trim()).filter(Boolean))];
  if (index >= 0) {
    // Retain position, enabled state and any criteria from the rule editor.
    if (terms.length) rules[index] = { ...rules[index], [kind]: terms };
    else rules.splice(index, 1);
  } else if (terms.length) {
    // Xray uses the first matching rule. Do not overtake existing blocks,
    // direct exceptions, or any other custom rule the admin already saved.
    rules.push({ type: 'field', outboundTag, ruleTag: marker(kind) + outboundTag, [kind]: terms });
  }
  template.routing.rules = rules;
}

export function renameCustomRouteMarker(rule: RuleObject, outboundTag: string) {
  for (const kind of ['domain', 'ip'] as const) {
    if (rule.ruleTag?.startsWith(marker(kind))) rule.ruleTag = marker(kind) + outboundTag;
  }
}
