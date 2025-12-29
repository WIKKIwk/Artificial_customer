import React, { useState, useMemo, useRef, useEffect } from 'react';
import { FunctionSquare, Search } from 'lucide-react';
import { usePresence } from '../utils/usePresence';

type FunctionItem = {
  name: string;
  snippet: string;
  description: string;
  category: string;
};

const FUNCTION_CATALOG: FunctionItem[] = [
  { name: 'SUM', snippet: 'SUM()', description: "Kataklar yig'indisini hisoblaydi", category: 'Math' },
  { name: 'AVERAGE', snippet: 'AVERAGE()', description: "O'rtacha qiymat", category: 'Math' },
  { name: 'MAX', snippet: 'MAX()', description: 'Eng katta qiymat', category: 'Math' },
  { name: 'MIN', snippet: 'MIN()', description: 'Eng kichik qiymat', category: 'Math' },
  { name: 'COUNT', snippet: 'COUNT()', description: 'Raqamli kataklar soni', category: 'Math' },
  { name: 'COUNTA', snippet: 'COUNTA()', description: "Bo'sh bo'lmagan kataklar soni", category: 'Math' },
  { name: 'SUMIF', snippet: 'SUMIF(range, criteria, sum_range)', description: "Shart bilan yig'ish", category: 'Logic' },
  { name: 'SUMIFS', snippet: 'SUMIFS(sum_range, criteria_range1, criteria1)', description: "Bir nechta shart bilan yig'ish", category: 'Logic' },
  { name: 'COUNTIF', snippet: 'COUNTIF(range, criteria)', description: "Shartga mos kataklar soni", category: 'Logic' },
  { name: 'COUNTIFS', snippet: 'COUNTIFS(criteria_range1, criteria1)', description: "Bir nechta shart bilan sanash", category: 'Logic' },
  { name: 'IF', snippet: 'IF(condition, value_if_true, value_if_false)', description: 'Shartli qiymat qaytaradi', category: 'Logic' },
  { name: 'IFS', snippet: 'IFS(condition1, value1, condition2, value2)', description: 'Bir nechta shartli qiymat', category: 'Logic' },
  { name: 'AND', snippet: 'AND(condition1, condition2)', description: 'Hamma shartlar bajarilsa TRUE', category: 'Logic' },
  { name: 'OR', snippet: 'OR(condition1, condition2)', description: 'Hech bo\'lmasa bitta shart TRUE', category: 'Logic' },
  { name: 'NOT', snippet: 'NOT(condition)', description: 'Mantiqiy inkor', category: 'Logic' },
  { name: 'VLOOKUP', snippet: 'VLOOKUP(search_key, range, index)', description: 'Vertikal qidiruv', category: 'Lookup' },
  { name: 'HLOOKUP', snippet: 'HLOOKUP(search_key, range, index)', description: 'Gorizontal qidiruv', category: 'Lookup' },
  { name: 'INDEX', snippet: 'INDEX(range, row, column)', description: 'Koordinata bo‘yicha qiymat', category: 'Lookup' },
  { name: 'MATCH', snippet: 'MATCH(search_key, range, [type])', description: 'Qiymatning pozitsiyasi', category: 'Lookup' },
  { name: 'TEXTJOIN', snippet: 'TEXTJOIN(delimiter, TRUE, value1, value2)', description: 'Matnlarni delimiter bilan birlashtiradi', category: 'Text' },
  { name: 'CONCAT', snippet: 'CONCAT(value1, value2)', description: 'Ikki matnni qo‘shadi', category: 'Text' },
  { name: 'UPPER', snippet: 'UPPER(text)', description: "Matnni katta harfga o'tkazish", category: 'Text' },
  { name: 'LOWER', snippet: 'LOWER(text)', description: "Matnni kichik harfga o'tkazish", category: 'Text' },
  { name: 'PROPER', snippet: 'PROPER(text)', description: "Har so'z boshini katta qiladi", category: 'Text' },
  { name: 'LEFT', snippet: 'LEFT(text, number)', description: 'Matn boshidan belgilar', category: 'Text' },
  { name: 'RIGHT', snippet: 'RIGHT(text, number)', description: 'Matn oxiridan belgilar', category: 'Text' },
  { name: 'MID', snippet: 'MID(text, start, number)', description: "Matn o'rtasidan kesish", category: 'Text' },
  { name: 'LEN', snippet: 'LEN(text)', description: 'Matn uzunligi', category: 'Text' },
  { name: 'DATE', snippet: 'DATE(year, month, day)', description: 'Sana yasash', category: 'Date/Time' },
  { name: 'TODAY', snippet: 'TODAY()', description: 'Bugungi sana', category: 'Date/Time' },
  { name: 'NOW', snippet: 'NOW()', description: 'Sana va vaqt', category: 'Date/Time' },
  { name: 'EDATE', snippet: 'EDATE(date, months)', description: 'Sana ustiga oy qo‘shish', category: 'Date/Time' },
  { name: 'NETWORKDAYS', snippet: 'NETWORKDAYS(start_date, end_date, [holidays])', description: 'Ish kunlari soni', category: 'Date/Time' },
  { name: 'ROUND', snippet: 'ROUND(value, places)', description: 'Yaxlitlash', category: 'Math' },
  { name: 'TRUNC', snippet: 'TRUNC(value, places)', description: 'Kasr qismini olib tashlash', category: 'Math' },
  { name: 'RANDBETWEEN', snippet: 'RANDBETWEEN(bottom, top)', description: 'Tasodifiy butun son', category: 'Math' },
];

