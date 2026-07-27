import { fireEvent, render, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import QrPanel from '@/pages/inbounds/qr/QrPanel';

const { copyText } = vi.hoisted(() => ({
  copyText: vi.fn<(_: string) => Promise<boolean>>(),
}));

vi.mock('@/utils', () => ({
  ClipboardManager: { copyText },
  FileManager: { downloadTextFile: vi.fn() },
}));

describe('QrPanel', () => {
  beforeEach(() => {
    copyText.mockReset();
    copyText.mockResolvedValue(true);
  });

  it('copies the represented link when the QR image is clicked', async () => {
    const value = 'https://sub.example.com/sub/ABC?name=email';
    const { container } = render(<QrPanel value={value} remark="client@example.com" size={180} />);
    const canvas = container.querySelector('.qr-panel-canvas');
    expect(canvas).not.toBeNull();

    fireEvent.click(canvas!);

    await waitFor(() => expect(copyText).toHaveBeenCalledWith(value));
  });
});
