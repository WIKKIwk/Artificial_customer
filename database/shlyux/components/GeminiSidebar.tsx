import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { X, Send, Loader2, Settings, Check, List, Paperclip, Square, MessageSquarePlus, History, Trash2 } from 'lucide-react';
import { SheetState } from '../types';
import { cellLabelToCoords, getColumnLabel, getCellId, NUM_COLS } from '../utils/spreadsheetUtils';
import { getGeminiApiKey, saveGeminiApiKey, convertExcel } from '../utils/api';
import { usePresence } from '../utils/usePresence';

// Helper to parse "row,col" id back to indices
const getCellIdParts = (id: string): { row: number; col: number } => {
  const [rowStr, colStr] = id.split(',');
  return { row: Number(rowStr), col: Number(colStr) };
};

const API_KEY_STORAGE = 'sheetmaster-gemini-api-key';
const LEGACY_CHAT_STORAGE = 'sheetmaster-gemini-chat-history-v1';
const CHATS_STORAGE = 'sheetmaster-gemini-chats-v2';
const ACTIVE_CHAT_STORAGE = 'sheetmaster-gemini-active-chat-v2';
const MAX_SAVED_MESSAGES = 120;
const MAX_CONTEXT_CELLS = 360;
const MAX_CONTEXT_CHARS = 14000;

type SheetBounds = { minRow: number; maxRow: number; minCol: number; maxCol: number };
type SheetRange = SheetBounds & { reason: string };
type SheetCoord = { row: number; col: number };

const clamp = (val: number, min: number, max: number) => Math.max(min, Math.min(max, val));

const rangeToA1 = (range: SheetBounds): string => {
  return `${getColumnLabel(range.minCol)}${range.minRow + 1}:${getColumnLabel(range.maxCol)}${range.maxRow + 1}`;
};

const computeUsedRange = (entryIds: string[]): SheetBounds | null => {
  let minRow = Number.POSITIVE_INFINITY;
  let minCol = Number.POSITIVE_INFINITY;
  let maxRow = -1;
  let maxCol = -1;

  entryIds.forEach((id) => {
    const { row, col } = getCellIdParts(id);
    if (!Number.isFinite(row) || !Number.isFinite(col)) return;
    minRow = Math.min(minRow, row);
    minCol = Math.min(minCol, col);
    maxRow = Math.max(maxRow, row);
    maxCol = Math.max(maxCol, col);
  });

  if (!Number.isFinite(minRow) || !Number.isFinite(minCol) || maxRow < 0 || maxCol < 0) return null;
  return { minRow, maxRow, minCol, maxCol };
};

const getCellDisplayString = (cell: any): string => {
  const raw = cell?.value;
  const computed = cell?.computed;
  const val = computed !== undefined && computed !== null && computed !== '' ? computed : raw;
  return (val ?? '').toString().replace(/\s+/g, ' ').trim();
};

const normalizeKey = (value: string): string => value.toLowerCase().replace(/\s+/g, ' ').trim();

const isNumericLike = (value: string): boolean => {
  const trimmed = value.trim();
  if (!trimmed) return false;
  return /^-?\d+(\.\d+)?$/.test(trimmed);
};

const formatCellLine = (row: number, col: number, cell: any): string => {
  const label = `${getColumnLabel(col)}${row + 1}`;
  const raw = (cell?.value ?? '').toString().replace(/\s+/g, ' ').trim();
  const computed = getCellDisplayString(cell);
  if (raw.startsWith('=') && computed && computed !== raw) {
    return `${label}: ${raw} -> ${computed}`;
  }
  return `${label}: ${computed || raw}`;
};

const parseA1Refs = (text: string): { cells: SheetCoord[]; ranges: SheetBounds[] } => {
  const upper = (text || '').toUpperCase();
  const ranges: SheetBounds[] = [];
  const cells: SheetCoord[] = [];

  const rangeRe = /(^|[^A-Z0-9])([A-Z]{1,3}[0-9]{1,7})\s*:\s*([A-Z]{1,3}[0-9]{1,7})(?=[^A-Z0-9]|$)/g;
  for (const match of upper.matchAll(rangeRe)) {
    const start = match[2];
    const end = match[3];
    const a = cellLabelToCoords(start);
    const b = cellLabelToCoords(end);
    if (!a || !b) continue;
    const minRow = Math.min(a.row, b.row);
    const maxRow = Math.max(a.row, b.row);
    const minCol = Math.min(a.col, b.col);
    const maxCol = Math.max(a.col, b.col);
    ranges.push({ minRow, maxRow, minCol, maxCol });
  }

  const cellRe = /(^|[^A-Z0-9])([A-Z]{1,3}[0-9]{1,7})(?=[^A-Z0-9]|$)/g;
  for (const match of upper.matchAll(cellRe)) {
    const label = match[2];
    const coords = cellLabelToCoords(label);
    if (!coords) continue;
    cells.push({ row: coords.row, col: coords.col });
  }

  // De-duplicate & cap for safety.
  const uniqueCells = Array.from(new Map(cells.map((c) => [`${c.row},${c.col}`, c])).values()).slice(0, 32);
  const uniqueRanges = Array.from(new Map(ranges.map((r) => [`${r.minRow}:${r.maxRow}:${r.minCol}:${r.maxCol}`, r])).values()).slice(0, 10);
  return { cells: uniqueCells, ranges: uniqueRanges };
};

const formatMaybeEmptyCellLine = (row: number, col: number, cell: any): string => {
  const label = `${getColumnLabel(col)}${row + 1}`;
  const raw = (cell?.value ?? '').toString();
  if (!raw) return `${label}: (empty)`;
  return formatCellLine(row, col, cell);
};

