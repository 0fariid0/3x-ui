export function normalizeTranslationPlaceholders(value: unknown): unknown {
  if (typeof value === 'string') return value.replace(/\{\{([a-z][a-zA-Z0-9_]*)\}\}/g, '{$1}');
  if (Array.isArray(value)) return value.map(normalizeTranslationPlaceholders);
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, normalizeTranslationPlaceholders(item)]));
  }
  return value;
}
