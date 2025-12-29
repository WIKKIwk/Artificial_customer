import React, { useEffect, useRef } from 'react';
import { ArrowDown, ArrowUp, CaseSensitive, Replace, X } from 'lucide-react';
import { usePresence } from '../utils/usePresence';

type FindScope = 'sheet' | 'selection';
type FindMode = 'find' | 'replace';

interface FindReplaceModalProps {
  isOpen: boolean;
  mode: FindMode;
  query: string;
  replaceText: string;
  matchCase: boolean;
  wholeCell: boolean;
  scope: FindScope;
  matchCount: number;
  currentIndex: number; // 0-based, -1 when none
  onChangeMode: (mode: FindMode) => void;
  onChangeQuery: (query: string) => void;
  onChangeReplaceText: (text: string) => void;
  onChangeMatchCase: (value: boolean) => void;
  onChangeWholeCell: (value: boolean) => void;
  onChangeScope: (scope: FindScope) => void;
  onFindNext: () => void;
  onFindPrev: () => void;
  onReplace: () => void;
  onReplaceAll: () => void;
  onClose: () => void;
}

const FindReplaceModal: React.FC<FindReplaceModalProps> = ({
  isOpen,
  mode,
  query,
  replaceText,
  matchCase,
  wholeCell,
  scope,
  matchCount,
  currentIndex,
  onChangeMode,
  onChangeQuery,
  onChangeReplaceText,
  onChangeMatchCase,
  onChangeWholeCell,
  onChangeScope,
  onFindNext,
  onFindPrev,
  onReplace,
  onReplaceAll,
  onClose
}) => {
  const findInputRef = useRef<HTMLInputElement | null>(null);
  const replaceInputRef = useRef<HTMLInputElement | null>(null);
  const modalPresence = usePresence(isOpen, { exitDurationMs: 180 });

  useEffect(() => {
    if (!isOpen) return;
    const t = window.setTimeout(() => {
      if (mode === 'replace') {
        (replaceInputRef.current || findInputRef.current)?.focus();
      } else {
        findInputRef.current?.focus();
      }
    }, 0);
    return () => window.clearTimeout(t);
  }, [isOpen, mode]);

  useEffect(() => {
    if (!isOpen) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [isOpen, onClose]);

  if (!modalPresence.isMounted) return null;

  const canNavigate = query.trim().length > 0 && matchCount > 0;
  const matchLabel = query.trim().length === 0
    ? 'Qidirish uchun matn kiriting'
    : matchCount === 0
      ? 'Topilmadi'
      : `${Math.min(matchCount, currentIndex + 1)}/${matchCount}`;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center ui-overlay"
      data-state={modalPresence.state}
      style={{ background: 'rgba(0,0,0,0.45)' }}
    >
      <div
        className="w-full max-w-xl rounded-lg shadow-xl overflow-hidden ui-modal"
        data-state={modalPresence.state}
        style={{ background: 'var(--card-bg)', border: '1px solid var(--border-color)' }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-5 py-4 flex items-center justify-between border-b" style={{ borderColor: 'var(--border-color)' }}>
          <div className="flex items-center gap-2">
            <div className="inline-flex rounded-md overflow-hidden" style={{ border: '1px solid var(--border-color)' }}>
              <button
                type="button"
                onClick={() => onChangeMode('find')}
                className="px-3 py-1.5 text-sm"
                style={{
                  background: mode === 'find' ? 'var(--bg-light)' : 'transparent',
                  color: 'var(--text-primary)'
                }}
              >
                Qidirish
              </button>
              <button
                type="button"
                onClick={() => onChangeMode('replace')}
                className="px-3 py-1.5 text-sm"
                style={{
                  background: mode === 'replace' ? 'var(--bg-light)' : 'transparent',
                  color: 'var(--text-primary)'
                }}
              >
                Almashtirish
              </button>
            </div>
            <span className="text-xs px-2 py-1 rounded" style={{ background: 'var(--bg-light)', color: 'var(--text-secondary)' }}>
              {matchLabel}
            </span>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-2 rounded hover:opacity-80 transition-opacity"
            style={{ color: 'var(--text-secondary)' }}
            title="Yopish"
          >
            <X size={18} />
          </button>
        </div>

        <div className="p-5 space-y-4">
          <div className="grid grid-cols-1 gap-3">
            <div>
              <label className="block text-xs font-medium mb-1" style={{ color: 'var(--text-secondary)' }}>
                Qidirish
              </label>
              <input
                ref={findInputRef}
                value={query}
                onChange={(e) => onChangeQuery(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    e.preventDefault();
                    if (e.shiftKey) onFindPrev();
                    else onFindNext();
                  }
                }}
                className="w-full px-3 py-2 rounded text-sm focus:outline-none"
                style={{ border: '1px solid var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
                placeholder="Masalan: summa, 2025, Aziz..."
              />
            </div>

            {mode === 'replace' && (
              <div>
                <label className="block text-xs font-medium mb-1" style={{ color: 'var(--text-secondary)' }}>
                  Almashtirish
                </label>
                <input
                  ref={replaceInputRef}
                  value={replaceText}
                  onChange={(e) => onChangeReplaceText(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault();
                      onReplace();
                    }
                  }}
                  className="w-full px-3 py-2 rounded text-sm focus:outline-none"
                  style={{ border: '1px solid var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
                  placeholder="Yangi matn..."
                />
              </div>
            )}
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium mb-1" style={{ color: 'var(--text-secondary)' }}>
                Qamrov (scope)
              </label>
              <select
                value={scope}
                onChange={(e) => onChangeScope(e.target.value as FindScope)}
                className="w-full px-3 py-2 rounded text-sm focus:outline-none"
                style={{ border: '1px solid var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
              >
                <option value="sheet">Butun jadval</option>
                <option value="selection">Tanlangan range</option>
              </select>
            </div>

            <div className="flex items-end gap-3">
              <label className="flex items-center gap-2 text-sm cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={matchCase}
                  onChange={(e) => onChangeMatchCase(e.target.checked)}
                />
                <span style={{ color: 'var(--text-primary)' }}>Katta-kichik harf</span>
                <CaseSensitive size={14} style={{ color: 'var(--text-secondary)' }} />
              </label>
              <label className="flex items-center gap-2 text-sm cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={wholeCell}
                  onChange={(e) => onChangeWholeCell(e.target.checked)}
                />
                <span style={{ color: 'var(--text-primary)' }}>Butun katak</span>
              </label>
            </div>
          </div>
        </div>

        <div className="px-5 py-4 flex flex-wrap items-center justify-between gap-2 border-t" style={{ borderColor: 'var(--border-color)', background: 'var(--bg-light)' }}>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={onFindPrev}
              disabled={!canNavigate}
              className="px-3 py-2 rounded text-sm flex items-center gap-2 disabled:opacity-50"
              style={{ background: 'var(--card-bg)', border: '1px solid var(--border-color)', color: 'var(--text-primary)' }}
              title="Oldingi (Shift+Enter)"
            >
              <ArrowUp size={16} />
              Oldingi
            </button>
            <button
              type="button"
              onClick={onFindNext}
              disabled={!canNavigate}
              className="px-3 py-2 rounded text-sm flex items-center gap-2 disabled:opacity-50"
              style={{ background: 'var(--card-bg)', border: '1px solid var(--border-color)', color: 'var(--text-primary)' }}
              title="Keyingi (Enter)"
            >
              <ArrowDown size={16} />
              Keyingi
            </button>
          </div>

          {mode === 'replace' ? (
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={onReplace}
                disabled={!canNavigate}
                className="px-3 py-2 rounded text-sm flex items-center gap-2 disabled:opacity-50"
                style={{ background: 'var(--card-bg)', border: '1px solid var(--border-color)', color: 'var(--text-primary)' }}
                title="Almashtirish (Enter)"
              >
                <Replace size={16} />
                Almashtirish
              </button>
              <button
                type="button"
                onClick={onReplaceAll}
                disabled={query.trim().length === 0}
                className="px-3 py-2 rounded text-sm text-white disabled:opacity-50"
                style={{ background: 'var(--primary)' }}
                title="Hammasini almashtirish"
              >
                Hammasi
              </button>
            </div>
          ) : (
            <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
              Ctrl+F — qidirish, Ctrl+H — almashtirish
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default FindReplaceModal;