const buildAiSheetContext = (sheetState: SheetState, userText?: string): {
  totalNonEmpty: number;
  usedRangeA1: string;
  usedBounds: SheetBounds | null;
  selectionA1: string | null;
  selectionBounds: SheetBounds | null;
  activeCellA1: string | null;
  explicitCellsA1: string[];
  explicitRangesA1: string[];
  headerRowIndex: number | null;
  headerPreview: string | null;
  schemaText: string;
  includedCells: number;
  truncated: boolean;
  contextText: string;
} => {
  const nonEmptyIds = Object.entries(sheetState.data)
    .filter(([, cell]) => cell?.value !== undefined && cell.value !== '')
    .map(([id]) => id);

  const totalNonEmpty = nonEmptyIds.length;
  const used = computeUsedRange(nonEmptyIds);

  const selection = sheetState.selection;
  const active = sheetState.activeCell;

  const refs = userText ? parseA1Refs(userText) : { cells: [], ranges: [] };
  const explicitCellsA1 = refs.cells.map((c) => `${getColumnLabel(c.col)}${c.row + 1}`);
  const explicitRangesA1 = refs.ranges.map((r) => rangeToA1(r));

  const ranges: SheetRange[] = [];
  refs.ranges.forEach((r) => {
    ranges.push({ ...r, reason: 'explicit_range' });
  });

  let selectionBounds: SheetBounds | null = null;
  let selectionA1: string | null = null;
  if (selection) {
    const minRow = Math.min(selection.start.row, selection.end.row);
    const maxRow = Math.max(selection.start.row, selection.end.row);
    const minCol = Math.min(selection.start.col, selection.end.col);
    const maxCol = Math.max(selection.start.col, selection.end.col);
    ranges.push({ minRow, maxRow, minCol, maxCol, reason: 'selection' });
    selectionBounds = { minRow, maxRow, minCol, maxCol };
    selectionA1 = rangeToA1(selectionBounds);
  }

  if (active) {
    const maxRowLimit = Math.max(0, (sheetState.rowCount || 1) - 1);
    const minRow = clamp(active.row - 15, 0, maxRowLimit);
    const maxRow = clamp(active.row + 15, 0, maxRowLimit);
    const minCol = clamp(active.col - 6, 0, NUM_COLS - 1);
    const maxCol = clamp(active.col + 6, 0, NUM_COLS - 1);
    ranges.push({ minRow, maxRow, minCol, maxCol, reason: 'active_window' });
  }

  const headerRowIndex = (() => {
    if (selectionBounds) return selectionBounds.minRow;
    if (!used) return active?.row ?? 0;

    const maxRowLimit = Math.max(0, (sheetState.rowCount || 1) - 1);
    const start = clamp(used.minRow, 0, maxRowLimit);
    const end = clamp(Math.min(used.minRow + 4, used.maxRow), 0, maxRowLimit);
    const minC = clamp(used.minCol, 0, NUM_COLS - 1);
    const maxC = clamp(used.maxCol, 0, NUM_COLS - 1);

    let bestRow = start;
    let bestScore = -1;
    for (let r = start; r <= end; r++) {
      let nonEmpty = 0;
      let textLike = 0;
      for (let c = minC; c <= maxC; c++) {
        const cell = (sheetState.data as any)[getCellId(r, c)];
        const value = getCellDisplayString(cell);
        if (!value) continue;
        nonEmpty++;
        if (!isNumericLike(value) && /[a-zа-я]/i.test(value)) {
          textLike++;
        }
      }
      const score = nonEmpty + textLike * 0.25;
      if (score > bestScore) {
        bestScore = score;
        bestRow = r;
      }
    }
    return bestScore > 0 ? bestRow : used.minRow;
  })();

  if (used) {
    const maxRowLimit = Math.max(0, (sheetState.rowCount || 1) - 1);
    const headerRow = clamp(headerRowIndex ?? used.minRow, 0, maxRowLimit);
    ranges.push({ minRow: headerRow, maxRow: headerRow, minCol: used.minCol, maxCol: used.maxCol, reason: 'header' });
    const topEnd = Math.min(used.minRow + 10, used.maxRow);
    ranges.push({ minRow: used.minRow, maxRow: topEnd, minCol: used.minCol, maxCol: used.maxCol, reason: 'top_rows' });
    const bottomStart = Math.max(used.maxRow - 8, used.minRow);
    ranges.push({ minRow: bottomStart, maxRow: used.maxRow, minCol: used.minCol, maxCol: used.maxCol, reason: 'bottom_rows' });
  }

  const inAnyRange = (row: number, col: number) => {
    if (ranges.length === 0) return true;
    return ranges.some((r) => row >= r.minRow && row <= r.maxRow && col >= r.minCol && col <= r.maxCol);
  };

  const nonEmptyCells = nonEmptyIds
    .map((id) => {
      const { row, col } = getCellIdParts(id);
      return { id, row, col };
    })
    .sort((a, b) => (a.row - b.row) || (a.col - b.col));

  const lines: string[] = [];
  let charCount = 0;
  let truncated = false;
  const seen = new Set<string>();

  const pushLine = (line: string, id?: string) => {
    const nextCharCount = charCount + line.length + 1;
    if (lines.length >= MAX_CONTEXT_CELLS || nextCharCount > MAX_CONTEXT_CHARS) {
      truncated = true;
      return false;
    }
    lines.push(line);
    charCount = nextCharCount;
    if (id) seen.add(id);
    return true;
  };

  // Always include explicit referenced cells (even if empty) first.
  for (const coord of refs.cells) {
    const id = getCellId(coord.row, coord.col);
    if (seen.has(id)) continue;
    const cell = (sheetState.data as any)[id];
    const line = formatMaybeEmptyCellLine(coord.row, coord.col, cell);
    if (!pushLine(line, id)) break;
  }

  // Always include header row cells early (non-empty) so the assistant can locate columns like "Category".
  if (!truncated && headerRowIndex !== null) {
    for (let c = 0; c < NUM_COLS; c++) {
      if (truncated) break;
      const id = getCellId(headerRowIndex, c);
      if (seen.has(id)) continue;
      const cell = (sheetState.data as any)[id];
      const value = getCellDisplayString(cell);
      if (!value) continue;
      const line = `${getColumnLabel(c)}${headerRowIndex + 1}: ${value}`;
      if (!pushLine(line, id)) break;
    }
  }

  // Then include non-empty cells from ranges by priority (order in `ranges`).
  for (const range of ranges) {
    if (truncated) break;
    for (const item of nonEmptyCells) {
      if (truncated) break;
      if (seen.has(item.id)) continue;
      if (!inAnyRange(item.row, item.col)) continue;
      // Keep ordering stable but still allow ranges to control inclusion: only include if inside current range.
      const insideThis =
        item.row >= range.minRow &&
        item.row <= range.maxRow &&
        item.col >= range.minCol &&
        item.col <= range.maxCol;
      if (!insideThis) continue;
      const cell = (sheetState.data as any)[item.id];
      const line = formatCellLine(item.row, item.col, cell);
      if (!pushLine(line, item.id)) break;
    }
  }

  const usedRangeA1 = used ? rangeToA1(used) : '(empty)';
  const activeCellA1 = active ? `${getColumnLabel(active.col)}${active.row + 1}` : null;
  const contextText = lines.length ? lines.join('\n') : '(no cells in context)';

  const headerPreview = (() => {
    if (headerRowIndex === null) return null;
    const minCol = used?.minCol ?? 0;
    const maxCol = used?.maxCol ?? Math.min(NUM_COLS - 1, minCol + 12);
    const parts: string[] = [];
    for (let c = minCol; c <= Math.min(maxCol, NUM_COLS - 1); c++) {
      const id = getCellId(headerRowIndex, c);
      const cell = (sheetState.data as any)[id];
      const raw = (cell?.computed ?? cell?.value ?? '').toString().trim();
      if (!raw) continue;
      parts.push(`${getColumnLabel(c)}: ${raw.replace(/\s+/g, ' ')}`);
      if (parts.length >= 12) break;
    }
    return parts.length ? parts.join(' | ') : null;
  })();

  const schemaText = (() => {
    const headerRow = headerRowIndex ?? 0;
    const headerPairs: string[] = [];
    const headerByCol = new Map<number, string>();
    for (let c = 0; c < NUM_COLS; c++) {
      const cell = (sheetState.data as any)[getCellId(headerRow, c)];
      const value = getCellDisplayString(cell);
      if (!value) continue;
      headerByCol.set(c, value);
      const safe = value.length > 40 ? `${value.slice(0, 40)}…` : value;
      headerPairs.push(`${getColumnLabel(c)}="${safe}"`);
    }

    const keywords = ['category', 'kategoriya', 'categoriya', 'катег', 'tur', 'type', 'group', 'bo‘lim', "bo'lim", 'section'];
    const categoryByHeader = Array.from(headerByCol.entries())
      .filter(([, name]) => keywords.some((k) => normalizeKey(name).includes(k)))
      .map(([c]) => `${getColumnLabel(c)}(${c})`)
      .slice(0, 6);

    const usedMinCol = clamp(used?.minCol ?? 0, 0, NUM_COLS - 1);
    const usedMaxCol = clamp(used?.maxCol ?? (NUM_COLS - 1), 0, NUM_COLS - 1);
    const usedColsText = used ? `${getColumnLabel(usedMinCol)}..${getColumnLabel(usedMaxCol)}` : 'n/a';

    const sampleStartRaw = headerRowIndex !== null ? headerRowIndex + 1 : (selectionBounds?.minRow ?? used?.minRow ?? 0);
    const sampleEndRaw = selectionBounds?.maxRow ?? used?.maxRow ?? sampleStartRaw;
    const maxRowLimit = Math.max(0, (sheetState.rowCount || 1) - 1);
    const sampleStart = clamp(sampleStartRaw, 0, maxRowLimit);
    const sampleEnd = clamp(sampleEndRaw, sampleStart, maxRowLimit);

    const makeSampleRows = (start: number, end: number, target: number) => {
      const span = end - start + 1;
      if (span <= 0) return [];
      if (span <= target) return Array.from({ length: span }, (_, i) => start + i);
      const step = span / (target - 1);
      const out: number[] = [];
      for (let i = 0; i < target; i++) {
        out.push(Math.floor(start + i * step));
      }
      return Array.from(new Set(out)).filter((r) => r >= start && r <= end);
    };

    const sampleRows = makeSampleRows(sampleStart, sampleEnd, 36);

    const dataCandidates: Array<{ col: number; uniqueCount: number; nonEmpty: number; examples: string[] }> = [];
    for (let c = usedMinCol; c <= usedMaxCol; c++) {
      const values: string[] = [];
      for (const r of sampleRows) {
        const cell = (sheetState.data as any)[getCellId(r, c)];
        const v = getCellDisplayString(cell);
        if (!v) continue;
        values.push(v);
      }
      if (values.length < 8) continue;
      const normalized = values.map(normalizeKey);
      const uniq = new Set(normalized);
      const numericCount = values.filter(isNumericLike).length;
      const textRatio = (values.length - numericCount) / values.length;
      const uniqueCount = uniq.size;
      const maxAllowed = Math.max(3, Math.floor(values.length * 0.35));
      if (textRatio < 0.75) continue;
      if (uniqueCount > maxAllowed) continue;

      const freq = new Map<string, { count: number; sample: string }>();
      normalized.forEach((k, idx) => {
        if (!k) return;
        const existing = freq.get(k);
        if (existing) {
          existing.count += 1;
        } else {
          freq.set(k, { count: 1, sample: values[idx] });
        }
      });
      const examples = [...freq.values()]
        .sort((a, b) => b.count - a.count)
        .slice(0, 4)
        .map((e) => (e.sample.length > 18 ? `${e.sample.slice(0, 18)}…` : e.sample));

      dataCandidates.push({ col: c, uniqueCount, nonEmpty: values.length, examples });
    }

    dataCandidates.sort((a, b) => (a.uniqueCount - b.uniqueCount) || (b.nonEmpty - a.nonEmpty));
    const categoryByData = dataCandidates.slice(0, 4).map((c) => {
      const headerName = headerByCol.get(c.col);
      const headerHint = headerName ? ` "${headerName.length > 22 ? `${headerName.slice(0, 22)}…` : headerName}"` : '';
      return `${getColumnLabel(c.col)}(${c.col})${headerHint} uniq ${c.uniqueCount}/${c.nonEmpty} ex: ${c.examples.join(', ')}`;
    });

    const headersText = headerPairs.length ? headerPairs.join(', ') : '(no headers found)';
    const headersCompact = headersText.length > 2600 ? `${headersText.slice(0, 2600)}…` : headersText;

    const lines = [
      `Used columns: ${usedColsText}`,
      `Header row guess (0-based): ${headerRowIndex ?? 'n/a'} (1-based: ${headerRowIndex !== null ? headerRowIndex + 1 : 'n/a'})`,
      `Headers: ${headersCompact}`,
      `Category by header: ${categoryByHeader.length ? categoryByHeader.join(', ') : '(none)'}`,
      `Category by data: ${categoryByData.length ? categoryByData.join(' | ') : '(none)'}`,
    ];
    return lines.join('\n');
  })();

  return {
    totalNonEmpty,
    usedRangeA1,
    usedBounds: used,
    selectionA1,
    selectionBounds,
    activeCellA1,
    explicitCellsA1,
    explicitRangesA1,
    headerRowIndex,
    headerPreview,
    schemaText,
    includedCells: lines.length,
    truncated,
    contextText,
  };
};

