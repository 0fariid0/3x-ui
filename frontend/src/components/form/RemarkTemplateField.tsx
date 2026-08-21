import { forwardRef, useImperativeHandle, useRef } from 'react';
import { Button, Input, Popover, Select, Tooltip } from 'antd';
import type { InputRef } from 'antd';
import { CodeOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';

import { REMARK_TEMPLATE_PRESETS, hasRemarkTokens, previewRemark, wrapToken } from '@/lib/remark/remarkVariables';
import RemarkVarPicker from './RemarkVarPicker';

interface RemarkTemplateFieldProps {
  // Injected by antd Form.Item:
  value?: string;
  onChange?: (value: string) => void;
  maxLength?: number;
  placeholder?: string;
  showPresets?: boolean;
  excludeTokens?: string[];
}

/**
 * RemarkTemplateField is a text input augmented with a {{VAR}} template picker
 * (insert-at-caret) and a live, sample-based preview of the expanded result.
 * Used for both the global subscription template and per-Host config names.
 */
const RemarkTemplateField = forwardRef<InputRef, RemarkTemplateFieldProps>(function RemarkTemplateField({
  value = '',
  onChange,
  maxLength,
  placeholder,
  showPresets = true,
  excludeTokens = [],
}, forwardedRef) {
  const { t } = useTranslation();
  const inputRef = useRef<InputRef>(null);
  useImperativeHandle(forwardedRef, () => inputRef.current as InputRef);

  function insertToken(token: string) {
    const el = inputRef.current?.input;
    const start = el?.selectionStart ?? value.length;
    const end = el?.selectionEnd ?? value.length;
    const insert = wrapToken(token);
    const next = value.slice(0, start) + insert + value.slice(end);
    onChange?.(maxLength ? next.slice(0, maxLength) : next);
    const caret = start + insert.length;
    // The controlled value updates next render; restore the caret after it.
    requestAnimationFrame(() => {
      el?.focus();
      el?.setSelectionRange(caret, caret);
    });
  }

  return (
    <div>
      <Input
        ref={inputRef}
        value={value}
        maxLength={maxLength}
        placeholder={placeholder}
        onChange={(e) => onChange?.(e.target.value)}
        suffix={
          <Popover
            content={<RemarkVarPicker onPick={insertToken} excludeTokens={excludeTokens} />}
            trigger="click"
            placement="bottomRight"
            title={t('pages.hosts.remarkVars.title')}
          >
            <Tooltip title={t('pages.hosts.remarkVars.title')}>
              <Button type="text" size="small" icon={<CodeOutlined />} aria-label={t('pages.hosts.remarkVars.title')} style={{ marginInlineEnd: -7 }} />
            </Tooltip>
          </Popover>
        }
      />
      {showPresets && (
        <Select
          size="small"
          value={undefined}
          placeholder={t('pages.hosts.remarkVars.quickTemplates')}
          options={REMARK_TEMPLATE_PRESETS.map((template) => ({ value: template, label: template }))}
          onChange={(template: string) => {
            onChange?.(maxLength ? template.slice(0, maxLength) : template);
          }}
          style={{ width: '100%', marginTop: 6, fontFamily: 'monospace' }}
          popupMatchSelectWidth={false}
        />
      )}
      {hasRemarkTokens(value) && (
        <div style={{ fontSize: 12, marginTop: 4, opacity: 0.7 }}>
          {t('pages.hosts.remarkVars.preview')}:{' '}
          <span style={{ fontFamily: 'monospace' }}>{previewRemark(value) || '—'}</span>
        </div>
      )}
    </div>
  );
});

export default RemarkTemplateField;