interface FunctionPickerProps {
  onInsert: (snippet: string) => void;
}

const FunctionPicker: React.FC<FunctionPickerProps> = ({ onInsert }) => {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const containerRef = useRef<HTMLDivElement>(null);
  const popoverPresence = usePresence(open, { exitDurationMs: 180 });

  useEffect(() => {
    if (!open) return;
    const handleClickOutside = (event: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, [open]);

  const filtered = useMemo(() => {
    const q = query.trim().toUpperCase();
    if (!q) return FUNCTION_CATALOG;
    return FUNCTION_CATALOG.filter(fn =>
      fn.name.toUpperCase().includes(q) || fn.description.toUpperCase().includes(q) || fn.category.toUpperCase().includes(q)
    );
  }, [query]);

  const handleSelect = (snippet: string) => {
    onInsert(snippet);
    setOpen(false);
    setQuery('');
  };

  return (
    <div className="relative" ref={containerRef}>
      <button
        type="button"
        onClick={() => setOpen(prev => !prev)}
        className="w-8 h-8 flex items-center justify-center rounded hover:bg-gray-100 transition-colors"
        style={{ border: '1px solid var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
        aria-label="Insert function"
      >
        <FunctionSquare size={18} />
      </button>
      {popoverPresence.isMounted && (
        <div
          className="absolute left-0 mt-2 w-72 rounded shadow-xl z-30 ui-popover"
          data-state={popoverPresence.state}
          style={{ background: 'var(--card-bg)', border: '1px solid var(--border-color)', transformOrigin: 'top left' }}
        >
          <div className="p-2 border-b" style={{ borderColor: 'var(--border-color)' }}>
            <div className="flex items-center px-2 py-1 rounded" style={{ background: 'var(--bg-light)' }}>
              <Search size={14} className="mr-1" style={{ color: 'var(--text-secondary)' }} />
              <input
                type="text"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Funksiya qidirish..."
                className="flex-1 text-sm bg-transparent focus:outline-none"
                style={{ color: 'var(--text-primary)' }}
              />
            </div>
          </div>
          <div className="max-h-64 overflow-y-auto">
            {filtered.length === 0 && (
              <div className="px-3 py-4 text-sm" style={{ color: 'var(--text-secondary)' }}>
                Mos funksiya topilmadi
              </div>
            )}
            {filtered.map((fn) => (
              <button
                key={fn.name}
                className="w-full text-left px-3 py-2 hover:bg-gray-100 transition-colors"
                onClick={() => handleSelect(fn.snippet)}
              >
                <div className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                  {fn.name}
                </div>
                <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                  {fn.description}
                </div>
                <div className="text-xs mt-1" style={{ color: 'var(--text-secondary)', opacity: 0.8 }}>
                  {fn.snippet}
                </div>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};

export default FunctionPicker;