type ChatMessage = { role: 'user' | 'model'; text: string };
type ChatThread = {
  id: string;
  title: string;
  createdAt: number;
  updatedAt: number;
  messages: ChatMessage[];
};

type AiCellEdit = { row: number; col: number; value: string };
type AiAction =
  | { action: 'update_cells'; cells: AiCellEdit[]; message?: string }
  | { action: 'clear_range'; startRow: number; endRow: number; startCol: number; endCol: number; message?: string }
  | { action: 'sort_range'; startRow: number; endRow: number; startCol: number; endCol: number; sortCol?: number; direction: 'asc' | 'desc'; hasHeader?: boolean; message?: string }
  | { action: 'delete_rows'; rows: number[]; message?: string }
  | { action: 'delete_cols'; cols: number[]; message?: string }
  | { action: 'noop'; message?: string }
  | { action: string; [key: string]: any };

type AiResponseEnvelope = { actions: AiAction[]; message?: string } | (AiAction & { message?: string });

const DEFAULT_MESSAGES: ChatMessage[] = [
  { role: 'model', text: "Salom! Men jadval yordamchisiman. Gemini API kalitini sozlamalardan kiriting va men uchun vazifa yuboring." }
];

const isChatMessage = (value: unknown): value is ChatMessage => {
  if (!value || typeof value !== 'object') return false;
  const msg = value as { role?: unknown; text?: unknown };
  return (msg.role === 'user' || msg.role === 'model') && typeof msg.text === 'string';
};

const makeChatId = (): string => {
  try {
    if (typeof crypto !== 'undefined' && typeof (crypto as any).randomUUID === 'function') {
      return (crypto as any).randomUUID();
    }
  } catch {
    // ignore
  }
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
};

const deriveChatTitle = (msgs: ChatMessage[]): string => {
  const firstUser = msgs.find((m) => m.role === 'user' && m.text.trim());
  const raw = (firstUser?.text || '').split('\n')[0]?.trim() || 'Yangi chat';
  const cleaned = raw.replace(/\s+/g, ' ').trim();
  if (!cleaned) return 'Yangi chat';
  return cleaned.length > 36 ? `${cleaned.slice(0, 36)}…` : cleaned;
};

const isChatThread = (value: unknown): value is ChatThread => {
  if (!value || typeof value !== 'object') return false;
  const t = value as Partial<ChatThread>;
  return (
    typeof t.id === 'string' &&
    typeof t.title === 'string' &&
    typeof t.createdAt === 'number' &&
    typeof t.updatedAt === 'number' &&
    Array.isArray(t.messages) &&
    t.messages.every(isChatMessage)
  );
};

const loadLegacyMessages = (): ChatMessage[] => {
  if (typeof window === 'undefined') return DEFAULT_MESSAGES;
  try {
    const saved = localStorage.getItem(LEGACY_CHAT_STORAGE);
    if (!saved) return DEFAULT_MESSAGES;
    const parsed = JSON.parse(saved) as unknown;
    if (!Array.isArray(parsed)) return DEFAULT_MESSAGES;
    const valid = parsed.filter(isChatMessage);
    return valid.length ? valid.slice(-MAX_SAVED_MESSAGES) : DEFAULT_MESSAGES;
  } catch {
    return DEFAULT_MESSAGES;
  }
};

const loadInitialChatState = (): { threads: ChatThread[]; activeId: string; messages: ChatMessage[] } => {
  if (typeof window === 'undefined') {
    const id = 'default';
    const createdAt = 0;
    const thread: ChatThread = { id, title: 'Yangi chat', createdAt, updatedAt: createdAt, messages: DEFAULT_MESSAGES };
    return { threads: [thread], activeId: id, messages: thread.messages };
  }

  const readThreads = (): ChatThread[] => {
    try {
      const raw = localStorage.getItem(CHATS_STORAGE);
      if (!raw) return [];
      const parsed = JSON.parse(raw) as unknown;
      if (!Array.isArray(parsed)) return [];
      const valid = parsed.filter(isChatThread).map((t) => ({ ...t, messages: t.messages.slice(-MAX_SAVED_MESSAGES) }));
      return valid;
    } catch {
      return [];
    }
  };

  let threads = readThreads();
  if (threads.length === 0) {
    const now = Date.now();
    const legacyMessages = loadLegacyMessages();
    const seedMessages = legacyMessages.length ? legacyMessages : DEFAULT_MESSAGES;
    const thread: ChatThread = {
      id: makeChatId(),
      title: deriveChatTitle(seedMessages),
      createdAt: now,
      updatedAt: now,
      messages: seedMessages.slice(-MAX_SAVED_MESSAGES),
    };
    threads = [thread];
  }

  const storedActive = localStorage.getItem(ACTIVE_CHAT_STORAGE);
  const activeThread = threads.find((t) => t.id === storedActive) || threads[0];

  return {
    threads,
    activeId: activeThread.id,
    messages: (activeThread.messages?.length ? activeThread.messages : DEFAULT_MESSAGES).slice(-MAX_SAVED_MESSAGES),
  };
};

interface GeminiSidebarProps {
  isOpen: boolean;
  onClose: () => void;
  sheetState: SheetState;
  onApplyChanges: (edits: Array<{ row: number; col: number; value: string }>) => void;
  onApplyAiAction?: (action: AiAction) => void;
  authToken?: string | null;
}

