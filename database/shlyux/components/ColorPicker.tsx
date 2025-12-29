import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Check, X } from 'lucide-react';
import { usePresence } from '../utils/usePresence';

const RECENT_COLORS_KEY = 'w12c-recent-colors-v1';

const COLOR_ROWS: string[][] = [
  ['#000000', '#434343', '#666666', '#999999', '#B7B7B7', '#CCCCCC', '#D9D9D9', '#EFEFEF', '#F3F3F3', '#FFFFFF'],
  ['#980000', '#FF0000', '#FF9900', '#FFFF00', '#00FF00', '#00FFFF', '#4A86E8', '#0000FF', '#9900FF', '#FF00FF'],
  ['#E6B8AF', '#F4CCCC', '#FCE5CD', '#FFF2CC', '#D9EAD3', '#D0E0E3', '#C9DAF8', '#CFE2F3', '#D9D2E9', '#EAD1DC'],
  ['#DD7E6B', '#EA9999', '#F9CB9C', '#FFE599', '#B6D7A8', '#A2C4C9', '#A4C2F4', '#9FC5E8', '#B4A7D6', '#D5A6BD'],
  ['#CC4125', '#E06666', '#F6B26B', '#FFD966', '#93C47D', '#76A5AF', '#6D9EEB', '#6FA8DC', '#8E7CC3', '#C27BA0'],
  ['#A61C00', '#CC0000', '#E69138', '#F1C232', '#6AA84F', '#45818E', '#3C78D8', '#3D85C6', '#674EA7', '#A64D79'],
  ['#85200C', '#990000', '#B45F06', '#BF9000', '#38761D', '#134F5C', '#1155CC', '#0B5394', '#351C75', '#741B47'],
];

