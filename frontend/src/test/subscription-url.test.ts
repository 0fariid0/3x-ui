import { describe, expect, it } from 'vitest';

import { withEmailConfigNames } from '@/lib/subscription-url';

describe('withEmailConfigNames', () => {
  it('appends email naming to a plain subscription URL', () => {
    expect(withEmailConfigNames('https://sub.example.com/sub/ABC'))
      .toBe('https://sub.example.com/sub/ABC?name=email');
  });

  it('preserves existing query parameters and fragments', () => {
    expect(withEmailConfigNames('https://example.com/sub/ABC?view=raw#top'))
      .toBe('https://example.com/sub/ABC?view=raw&name=email#top');
  });

  it('replaces an existing name mode', () => {
    expect(withEmailConfigNames('/sub/ABC?name=host'))
      .toBe('/sub/ABC?name=email');
  });
});