const GeminiSidebar: React.FC<GeminiSidebarProps> = ({ isOpen, onClose, sheetState, onApplyChanges, onApplyAiAction, authToken }) => {
  const [input, setInput] = useState('');
  const inputRef = useRef<HTMLTextAreaElement | null>(null);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);
  const [loading, setLoading] = useState(false);
  const initialChatStateRef = useRef<ReturnType<typeof loadInitialChatState> | null>(null);
  if (initialChatStateRef.current === null) {
    initialChatStateRef.current = loadInitialChatState();
  }
  const initialChatState = initialChatStateRef.current;

  const [chatThreads, setChatThreads] = useState<ChatThread[]>(initialChatState.threads);
  const [activeChatId, setActiveChatId] = useState<string>(initialChatState.activeId);
  const [messages, setMessages] = useState<ChatMessage[]>(initialChatState.messages);
  const [typingMessageIndex, setTypingMessageIndex] = useState<number | null>(null);
  const [displayedText, setDisplayedText] = useState<string>('');
  const [attachedName, setAttachedName] = useState<string | null>(null);
  const [attachedContent, setAttachedContent] = useState<string | null>(null);
  const [attachedPreview, setAttachedPreview] = useState<string | null>(null); // for images/base64
  const [fileError, setFileError] = useState<string | null>(null);
  const [fileLoading, setFileLoading] = useState(false);
  const [serverKeyLoading, setServerKeyLoading] = useState(false);
  const abortControllerRef = useRef<AbortController | null>(null);
  const [showHistory, setShowHistory] = useState(false);
  const historyPresence = usePresence(showHistory, { exitDurationMs: 180 });

  // Settings state
  const [showSettings, setShowSettings] = useState(() => {
    if (typeof window === 'undefined') return false;
    return !localStorage.getItem(API_KEY_STORAGE);
  });
  const [apiKey, setApiKey] = useState(() => {
    if (typeof window === 'undefined') return '';
    return localStorage.getItem(API_KEY_STORAGE) || '';
  });
  const [apiKeyValidating, setApiKeyValidating] = useState(false);
  const [apiKeyValid, setApiKeyValid] = useState(() => {
    if (typeof window === 'undefined') return false;
    return !!localStorage.getItem(API_KEY_STORAGE);
  });
  const [apiKeyError, setApiKeyError] = useState<string | null>(null);
  const [showPrompts, setShowPrompts] = useState(false);
  const promptsPresence = usePresence(showPrompts, { exitDurationMs: 180 });
  const [selectedLanguage, setSelectedLanguage] = useState(() => {
    if (typeof window === 'undefined') return 'uzbek';
    return localStorage.getItem('ai-language') || 'uzbek';
  });

  const activeThread = useMemo(
    () => chatThreads.find((t) => t.id === activeChatId) || chatThreads[0],
    [chatThreads, activeChatId]
  );
  const activeChatTitle = activeThread?.title || 'Yangi chat';
  const sortedThreads = useMemo(
    () => [...chatThreads].sort((a, b) => (b.updatedAt || 0) - (a.updatedAt || 0)),
    [chatThreads]
  );

  // Persist chat threads across refresh.
  useEffect(() => {
    if (typeof window === 'undefined') return;
    try {
      localStorage.setItem(CHATS_STORAGE, JSON.stringify(chatThreads));
    } catch {
      // ignore localStorage quota/errors
    }
  }, [chatThreads]);

  useEffect(() => {
    if (typeof window === 'undefined') return;
    try {
      localStorage.setItem(ACTIVE_CHAT_STORAGE, activeChatId);
    } catch {
      // ignore
    }
  }, [activeChatId]);

  const switchToChat = useCallback((id: string) => {
    const thread = chatThreads.find((t) => t.id === id);
    if (!thread) return;
    setActiveChatId(thread.id);
    setMessages((thread.messages?.length ? thread.messages : DEFAULT_MESSAGES).slice(-MAX_SAVED_MESSAGES));
    setTypingMessageIndex(null);
    setDisplayedText('');
    setShowHistory(false);
    setTimeout(() => inputRef.current?.focus(), 50);
  }, [chatThreads]);

  const startNewChat = useCallback(() => {
    const now = Date.now();
    const id = makeChatId();
    const thread: ChatThread = {
      id,
      title: 'Yangi chat',
      createdAt: now,
      updatedAt: now,
      messages: DEFAULT_MESSAGES,
    };
    setChatThreads((prev) => [thread, ...prev]);
    setActiveChatId(id);
    setMessages(thread.messages);
    setTypingMessageIndex(null);
    setDisplayedText('');
    setShowSettings(false);
    setShowHistory(false);
    setTimeout(() => inputRef.current?.focus(), 50);
  }, []);

  const deleteChat = useCallback((id: string) => {
    setChatThreads((prev) => {
      const next = prev.filter((t) => t.id !== id);
      if (next.length === 0) {
        const now = Date.now();
        const fresh: ChatThread = { id: makeChatId(), title: 'Yangi chat', createdAt: now, updatedAt: now, messages: DEFAULT_MESSAGES };
        setActiveChatId(fresh.id);
        setMessages(fresh.messages);
        return [fresh];
      }

      if (activeChatId === id) {
        const fallback = next[0];
        setActiveChatId(fallback.id);
        setMessages((fallback.messages?.length ? fallback.messages : DEFAULT_MESSAGES).slice(-MAX_SAVED_MESSAGES));
        setTypingMessageIndex(null);
        setDisplayedText('');
      }
      return next;
    });
  }, [activeChatId]);

  const appendMessage = useCallback((msg: ChatMessage, opts?: { startTyping?: boolean }) => {
    setMessages((prev) => {
      const next = [...prev, msg].slice(-MAX_SAVED_MESSAGES);
      setChatThreads((threadsPrev) => threadsPrev.map((t) => {
        if (t.id !== activeChatId) return t;
        const nextTitle = t.title && t.title !== 'Yangi chat' ? t.title : deriveChatTitle(next);
        return {
          ...t,
          title: nextTitle,
          updatedAt: Date.now(),
          messages: next,
        };
      }));

      if (opts?.startTyping && msg.role === 'model') {
        setTypingMessageIndex(next.length - 1);
      }
      return next;
    });
  }, [activeChatId]);

  const appendUser = useCallback((text: string) => {
    appendMessage({ role: 'user', text });
  }, [appendMessage]);

  const appendModel = useCallback((text: string, opts?: { startTyping?: boolean }) => {
    appendMessage({ role: 'model', text }, opts);
  }, [appendMessage]);

  const formatChatTime = useCallback((ts: number) => {
    try {
      return new Date(ts).toLocaleString(undefined, { month: 'short', day: '2-digit', hour: '2-digit', minute: '2-digit' });
    } catch {
      return '';
    }
  }, []);

  // Close on Escape for better UX.
  useEffect(() => {
    if (!isOpen) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [isOpen, onClose]);

  const isTextFile = (file: File) => {
    const textTypes = ['text/', 'application/json', 'application/csv', 'application/xml'];
    const textExts = ['.txt', '.csv', '.json', '.md', '.xml', '.yaml', '.yml', '.tsv'];
    return textTypes.some((t) => file.type.startsWith(t)) || textExts.some((ext) => file.name.toLowerCase().endsWith(ext));
  };

  const isExcelFile = (file: File) => {
    const excelTypes = ['application/vnd.openxmlformats-officedocument.spreadsheetml.sheet', 'application/vnd.ms-excel'];
    const excelExts = ['.xlsx', '.xls'];
    return excelTypes.some((t) => file.type === t) || excelExts.some((ext) => file.name.toLowerCase().endsWith(ext));
  };

  // Convert Excel file to CSV text for Gemini
  const convertExcelToText = async (file: File): Promise<string> => {
    try {
      // Use the existing convertExcel API to convert to CSV
      const { blob } = await convertExcel(file, authToken || undefined);

      // Read the CSV blob as text
      const text = await blob.text();
      return text;
    } catch (error) {
      console.error('Failed to convert Excel:', error);
      throw new Error('Excel faylni matn formatiga o\'tkazishda xato');
    }
  };

  useEffect(() => {
    if (!apiKey.trim()) {
      setApiKeyValid(false);
      return;
    }
    const savedKey = typeof window !== 'undefined' ? localStorage.getItem(API_KEY_STORAGE) : null;
    if (apiKeyValid && savedKey && apiKey.trim() !== savedKey) {
      setApiKeyValid(false);
    }
  }, [apiKey, apiKeyValid]);

  useEffect(() => {
    if (!showSettings) {
      const t = setTimeout(() => inputRef.current?.focus(), 50);
      return () => clearTimeout(t);
    }
  }, [showSettings]);

  // Load saved Gemini API key from backend for this user (if available)
  useEffect(() => {
    if (!authToken) return;
    let cancelled = false;
    setServerKeyLoading(true);
    (async () => {
      try {
        const res = await getGeminiApiKey(authToken);
        if (cancelled) return;
        const key = res.gemini_api_key || '';
        if (key) {
          setApiKey(key);
          setApiKeyValid(true);
          setShowSettings(false);
          localStorage.setItem(API_KEY_STORAGE, key);
          setTimeout(() => inputRef.current?.focus(), 50);
        }
      } catch {
        // ignore; fallback to local storage
      } finally {
        if (!cancelled) setServerKeyLoading(false);
      }
    })();

    return () => { cancelled = true; };
  }, [authToken]);

  // Auto-scroll to bottom when messages change
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, loading]);

  // Typing animation effect for AI messages
  useEffect(() => {
    if (typingMessageIndex === null) return;

    const message = messages[typingMessageIndex];
    if (!message || message.role !== 'model') {
      setTypingMessageIndex(null);
      return;
    }

    const fullText = message.text;
    let currentIndex = 0;

    const typingInterval = setInterval(() => {
      if (currentIndex <= fullText.length) {
        setDisplayedText(fullText.slice(0, currentIndex));
        currentIndex++;
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
      } else {
        clearInterval(typingInterval);
        setTypingMessageIndex(null);
        setDisplayedText('');
      }
    }, 20); // Speed of typing (20ms per character)

    return () => clearInterval(typingInterval);
  }, [typingMessageIndex, messages]);

  const handleActivateApiKey = async () => {
    if (!apiKey.trim()) {
      setApiKeyError('API kalitini kiriting');
      return;
    }

    setApiKeyValidating(true);
    setApiKeyError(null);

    try {
      // Test the API key by making a simple request
      const { GoogleGenAI } = await import('@google/genai');
      const ai = new GoogleGenAI({ apiKey: apiKey.trim() });

      // Make a minimal test request
      await ai.models.generateContent({
        model: 'gemini-2.5-flash',
        contents: [{ role: 'user', parts: [{ text: 'ping' }] }],
      });

      // If we get here, the API key is valid
      setApiKeyValid(true);
      setApiKeyError(null);
      localStorage.setItem(API_KEY_STORAGE, apiKey.trim());

      if (authToken) {
        try {
          await saveGeminiApiKey(authToken, apiKey.trim());
        } catch (saveErr: any) {
          // Keep key in memory/localStorage even if server save fails
          console.error('Failed to store Gemini key on server:', saveErr);
          setApiKeyError('Kalit tasdiqlandi, lekin serverga saqlashda xato. Internetni tekshiring.');
        }
      }

      // Auto-switch to chat after validation
      setTimeout(() => {
        setShowSettings(false);
        setTimeout(() => inputRef.current?.focus(), 50);
      }, 1200);
    } catch (err: any) {
      setApiKeyValid(false);
      localStorage.removeItem(API_KEY_STORAGE);
      setApiKeyError(err?.message || "API kaliti tasdiqlanmadi. Kalitni tekshirib qayta urinib ko'ring.");
    } finally {
      setApiKeyValidating(false);
    }
  };

  const handleStop = () => {
    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
      abortControllerRef.current = null;
      setLoading(false);
      appendModel('Generatsiya to\'xtatildi.');
    }
  };

  const handleSend = async () => {
    if (!input.trim() && !attachedContent) return;
    if (fileLoading) {
      appendModel('Fayl yuklanmoqda, biroz kuting.');
      return;
    }

    const userMsg = input;
    const currentAttachedContent = attachedContent;
    const currentAttachedName = attachedName;

    // Clear input and file attachment immediately for better UX
    setInput('');
    setAttachedName(null);
    setAttachedContent(null);
    setAttachedPreview(null);

    const displayMsg = currentAttachedName ? `${userMsg || 'matn kiritilmagan'}\n[Fayl: ${currentAttachedName}]` : userMsg;
    appendUser(displayMsg);
    setLoading(true);

    // Create new AbortController for this request
    const abortController = new AbortController();
    abortControllerRef.current = abortController;

    const fallbackApiKey = '';
    const effectiveApiKey = (apiKeyValid ? apiKey.trim() : '') || fallbackApiKey;

    if (!effectiveApiKey) {
      setLoading(false);
      abortControllerRef.current = null;
      setApiKeyError('Gemini API kalitini kiriting va aktivlashtiring.');
      setShowSettings(true);
      appendModel('AI ishlashi uchun avval Gemini API kalitini sozlamalardan kiriting.');
      return;
    }

    try {
      // Use user's API key if available, otherwise fallback to env variable
      const { GoogleGenAI } = await import('@google/genai');
      const ai = new GoogleGenAI({ apiKey: effectiveApiKey });

      const sheetContext = buildAiSheetContext(sheetState, displayMsg);
      const contextData = `SHEET CONTEXT (non-empty cells only; empty cells omitted):\n${sheetContext.contextText}`;

      // Get selected cell context
      const selectedCellInfo = sheetState.activeCell
        ? `Selected Cell: ${getColumnLabel(sheetState.activeCell.col)}${sheetState.activeCell.row + 1} (Value: ${sheetState.data[getCellId(sheetState.activeCell.row, sheetState.activeCell.col)]?.value || 'empty'})`
        : 'No cell selected';

      // Build conversation history (last 6 messages for context)
      const contextMessages = [...messages, { role: 'user' as const, text: displayMsg }].slice(-6);
      const recentHistory = contextMessages.map(m => `${m.role === 'user' ? 'User' : 'AI'}: ${m.text}`).join('\n');
      const conversationContext = recentHistory ? `\n\nRECENT CONVERSATION:\n${recentHistory}\n` : '';

      const systemPrompt = `
        You are an expert spreadsheet assistant with deep knowledge of Excel/Google Sheets formulas, data analysis, and visualization.
        You can analyze images, PDFs, and documents to extract data and provide insights.

        CURRENT CONTEXT:
        - ${selectedCellInfo}
        - Total non-empty cells: ${sheetContext.totalNonEmpty}
        - Used range: ${sheetContext.usedRangeA1}
        - Used range indices (0-based): ${sheetContext.usedBounds ? `rows ${sheetContext.usedBounds.minRow}..${sheetContext.usedBounds.maxRow}, cols ${sheetContext.usedBounds.minCol}..${sheetContext.usedBounds.maxCol}` : '(empty)'}
        - Selection: ${sheetContext.selectionA1 || '(none)'}
        - Selection indices (0-based): ${sheetContext.selectionBounds ? `rows ${sheetContext.selectionBounds.minRow}..${sheetContext.selectionBounds.maxRow}, cols ${sheetContext.selectionBounds.minCol}..${sheetContext.selectionBounds.maxCol}` : '(none)'}
        - Header preview (row ${sheetContext.headerRowIndex ?? 'n/a'}): ${sheetContext.headerPreview || '(none)'}
        - Explicit refs from user message: ${sheetContext.explicitCellsA1.length || sheetContext.explicitRangesA1.length
          ? `cells(${sheetContext.explicitCellsA1.length}): ${sheetContext.explicitCellsA1.slice(0, 12).join(', ')}${sheetContext.explicitCellsA1.length > 12 ? ', …' : ''}; ranges(${sheetContext.explicitRangesA1.length}): ${sheetContext.explicitRangesA1.slice(0, 6).join(', ')}${sheetContext.explicitRangesA1.length > 6 ? ', …' : ''}`
          : '(none)'}
        - Context included cells: ${sheetContext.includedCells} ${sheetContext.truncated ? '(TRUNCATED)' : ''}

        SCHEMA SUMMARY:
        ${sheetContext.schemaText}

        Context: ${contextData}
        ${conversationContext}

        LANGUAGE: ${selectedLanguage === 'uzbek' ? "O'zbek (Uzbek)" : selectedLanguage === 'russian' ? 'Русский (Russian)' : 'English'}

        FILE CAPABILITIES:
        - Extract tables from images and import to sheet
        - Analyze charts/graphs and provide insights
        - Read Excel/CSV/JSON files and process data (Excel files are auto-converted to CSV)
        - OCR text from images if needed

        OUTPUT RULES:
        - Always respond with a single valid JSON object only (no markdown, no backticks).
        - Put any explanation inside "message". Do not include extra text outside JSON.
        - Never rewrite the whole sheet. Prefer minimal structured operations.
        - If the context is TRUNCATED or insufficient, ask the user to select a smaller range.

        ACTIONS (0-based row/col indices):
        1) Update specific cells:
           {"action":"update_cells","cells":[{"row":0,"col":0,"value":"Data"}],"message":"Done"}
        2) Clear a range:
           {"action":"clear_range","startRow":0,"endRow":10,"startCol":0,"endCol":5,"message":"Cleared"}
        3) Sort a range (rearrange, no rewrite):
           {"action":"sort_range","startRow":0,"endRow":100,"startCol":0,"endCol":5,"sortCol":2,"direction":"asc","hasHeader":true,"message":"Sorted"}
        4) Delete rows/cols (shifts the sheet):
           {"action":"delete_rows","rows":[3,4,10],"message":"Deleted rows"}
           {"action":"delete_cols","cols":[0,2],"message":"Deleted columns"}
        5) Multiple steps:
           {"actions":[{...},{...}],"message":"Done"}
        6) No-op:
           {"action":"noop","message":"What I need from you..."}

        CONSTRAINTS:
        - Keep "cells" <= 300. If more is needed, use sort/delete/clear actions or ask for a smaller selection.
      `;

      // Build parts array for multimodal content
      const parts: any[] = [{ text: systemPrompt + '\n\nUser Request: ' + userMsg }];

      // Add file attachment if present (use saved references)
      if (currentAttachedContent && currentAttachedName) {
        if (currentAttachedContent.startsWith('data:')) {
          // This is a base64 data URL (image or binary file)
          const [mimeTypePart, base64Data] = currentAttachedContent.split(',');
          const mimeType = mimeTypePart.match(/:(.*?);/)?.[1] || 'application/octet-stream';

          parts.push({
            inlineData: {
              mimeType: mimeType,
              data: base64Data
            }
          });
        } else {
          // This is plain text content
          parts.push({ text: `\n\nAttached file "${currentAttachedName}" content:\n${currentAttachedContent}` });
        }
      }

      const response = await ai.models.generateContent({
        model: 'gemini-2.5-flash',
        contents: {
          role: 'user',
          parts: parts
        },
        config: {
          abortSignal: abortController.signal,
          responseMimeType: 'application/json',
          temperature: 0.2,
        },
      });

      // Check if aborted after API call
      if (!abortControllerRef.current) {
        return; // Request was aborted, don't process response
      }

      const text = response.text.trim();

      // Clear attachment after sending
      setAttachedContent(null);
      setAttachedName(null);
      setAttachedPreview(null);

      // Check again before updating state
      if (!abortControllerRef.current) {
        return;
      }

      // Attempt to parse JSON
      try {
        const parseJson = (raw: string): any | null => {
          const trimmed = raw.trim();
          if (!trimmed) return null;
          try {
            return JSON.parse(trimmed);
          } catch {
            // fallback: extract first object/array block
            const objMatch = trimmed.match(/\{[\s\S]*\}/);
            if (objMatch) {
              try {
                return JSON.parse(objMatch[0]);
              } catch {
                // ignore
              }
            }
            const arrMatch = trimmed.match(/\[[\s\S]*\]/);
            if (arrMatch) {
              try {
                return JSON.parse(arrMatch[0]);
              } catch {
                // ignore
              }
            }
            return null;
          }
        };

        const parsed = parseJson(text);
        const envelope: { actions: AiAction[]; message?: string } | null = (() => {
          if (!parsed) return null;
          if (Array.isArray(parsed)) {
            const actions = parsed.filter((a) => a && typeof a === 'object' && typeof a.action === 'string');
            return actions.length ? { actions } : null;
          }
          if (typeof parsed !== 'object') return null;
          if (Array.isArray((parsed as any).actions)) {
            const actions = (parsed as any).actions.filter((a: any) => a && typeof a === 'object' && typeof a.action === 'string');
            const msg = typeof (parsed as any).message === 'string' ? (parsed as any).message : undefined;
            return actions.length ? { actions, message: msg } : null;
          }
          if (typeof (parsed as any).action === 'string') {
            const msg = typeof (parsed as any).message === 'string' ? (parsed as any).message : undefined;
            return { actions: [parsed as AiAction], message: msg };
          }
          return null;
        })();

        if (!envelope) {
          appendModel(text, { startTyping: true });
          return;
        }

        if (!abortControllerRef.current) return;

        const MAX_AI_EDITS = 300;
        const actionMessages: string[] = [];

        for (const action of envelope.actions) {
          if (!abortControllerRef.current) return;

          if (action.action === 'update_cells' && Array.isArray((action as any).cells)) {
            const edits: AiCellEdit[] = [];
            (action as any).cells.forEach((cell: any) => {
              const row = Number(cell?.row);
              const col = Number(cell?.col);
              if (!Number.isFinite(row) || !Number.isFinite(col) || row < 0 || col < 0) return;
              const value = (cell?.value ?? '').toString();
              edits.push({ row, col, value });
            });

            if (edits.length > MAX_AI_EDITS) {
              appendModel(`Juda ko'p o'zgarish (${edits.length}). Avval range tanlang yoki "sort/delete/clear" kabi amallarni ishlating.`, { startTyping: true });
              return;
            }

            if (edits.length > 0) {
              onApplyChanges(edits);
            }
            if (typeof (action as any).message === 'string' && (action as any).message.trim()) {
              actionMessages.push((action as any).message.trim());
            }
            continue;
          }

          if (action.action === 'noop') {
            if (typeof (action as any).message === 'string' && (action as any).message.trim()) {
              actionMessages.push((action as any).message.trim());
            }
            continue;
          }

          if (onApplyAiAction) {
            onApplyAiAction(action);
            if (typeof (action as any).message === 'string' && (action as any).message.trim()) {
              actionMessages.push((action as any).message.trim());
            }
          } else {
            actionMessages.push(`(AI action unsupported: ${action.action})`);
          }
        }

        const finalMessage = (envelope.message && envelope.message.trim())
          ? envelope.message.trim()
          : (actionMessages.find(Boolean) || 'Bajarildi.');
        appendModel(finalMessage, { startTyping: true });
      } catch (e) {
        if (!abortControllerRef.current) return; // Don't show error if aborted
        appendModel(text, { startTyping: true });
      }

    } catch (err: any) {
      // Check if it was aborted
      if (err?.name === 'AbortError' || abortControllerRef.current === null) {
        // Already handled in handleStop
        return;
      }

      appendModel("Uzr, so'rovni bajarishda xatolik yuz berdi.", { startTyping: true });
      console.error(err);
    } finally {
      setLoading(false);
      abortControllerRef.current = null;
    }
  };

  if (!isOpen) {
    return null;
  }

  return (
    <>
	      <div
	        className={`relative flex flex-col h-full w-96 transition-all duration-300 ease-out ${isOpen ? 'translate-x-0 opacity-100' : 'translate-x-full opacity-0'}`}
	        style={{
	          borderLeft: '1px solid var(--border-color)',
	          background: 'var(--card-bg)',
	          boxShadow: '-4px 0 20px rgba(0, 0, 0, 0.12)'
	        }}
	      >
	        <div
	          className="flex items-center justify-between px-4 py-3 border-b"
	          style={{ borderColor: 'var(--border-color)', background: 'var(--card-bg)' }}
	        >
	          <div className="flex items-center gap-2.5">
	            <div>
	              <h3 className="font-semibold text-base" style={{ color: 'var(--text-primary)' }}>
	                AI yordamchi
	              </h3>
	              <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
	                {activeChatTitle} · Gemini
	              </p>
	            </div>
	          </div>
	          <div className="flex items-center gap-1">
	            <button
	              onClick={startNewChat}
	              className="p-1.5 rounded hover:opacity-80 transition-opacity"
	              style={{ background: 'transparent', color: 'var(--text-primary)' }}
	              title="Yangi chat"
	            >
	              <MessageSquarePlus size={18} />
	            </button>
	            <button
	              onClick={() => {
	                setShowHistory((v) => !v);
	                setShowSettings(false);
	              }}
	              className="p-1.5 rounded hover:opacity-80 transition-opacity"
	              style={{ background: showHistory ? 'var(--bg-light)' : 'transparent', color: 'var(--text-primary)' }}
	              title="Chatlar tarixi"
	            >
	              <History size={18} />
	            </button>
	            <button
	              onClick={() => {
	                setShowSettings((v) => !v);
	                setShowHistory(false);
	              }}
	              className="p-1.5 rounded hover:opacity-80 transition-opacity"
	              style={{ background: showSettings ? 'var(--bg-light)' : 'transparent', color: 'var(--text-primary)' }}
	              title="Sozlamalar"
	            >
	              <Settings size={18} />
	            </button>
	            <button
	              onClick={onClose}
	              className="p-1.5 rounded hover:opacity-80 transition-opacity"
	              style={{ background: 'transparent', color: 'var(--text-primary)' }}
	              title="Yopish"
	            >
	              <X size={18} />
	            </button>
	          </div>
	        </div>

	        {!showSettings && historyPresence.isMounted && (
	          <>
	            <div
	              className="absolute inset-0 z-30 ui-overlay"
	              data-state={historyPresence.state}
	              style={{ background: 'rgba(0,0,0,0.25)' }}
	              onClick={() => setShowHistory(false)}
	            />
	            <div
	              className="absolute left-3 right-3 top-14 z-40 rounded-lg shadow-xl ui-popover overflow-hidden"
	              data-state={historyPresence.state}
	              style={{
	                background: 'var(--card-bg)',
	                border: '1px solid var(--border-color)',
	                transformOrigin: 'top right',
	              }}
	              onClick={(e) => e.stopPropagation()}
	            >
	              <div className="px-3 py-2 flex items-center justify-between border-b" style={{ borderColor: 'var(--border-color)' }}>
	                <div className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
	                  Chatlar
	                </div>
	                <button
	                  type="button"
	                  onClick={() => setShowHistory(false)}
	                  className="p-1 rounded hover:opacity-80 transition-opacity"
	                  style={{ color: 'var(--text-secondary)' }}
	                  title="Yopish"
	                >
	                  <X size={16} />
	                </button>
	              </div>

	              <div className="p-3">
	                <button
	                  type="button"
	                  onClick={startNewChat}
	                  className="w-full inline-flex items-center justify-center gap-2 px-3 py-2 rounded border text-sm hover:opacity-80 transition-opacity"
	                  style={{ borderColor: 'var(--border-color)', background: 'var(--bg-light)', color: 'var(--text-primary)' }}
	                >
	                  <MessageSquarePlus size={16} />
	                  Yangi chat
	                </button>
	              </div>

	              <div className="max-h-80 overflow-y-auto px-2 pb-2">
	                {sortedThreads.map((t) => {
	                  const last = t.messages?.[t.messages.length - 1]?.text || '';
	                  const preview = last.split('\n')[0]?.trim() || '';
	                  const shortPreview = preview.length > 46 ? `${preview.slice(0, 46)}…` : preview;
	                  const isActive = t.id === activeChatId;
	                  return (
	                    <div
	                      key={t.id}
	                      className="flex items-start gap-2 rounded-md px-2 py-2"
	                      style={{
	                        background: isActive ? 'var(--bg-light)' : 'transparent',
	                        border: '1px solid',
	                        borderColor: isActive ? 'var(--sheet-selection-border)' : 'transparent',
	                      }}
	                    >
	                      <button
	                        type="button"
	                        className="flex-1 text-left rounded"
	                        onClick={() => switchToChat(t.id)}
	                        style={{ color: 'var(--text-primary)' }}
	                      >
	                        <div className="text-sm font-medium truncate">{t.title || 'Chat'}</div>
	                        {shortPreview && (
	                          <div className="text-xs truncate mt-0.5" style={{ color: 'var(--text-secondary)' }}>
	                            {shortPreview}
	                          </div>
	                        )}
	                        <div className="text-[11px] mt-1" style={{ color: 'var(--text-secondary)' }}>
	                          {formatChatTime(t.updatedAt || t.createdAt)}
	                        </div>
	                      </button>
	                      <button
	                        type="button"
	                        onClick={() => deleteChat(t.id)}
	                        className="p-1.5 rounded hover:opacity-80 transition-opacity"
	                        style={{ color: 'var(--text-secondary)' }}
	                        title="O‘chirish"
	                      >
	                        <Trash2 size={14} />
	                      </button>
	                    </div>
	                  );
	                })}
	              </div>
	            </div>
	          </>
	        )}

        {showSettings ? (
          <div className="flex-1 overflow-y-auto p-4" style={{ background: 'var(--bg-light)' }}>
            <div className="space-y-4">
              <div>
                <h3 className="text-lg font-semibold mb-2" style={{ color: 'var(--text-primary)' }}>AI sozlamalari</h3>
                <p className="text-sm mb-4" style={{ color: 'var(--text-secondary)' }}>
                  Gemini ni faollashtirish uchun API kalitingizni kiriting. Har bir foydalanuvchi o'z kalitidan foydalana oladi.
                </p>
              </div>

              <div className="space-y-3">
                <div>
                  <label className="block text-sm font-medium mb-2" style={{ color: 'var(--text-primary)' }}>
                    Gemini API kaliti
                  </label>
                  <input
                    type="password"
                    value={apiKey}
                    onChange={(e) => setApiKey(e.target.value)}
                    placeholder="Gemini API kalitini kiriting"
                    className="w-full px-3 py-2 rounded text-sm focus:outline-none"
                    style={{
                      border: '1px solid var(--border-color)',
                      background: 'var(--card-bg)',
                      color: 'var(--text-primary)',
                    }}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        handleActivateApiKey();
                      }
                    }}
                  />
                </div>

                {apiKeyError && (
                  <div
                    className="text-sm p-2 rounded border"
                    style={{ background: 'var(--sheet-danger-fill)', borderColor: 'var(--sheet-danger-border)', color: 'var(--text-primary)' }}
                  >
                    {apiKeyError}
                  </div>
                )}

                {apiKeyValid && (
                  <div
                    className="text-sm p-2 rounded flex items-center gap-2 border"
                    style={{ background: 'rgba(34, 197, 94, 0.16)', borderColor: 'rgba(34, 197, 94, 0.45)', color: 'var(--text-primary)' }}
                  >
                    <Check size={16} style={{ color: '#22c55e' }} />
                    <span>API kaliti tasdiqlandi! Chatga o'tmoqdaman...</span>
                  </div>
                )}

                {serverKeyLoading && (
                  <div className="text-sm p-2 rounded flex items-center gap-2" style={{ background: 'var(--bg-light)', color: 'var(--text-primary)', border: '1px dashed var(--border-color)' }}>
                    <Loader2 size={16} className="animate-spin" />
                    <span>Saqlangan kalit yuklanmoqda...</span>
                  </div>
                )}

                <button
                  onClick={handleActivateApiKey}
                  disabled={apiKeyValidating || !apiKey.trim()}
                  className="w-full py-2 px-4 rounded font-medium text-white flex items-center justify-center gap-2 hover:opacity-90 disabled:opacity-50 transition-opacity"
                  style={{ background: 'var(--primary)' }}
                >
                  {apiKeyValidating ? (
                    <>
                      <Loader2 size={16} className="animate-spin" />
                      <span>Tekshirilmoqda...</span>
                    </>
                  ) : (
                    <span>Aktivlashtirish</span>
                  )}
                </button>
              </div>

              {/* Language Selection */}
              <div className="space-y-3 pt-4 border-t" style={{ borderColor: 'var(--border-color)' }}>
                <div>
                  <label className="block text-sm font-medium mb-2" style={{ color: 'var(--text-primary)' }}>
                    AI javob tili
                  </label>
                  <select
                    value={selectedLanguage}
                    onChange={(e) => {
                      setSelectedLanguage(e.target.value);
                      localStorage.setItem('ai-language', e.target.value);
                    }}
                    className="w-full px-3 py-2 rounded text-sm focus:outline-none"
                    style={{
                      border: '1px solid var(--border-color)',
                      background: 'var(--card-bg)',
                      color: 'var(--text-primary)',
                    }}
                  >
                    <option value="uzbek">O'zbek tili (Uzbek)</option>
                    <option value="english">English</option>
                    <option value="russian">Русский (Russian)</option>
                  </select>
                  <p className="text-xs mt-1" style={{ color: 'var(--text-secondary)' }}>
                    AI shu tilda javob beradi
                  </p>
                </div>
              </div>

              <div className="mt-6 p-3 rounded text-xs space-y-2" style={{ background: 'var(--card-bg)', border: '1px solid var(--border-color)', color: 'var(--text-secondary)' }}>
                <p className="font-medium" style={{ color: 'var(--text-primary)' }}>API kalitini olish tartibi:</p>
                <ol className="list-decimal list-inside space-y-1">
                  <li>Google AI Studio sahifasiga kiring</li>
                  <li>Google akkauntingiz bilan kiring</li>
                  <li>Yangi API kalit yarating</li>
                  <li>Kalitni bu yerga nusxa ko'chirib qo'ying</li>
                </ol>
              </div>
            </div>
          </div>
        ) : (
          <>
            {/* Messages Section */}
            <div className="flex-1 overflow-y-auto p-4 space-y-4" style={{ background: 'var(--bg-light)' }}>
              {/* File Error Display */}
              {fileError && (
                <div
                  className="text-xs p-2 rounded border animate-fadeIn"
                  style={{ background: 'var(--sheet-danger-fill)', borderColor: 'var(--sheet-danger-border)', color: 'var(--text-primary)' }}
                >
                  {fileError}
                </div>
              )}

              {/* Messages with Modern Avatars */}
              {messages.map((msg, i) => {
                const isTyping = typingMessageIndex === i;
                const textToShow = isTyping ? displayedText : msg.text;

                return (
                  <div
                    key={i}
                    className={`flex gap-3 animate-chatBubbleIn ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}
                  >
                    <div
                      className={`rounded-xl px-4 py-2.5 max-w-[75%] ${msg.role === 'user' ? 'ml-auto' : ''}`}
                      style={{
                        background: msg.role === 'user'
                          ? 'var(--chat-user-bubble)'
                          : 'var(--card-bg)',
                        color: msg.role === 'user' ? 'var(--chat-user-text)' : 'var(--text-primary)',
                        textShadow: msg.role === 'user' ? '0 1px 1px rgba(0,0,0,0.25)' : 'none',
                        border: msg.role === 'model' ? '1px solid var(--border-color)' : 'none',
                        boxShadow: msg.role === 'user'
                          ? 'var(--shadow-md)'
                          : 'var(--shadow-sm)'
                      }}
                    >
                      <p className="text-sm whitespace-pre-wrap leading-relaxed">
                        {textToShow}
                        {isTyping && <span className="animate-pulse">|</span>}
                      </p>
                    </div>
                  </div>
                );
              })}
              {loading && (
                <div className="flex gap-3">
                  <div
                    className="rounded-xl px-4 py-2.5"
                    style={{
                      background: 'var(--card-bg)',
                      border: '1px solid var(--border-color)'
                    }}
                  >
                    <div className="flex gap-1">
                      <style>{`
                        @keyframes dotPulse {
                          0%, 80%, 100% { opacity: 0.3; }
                          40% { opacity: 1; }
                        }
                      `}</style>
                      <div
                        style={{
                          width: '6px',
                          height: '6px',
                          borderRadius: '50%',
                          backgroundColor: 'var(--text-secondary)',
                          animation: 'dotPulse 1.4s infinite',
                          animationDelay: '0s'
                        }}
                      />
                      <div
                        style={{
                          width: '6px',
                          height: '6px',
                          borderRadius: '50%',
                          backgroundColor: 'var(--text-secondary)',
                          animation: 'dotPulse 1.4s infinite',
                          animationDelay: '0.2s'
                        }}
                      />
                      <div
                        style={{
                          width: '6px',
                          height: '6px',
                          borderRadius: '50%',
                          backgroundColor: 'var(--text-secondary)',
                          animation: 'dotPulse 1.4s infinite',
                          animationDelay: '0.4s'
                        }}
                      />
                    </div>
                  </div>
                </div>
              )}
              {/* Invisible element to scroll to */}
              <div ref={messagesEndRef} />
            </div>

            {/* Quick Actions */}
            <div className="px-3 py-2 flex gap-2 overflow-x-auto" style={{ background: 'var(--bg-light)', borderTop: '1px solid var(--border-color)' }}>
              {[
                { label: 'Sum', icon: '∑', prompt: 'Sum the selected column/row' },
                { label: 'Average', icon: '≈', prompt: 'Calculate average of selected range' },
                { label: 'Sort ↓', icon: '↓', prompt: 'Sort data in descending order' },
                { label: 'Analyze', icon: '📊', prompt: 'Analyze this data and show insights' },
                { label: 'Clean', icon: '✨', prompt: 'Clean and fix data formatting' },
                { label: 'Fill', icon: '⬇', prompt: 'Fill down this pattern' },
              ].map((action) => (
                <button
                  key={action.label}
                  onClick={() => {
                    setInput(action.prompt);
                    setTimeout(() => inputRef.current?.focus(), 50);
                  }}
                  className="flex-shrink-0 px-3 py-1.5 rounded-lg text-xs font-medium border hover:opacity-80 transition-opacity"
                  style={{
                    background: 'var(--card-bg)',
                    borderColor: 'var(--border-color)',
                    color: 'var(--text-primary)'
                  }}
                  title={action.prompt}
                >
                  {action.icon} {action.label}
                </button>
              ))}
            </div>

            {/* Input Area */}
            <div className="p-3" style={{ background: 'var(--card-bg)', borderTop: '1px solid var(--border-color)' }}>
              {/* File Preview - shown above message bar when file is attached */}
              {attachedName && (
                <div className="mb-2 animate-fadeIn" style={{ animation: 'slideDown 0.3s ease-out' }}>
                  <style>{`
                    @keyframes slideDown {
                      from {
                        opacity: 0;
                        transform: translateY(-10px);
                      }
                      to {
                        opacity: 1;
                        transform: translateY(0);
                      }
                    }
                  `}</style>
                  <div
                    className="flex items-center gap-2 p-2 rounded-md"
                    style={{
                      background: 'var(--bg-light)',
                      border: '1px solid var(--border-color)',
                    }}
                  >
                    <div
                      className="flex items-center justify-center rounded"
                      style={{
                        width: '32px',
                        height: '32px',
                        background: 'var(--brand)',
                        color: 'white',
                      }}
                    >
                      <Paperclip size={16} />
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium truncate" style={{ color: 'var(--text-primary)' }}>
                        {attachedName}
                      </p>
                      <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                        {fileLoading ? 'Yuklanmoqda...' : 'Tayyor'}
                      </p>
                    </div>
                    <button
                      onClick={() => {
                        setAttachedName(null);
                        setAttachedContent(null);
                        setAttachedPreview(null);
                        setFileError(null);
                      }}
                      className="p-1 rounded hover:opacity-70 transition-opacity"
                      style={{ color: 'var(--text-secondary)' }}
                      title="Faylni olib tashlash"
                    >
                      <X size={16} />
                    </button>
                  </div>
                </div>
              )}

              <div className="relative">
                <textarea
                  ref={inputRef}
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && !e.shiftKey) {
                      e.preventDefault();
                      handleSend();
                    }
                  }}
                  placeholder="AI dan ma'lumot yoki formula so'rang..."
                  className="w-full rounded-md pl-3 pr-28 py-2 text-sm focus:outline-none resize-none h-20"
                  style={{ border: '1px solid var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
                />
                <div className="absolute bottom-2 right-2 flex items-center gap-1">
                  {/* File Attachment Button */}
                  <input
                    id="gemini-message-file-input"
                    type="file"
                    className="hidden"
                    accept="*/*"
                    onChange={async (e) => {
                      setFileError(null);
                      const file = e.target.files?.[0];
                      if (!file) return;
                      if (file.size > 300 * 1024 * 1024) {
                        setFileError('Fayl juda katta (max 300MB).');
                        return;
                      }

                      setFileLoading(true);

                      try {
                        // Handle Excel files specially - convert to CSV text
                        if (isExcelFile(file)) {
                          const csvText = await convertExcelToText(file);
                          setAttachedContent(csvText);
                          setAttachedPreview(null);
                          setAttachedName(file.name);
                          setFileLoading(false);
                          return;
                        }

                        // Handle other files normally
                        const reader = new FileReader();
                        reader.onload = () => {
                          if (typeof reader.result === 'string') {
                            setAttachedContent(reader.result);
                            setAttachedPreview(reader.result.startsWith('data:') ? reader.result : null);
                          } else {
                            setAttachedContent('');
                          }
                          setAttachedName(file.name);
                          setFileLoading(false);
                        };
                        reader.onerror = () => { setFileError('Faylni o\'qishda xato'); setFileLoading(false); };
                        if (isTextFile(file)) {
                          reader.readAsText(file);
                        } else {
                          reader.readAsDataURL(file);
                        }
                      } catch (err: any) {
                        setFileError(err?.message || 'Faylni o\'qishda xato');
                        setFileLoading(false);
                      }

                      // Reset input so same file can be selected again
                      e.target.value = '';
                    }}
                  />
                  <button
                    onClick={() => document.getElementById('gemini-message-file-input')?.click()}
                    className="p-1.5 rounded hover:opacity-80 transition-opacity"
                    style={{ background: 'var(--bg-light)', border: '1px solid var(--border-color)', color: 'var(--text-primary)' }}
                    title="Fayl biriktirish"
                    disabled={fileLoading}
                  >
                    {fileLoading ? <Loader2 size={16} className="animate-spin" /> : <Paperclip size={16} />}
                  </button>

                  {/* Prompts Dropdown */}
                  <div className="relative">
                    <button
                      onClick={() => setShowPrompts(!showPrompts)}
                      className="p-1.5 rounded hover:opacity-80 transition-opacity"
                      style={{ background: 'var(--bg-light)', border: '1px solid var(--border-color)', color: 'var(--text-primary)' }}
                      title="Quick prompts"
                    >
                      <List size={16} />
                    </button>

                    {promptsPresence.isMounted && (
                      <>
                        <div
                          className="fixed inset-0 z-40 ui-overlay"
                          data-state={promptsPresence.state}
                          onClick={() => setShowPrompts(false)}
                        />
                        <div
                          className="absolute bottom-full right-0 mb-2 w-64 rounded-md shadow-lg z-50 max-h-80 overflow-y-auto ui-popover"
                          data-state={promptsPresence.state}
                          style={{ background: 'var(--card-bg)', border: '1px solid var(--border-color)' }}
                        >
                          <div className="p-2 border-b" style={{ borderColor: 'var(--border-color)' }}>
                            <p className="text-xs font-semibold" style={{ color: 'var(--text-secondary)' }}>QUICK PROMPTS</p>
                          </div>
                          <div className="p-1">
                            {[
                              { icon: '📊', label: 'Budget Template', text: 'Create a monthly budget template' },
                              { icon: '✨', label: 'Mock Data', text: 'Fill A1:A5 with random sales numbers' },
                              { icon: '∑', label: 'Sum Formula', text: 'Add a SUM formula in C10 for column A' },
                              { icon: '📊', label: 'Average', text: 'Calculate average of column B' },
                              { icon: '❓', label: 'IF Condition', text: 'Create an IF formula to check if A1 > 100' },
                              { icon: '🔍', label: 'VLOOKUP', text: 'Add VLOOKUP to find values in table' },
                              { icon: '📈', label: 'Analyze Trends', text: 'Analyze my data and find trends' },
                              { icon: '🧹', label: 'Clean Data', text: 'Find duplicates and missing values' },
                              { icon: '📊', label: 'Chart Suggestion', text: 'Recommend best chart for this data' },
                              { icon: '📉', label: 'Statistics', text: 'Give me statistical summary of my data' },
                            ].map((prompt, idx) => (
                              <button
                                key={idx}
                                onClick={() => {
                                  setInput(prompt.text);
                                  setShowPrompts(false);
                                }}
                                className="w-full text-left px-3 py-2 rounded text-sm hover:opacity-80 transition-all flex items-center gap-2"
                                style={{ background: 'transparent', color: 'var(--text-primary)' }}
                                onMouseEnter={(e) => e.currentTarget.style.background = 'var(--bg-light)'}
                                onMouseLeave={(e) => e.currentTarget.style.background = 'transparent'}
                              >
                                <span>{prompt.icon}</span>
                                <span>{prompt.label}</span>
                              </button>
                            ))}
                          </div>
                        </div>
                      </>
                    )}
                  </div>

                  {loading ? (
                    <button
                      onClick={handleStop}
                      className="p-1.5 text-white rounded hover:opacity-80 transition-opacity"
                      style={{ background: '#ef4444' }}
                      title="Generatsiyani to'xtatish"
                    >
                      <Square size={16} fill="currentColor" />
                    </button>
                  ) : (
                    <button
                      onClick={handleSend}
                      disabled={serverKeyLoading || (!input.trim() && !attachedContent)}
                      className="p-1.5 text-white rounded hover:opacity-80 disabled:opacity-50 transition-opacity"
                      style={{ background: 'var(--primary)' }}
                      title="Yuborish"
                    >
                      <Send size={16} />
                    </button>
                  )}
                </div>
              </div>
            </div>
          </>
        )}
      </div>
    </>
  );
};

export default GeminiSidebar;