const normalizeHex = (input: string): string | null => {
  const raw = input.trim().replace(/^#/, '');
  if (raw.length === 0) return null;
  if (raw.length === 3 && /^[0-9a-fA-F]{3}$/.test(raw)) {
    const expanded = raw.split('').map((ch) => ch + ch).join('');
    return `#${expanded.toUpperCase()}`;
  }
  if (raw.length === 6 && /^[0-9a-fA-F]{6}$/.test(raw)) {
    return `#${raw.toUpperCase()}`;
  }
  return null;
};

const readRecentColors = (): string[] => {
  if (typeof window === 'undefined') return [];
  try {
    const raw = localStorage.getItem(RECENT_COLORS_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    return parsed.filter((c): c is string => typeof c === 'string' && /^#[0-9A-Fa-f]{6}$/.test(c)).slice(0, 8);
  } catch {
    return [];
  }
};

const pushRecentColor = (color: string) => {
  if (typeof window === 'undefined') return;
  try {
    const next = [color.toUpperCase(), ...readRecentColors().filter((c) => c.toUpperCase() !== color.toUpperCase())].slice(0, 8);
    localStorage.setItem(RECENT_COLORS_KEY, JSON.stringify(next));
  } catch {
    // ignore
  }
};

export default function ColorPicker({
  title,
  value,
  defaultLabel,
  defaultSwatch,
  onChange,
}: {
  title: string;
  value?: string;
  defaultLabel: string;
  defaultSwatch: string;
  onChange: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const presence = usePresence(open, { exitDurationMs: 180 });
  const containerRef = useRef<HTMLDivElement | null>(null);

  const [recentColors, setRecentColors] = useState<string[]>(() => readRecentColors());
  const [hexInput, setHexInput] = useState('');

  const normalizedSelected = useMemo(() => normalizeHex(value || '') || '', [value]);

  useEffect(() => {
    if (!open) return;
    setRecentColors(readRecentColors());
    setHexInput(normalizedSelected || '');
  }, [open, normalizedSelected]);

  useEffect(() => {
    if (!open) return;
    const onMouseDown = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onMouseDown);
    window.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('mousedown', onMouseDown);
      window.removeEventListener('keydown', onKeyDown);
    };
  }, [open]);

  const applyColor = (next: string) => {
    onChange(next);
    if (next) pushRecentColor(next);
  };

  const pick = (next: string) => {
    applyColor(next);
    setOpen(false);
  };

  const applyHex = () => {
    const normalized = normalizeHex(hexInput);
    if (!normalized) return;
    pick(normalized);
  };

  const displaySwatch = normalizedSelected ? normalizedSelected : defaultSwatch;

  return (
    <div className="relative" ref={containerRef}>
      <button
        type="button"
        className="h-8 px-2 rounded border inline-flex items-center gap-1 hover:bg-gray-50 transition-colors"
        style={{ borderColor: 'var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
        onClick={() => setOpen((p) => !p)}
        title={title}
        aria-label={title}
      >
        <span
          className="inline-block w-4 h-4 rounded-sm"
          style={{
            background: displaySwatch,
            border: '1px solid var(--border-color)',
            boxShadow: normalizedSelected ? `0 0 0 2px var(--sheet-selection-fill-weak)` : 'none',
          }}
        />
        <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>
          ▾
        </span>
      </button>

      {presence.isMounted && (
        <>
          <div className="fixed inset-0 z-40 ui-overlay" data-state={presence.state} onClick={() => setOpen(false)} />
          <div
            className="absolute left-0 mt-2 z-50 w-72 rounded-lg shadow-xl ui-popover"
            data-state={presence.state}
            style={{
              background: 'var(--card-bg)',
              border: '1px solid var(--border-color)',
              boxShadow: 'var(--shadow-lg)',
              transformOrigin: 'top left',
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="px-3 py-2 flex items-center justify-between border-b" style={{ borderColor: 'var(--border-color)' }}>
              <div className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                {title}
              </div>
              <button
                type="button"
                onClick={() => setOpen(false)}
                className="p-1 rounded hover:opacity-80 transition-opacity"
                style={{ color: 'var(--text-secondary)' }}
                title="Yopish"
              >
                <X size={16} />
              </button>
            </div>

            <div className="p-3 space-y-3">
              <div className="flex items-center justify-between gap-2">
                <button
                  type="button"
                  className="inline-flex items-center gap-2 px-2 py-1.5 rounded border text-sm hover:opacity-80 transition-opacity"
                  style={{ borderColor: 'var(--border-color)', background: 'var(--bg-light)', color: 'var(--text-primary)' }}
                  onClick={() => pick('')}
                  title={defaultLabel}
                >
                  <span
                    className="inline-block w-4 h-4 rounded-sm"
                    style={{ background: defaultSwatch, border: '1px solid var(--border-color)' }}
                  />
                  <span>{defaultLabel}</span>
                </button>

                {normalizedSelected && (
                  <button
                    type="button"
                    className="inline-flex items-center gap-2 px-2 py-1.5 rounded border text-sm hover:opacity-80 transition-opacity"
                    style={{ borderColor: 'var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
                    onClick={() => pick('')}
                    title="Tozalash"
                  >
                    <X size={14} />
                    Tozalash
                  </button>
                )}
              </div>

              {recentColors.length > 0 && (
                <div>
                  <div className="text-xs font-semibold mb-2" style={{ color: 'var(--text-secondary)' }}>
                    Recent
                  </div>
                  <div className="grid grid-cols-8 gap-1">
                    {recentColors.map((c) => (
                      <button
                        key={c}
                        type="button"
                        className="w-7 h-7 rounded border hover:opacity-90 transition-opacity"
                        style={{
                          background: c,
                          borderColor: 'var(--border-color)',
                          boxShadow: normalizedSelected?.toUpperCase() === c.toUpperCase()
                            ? `0 0 0 2px var(--sheet-selection-border)`
                            : 'none',
                        }}
                        onClick={() => pick(c)}
                        title={c}
                        aria-label={c}
                      />
                    ))}
                  </div>
                </div>
              )}

              <div>
                <div className="text-xs font-semibold mb-2" style={{ color: 'var(--text-secondary)' }}>
                  Ranglar
                </div>
                <div className="space-y-1.5">
                  {COLOR_ROWS.map((row, idx) => (
                    <div key={idx} className="grid grid-cols-10 gap-1">
                      {row.map((c) => (
                        <button
                          key={c}
                          type="button"
                          className="w-7 h-7 rounded border hover:opacity-90 transition-opacity"
                          style={{
                            background: c,
                            borderColor: 'var(--border-color)',
                            boxShadow: normalizedSelected?.toUpperCase() === c.toUpperCase()
                              ? `0 0 0 2px var(--sheet-selection-border)`
                              : 'none',
                          }}
                          onClick={() => pick(c)}
                          title={c}
                          aria-label={c}
                        />
                      ))}
                    </div>
                  ))}
                </div>
              </div>

              <div className="rounded-lg p-2" style={{ background: 'var(--bg-light)', border: '1px solid var(--border-color)' }}>
                <div className="text-xs font-semibold mb-2" style={{ color: 'var(--text-secondary)' }}>
                  HEX
                </div>
                <div className="flex items-center gap-2">
                  <span
                    className="inline-block w-8 h-8 rounded border"
                    style={{ borderColor: 'var(--border-color)', background: normalizeHex(hexInput) || defaultSwatch }}
                    title="Preview"
                  />
                  <input
                    value={hexInput}
                    onChange={(e) => setHexInput(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') applyHex();
                    }}
                    placeholder="#RRGGBB"
                    className="flex-1 px-2 py-2 text-sm rounded border focus:outline-none"
                    style={{ borderColor: 'var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
                  />
                  <button
                    type="button"
                    onClick={applyHex}
                    disabled={!normalizeHex(hexInput)}
                    className="px-3 py-2 rounded text-sm text-white disabled:opacity-50 disabled:cursor-not-allowed hover:opacity-90 transition-opacity btn-lift"
                    style={{ background: 'var(--primary)' }}
                    title="Qo‘llash"
                  >
                    <Check size={16} />
                  </button>
                </div>
                <div className="mt-2 text-[11px]" style={{ color: 'var(--text-secondary)' }}>
                  Masalan: <span style={{ color: 'var(--text-primary)' }}>#2563EB</span>
                </div>
              </div>
            </div>
          </div>
        </>
      )}
    </div>
  );
}

