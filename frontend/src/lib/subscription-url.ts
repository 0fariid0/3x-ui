/**
 * Returns a plain subscription URL for clients that reject profile-name
 * query parameters. Existing non-name query parameters and fragments are
 * preserved, while a stale `name` parameter is removed.
 */
export function withEmailConfigNames(rawUrl: string, _email?: string): string {
  if (!rawUrl) return '';

  const hashIndex = rawUrl.indexOf('#');
  const beforeHash = hashIndex >= 0 ? rawUrl.slice(0, hashIndex) : rawUrl;
  const hash = hashIndex >= 0 ? rawUrl.slice(hashIndex) : '';
  const queryIndex = beforeHash.indexOf('?');

  if (queryIndex < 0) {
    return rawUrl;
  }

  const base = beforeHash.slice(0, queryIndex);
  const query = beforeHash.slice(queryIndex + 1);
  const params = new URLSearchParams(query);
  params.delete('name');

  const nextQuery = params.toString();
  return `${base}${nextQuery ? `?${nextQuery}` : ''}${hash}`;
}
