import { describe, expect, it } from 'vitest';
import { withEmailConfigNames } from '@/lib/subscription-url';

describe('withEmailConfigNames', () => {
  it('keeps a plain subscription URL unchanged', () => {
    expect(withEmailConfigNames('https://sub.example.com/sub/ABC', 'client-123'))
      .toBe('https://sub.example.com/sub/ABC');
  });

  it('preserves non-name params and fragments', () => {
    expect(withEmailConfigNames('https://example.com/sub/ABC?view=raw&name=user%40example.com#top', 'user@example.com'))
      .toBe('https://example.com/sub/ABC?view=raw#top');
  });

  it('removes an existing name parameter', () => {
    expect(withEmailConfigNames('/sub/ABC?name=host', 'new-name'))
      .toBe('/sub/ABC');
  });

  it('does not add legacy email mode when no client name is available', () => {
    expect(withEmailConfigNames('/sub/ABC')).toBe('/sub/ABC');
  });
});
