/**
 * Adds the panel's explicit email naming mode to a subscription URL.
 * Existing query parameters and fragments are preserved, and an existing
 * `name` parameter is replaced instead of duplicated.
 */
export function withEmailConfigNames(rawUrl: string): string {
  if (!rawUrl) return '';

  const hashIndex = rawUrl.indexOf('#');
  const beforeHash = hashIndex >= 0 ? rawUrl.slice(0, hashIndex) : rawUrl;
  const hash = hashIndex >= 0 ? rawUrl.slice(hashIndex) : '';
  const queryIndex = beforeHash.indexOf('?');
  const base = queryIndex >= 0 ? beforeHash.slice(0, queryIndex) : beforeHash;
  const query = queryIndex >= 0 ? beforeHash.slice(queryIndex + 1) : '';
  const params = new URLSearchParams(query);
  params.set('name', 'email');
  return `${base}?${params.toString()}${hash}`;
}
