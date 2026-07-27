import { describe, expect, it } from 'vitest';
import { withEmailConfigNames } from '@/lib/subscription-url';

describe('withEmailConfigNames', () => {
  it('adds the actual client email as the name parameter', () => {
    expect(withEmailConfigNames('https://sub.example.com/sub/ABC', 'client-123'))
      .toBe('https://sub.example.com/sub/ABC?name=client-123');
  });

  it('preserves existing params and fragments', () => {
    expect(withEmailConfigNames('https://example.com/sub/ABC?view=raw#top', 'user@example.com'))
      .toBe('https://example.com/sub/ABC?view=raw&name=user%40example.com#top');
  });

  it('replaces an existing name parameter', () => {
    expect(withEmailConfigNames('/sub/ABC?name=host', 'new-name'))
      .toBe('/sub/ABC?name=new-name');
  });

  it('keeps legacy email mode when no client name is available', () => {
    expect(withEmailConfigNames('/sub/ABC')).toBe('/sub/ABC?name=email');
  });
});
