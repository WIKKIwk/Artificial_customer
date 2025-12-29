import React, { useEffect, useRef, useState } from 'react';
import FunctionPicker from './FunctionPicker';

interface FormulaBarProps {
  activeCellLabel: string;
  value: string;
  onChange: (val: string) => void;
  onSubmit: () => void;
  onGoToCell?: (label: string) => void;
  readOnly?: boolean;
  density?: 'comfortable' | 'compact';
}

const NAME_BOX_ID = 'sheetmaster-name-box';

const FormulaBar: React.FC<FormulaBarProps> = ({ activeCellLabel, value, onChange, onSubmit, onGoToCell, readOnly, density }) => {
  const inputRef = useRef<HTMLInputElement>(null);
  const nameBoxRef = useRef<HTMLInputElement>(null);
  const [nameBoxValue, setNameBoxValue] = useState(activeCellLabel);
  const isCompact = density === 'compact';
  const barHeight = isCompact ? 36 : 40;
  const controlHeight = isCompact ? 28 : 32;
  const nameBoxHeight = isCompact ? 22 : 24;

  useEffect(() => {
    setNameBoxValue(activeCellLabel);
  }, [activeCellLabel]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      onSubmit();
      inputRef.current?.blur();
    }
  };

  const handleInsertFunction = (snippet: string) => {
    const input = inputRef.current;
    const selectionStart = input?.selectionStart ?? value.length;
    const selectionEnd = input?.selectionEnd ?? value.length;
    const needsEquals = !value.trim().startsWith('=') && selectionStart === 0;
    const cleanSnippet = snippet.startsWith('=') ? snippet : `${needsEquals ? '=' : ''}${snippet}`;
    const nextValue = value.slice(0, selectionStart) + cleanSnippet + value.slice(selectionEnd);
    onChange(nextValue);

    const cursorAdjustment = snippet.endsWith('()') ? 1 : 0;
    requestAnimationFrame(() => {
      if (!inputRef.current) return;
      inputRef.current.focus();
      const cursorPos = selectionStart + cleanSnippet.length - cursorAdjustment;
      inputRef.current.setSelectionRange(cursorPos, cursorPos);
    });
  };

  return (
    <div
      className="flex items-center px-2 space-x-2 z-10"
      style={{ height: barHeight, borderBottom: '1px solid var(--border-color)', background: 'var(--card-bg)' }}
    >
      <input
        id={NAME_BOX_ID}
        ref={nameBoxRef}
        className="w-16 text-center text-sm font-medium rounded focus:outline-none"
        style={{
          height: nameBoxHeight,
          lineHeight: `${nameBoxHeight}px`,
          background: 'var(--card-bg)',
          border: '1px solid var(--border-color)',
          color: 'var(--text-primary)',
        }}
        value={nameBoxValue}
        onChange={(e) => setNameBoxValue(e.target.value)}
        onFocus={() => setNameBoxValue(activeCellLabel)}
        onBlur={() => setNameBoxValue(activeCellLabel)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault();
            onGoToCell?.(nameBoxValue);
            nameBoxRef.current?.blur();
          } else if (e.key === 'Escape') {
            e.preventDefault();
            setNameBoxValue(activeCellLabel);
            nameBoxRef.current?.blur();
          }
        }}
        placeholder="A1"
        title="Katakka o'tish (Ctrl+G)"
        disabled={!onGoToCell}
      />
      {!readOnly && <FunctionPicker onInsert={handleInsertFunction} />}
      <div className="flex-1 relative" style={{ height: controlHeight }}>
        <input
          ref={inputRef}
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={handleKeyDown}
          className="w-full h-full rounded px-2 text-sm focus:outline-none"
          style={{
            border: '1px solid var(--border-color)',
            background: 'var(--card-bg)',
            color: 'var(--text-primary)'
          }}
          placeholder="Enter value or formula (e.g., =SUM(A1:B5))"
          disabled={!!readOnly}
        />
      </div>
    </div>
  );
};

export default FormulaBar;
export { NAME_BOX_ID };
