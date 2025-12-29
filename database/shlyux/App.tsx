import React, { useState, useEffect, useCallback, useRef, Suspense, useMemo } from 'react';
import Toolbar from './components/Toolbar';
import FormulaBar, { NAME_BOX_ID } from './components/FormulaBar';
import Grid from './components/Grid';
import AuthWall from './components/AuthWall';
import ContextMenu from './components/ContextMenu';
import Header from './components/Header';
import Profile from './components/Profile';
import DeleteConfirmModal from './components/DeleteConfirmModal';
import OverwriteConfirmModal from './components/OverwriteConfirmModal';
import ErrorToast from './components/ErrorToast';
import FindReplaceModal from './components/FindReplaceModal';
import TemplatePickerModal from './components/TemplatePickerModal';
import ShareModal from './components/ShareModal';
import StatusBar from './components/StatusBar';
import { API_BASE, AuthUser, fetchMe, getFile, getFileRealtimeToken, listFiles, login, register, saveFile, deleteFile, SheetFileMeta, convertExcel } from './utils/api';
import { SheetState, ClipboardData, ContextMenuState, GridData, CellStyle, CellData } from './types';
import { getCellId, getColumnLabel, recomputeSheet, NUM_COLS, NUM_ROWS, cellLabelToCoords } from './utils/spreadsheetUtils';
import { RealtimeClient } from './utils/realtime';

const GeminiSidebar = React.lazy(() => import('./components/GeminiSidebar'));

const ROW_BATCH_SIZE = 100;

// CSV parser utility function (outside component to avoid re-creation)
const parseCsv = (text: string): string[][] => {
  const rows: string[][] = [];
  let current: string[] = [];
  let field = '';
  let inQuotes = false;

  const pushField = () => {
    current.push(field);
    field = '';
  };

  const pushRow = () => {
    rows.push(current);
    current = [];
  };

  for (let i = 0; i < text.length; i++) {
    const char = text[i];
    if (inQuotes) {
      if (char === '"' && text[i + 1] === '"') {
        field += '"';
        i++;
      } else if (char === '"') {
        inQuotes = false;
      } else {
        field += char;
      }
    } else {
      if (char === '"') {
        inQuotes = true;
      } else if (char === ',') {
        pushField();
      } else if (char === '\n') {
        pushField();
        pushRow();
      } else if (char === '\r') {
        continue;
      } else {
        field += char;
      }
    }
  }

  pushField();
  if (current.length) {
    pushRow();
  }
  return rows;
};

const normalizeRowCount = (count?: number) => Math.max(count || 0, NUM_ROWS);

const ensureRowCountForIndex = (count: number, rowIndex: number) => {
  let next = normalizeRowCount(count);
  if (rowIndex < 0 || rowIndex < next) return next;
  const requiredRows = rowIndex + 1;
  const deficit = requiredRows - next;
  const batches = Math.ceil(deficit / ROW_BATCH_SIZE);
  return next + batches * ROW_BATCH_SIZE;
};

const getHighestRowIndex = (data: GridData): number => {
  let max = -1;
  Object.keys(data).forEach(key => {
    const [rowStr] = key.split(',');
    const row = parseInt(rowStr, 10);
    if (!isNaN(row) && row > max) {
      max = row;
    }
  });
  return max;
};

const App: React.FC = () => {
  const TOKEN_KEY = 'sheetmaster-auth-token';
  // Load from localStorage on initial render
  const emptySheet: SheetState = {
    data: {},
    activeCell: { row: 0, col: 0 },
    selection: { start: { row: 0, col: 0 }, end: { row: 0, col: 0 } },
    columnWidths: {},
    rowHeights: {},
    mergedCells: [],
    rowCount: NUM_ROWS,
  };

  const [sheet, setSheet] = useState<SheetState>(() => {
    // Try loading from the new localStorage key first
    const savedNew = localStorage.getItem('sheetmaster-current-sheet');
    if (savedNew) {
      try {
        const parsed = JSON.parse(savedNew) as SheetState;
        const highestRow = getHighestRowIndex(parsed.data || {});
        return {
          ...emptySheet,
          ...parsed,
          rowCount: ensureRowCountForIndex(parsed.rowCount ?? NUM_ROWS, highestRow)
        };
      } catch {
        // If parse fails, try old key
      }
    }

    // Fallback to old key for backward compatibility
    const savedOld = localStorage.getItem('sheetmaster-data');
    if (savedOld) {
      try {
        const parsed = JSON.parse(savedOld) as SheetState;
        const highestRow = getHighestRowIndex(parsed.data || {});
        return {
          ...emptySheet,
          ...parsed,
          rowCount: ensureRowCountForIndex(parsed.rowCount ?? NUM_ROWS, highestRow)
        };
      } catch {
        // If parse fails, return default
      }
    }
    return emptySheet;
  });

  const [formulaValue, setFormulaValue] = useState('');
  const [isAiOpen, setIsAiOpen] = useState(false);
  const [findOpen, setFindOpen] = useState(false);
  const [findMode, setFindMode] = useState<'find' | 'replace'>('find');
  const [findQuery, setFindQuery] = useState('');
  const [findReplaceText, setFindReplaceText] = useState('');
  const [findScope, setFindScope] = useState<'sheet' | 'selection'>('sheet');
  const [findMatchCase, setFindMatchCase] = useState(false);
  const [findWholeCell, setFindWholeCell] = useState(false);
  const [findIndex, setFindIndex] = useState(-1);
  const [templatesOpen, setTemplatesOpen] = useState(false);
  const [editingCell, setEditingCell] = useState<{ row: number, col: number } | null>(null);
  const [clipboard, setClipboard] = useState<ClipboardData | null>(null);
  const clipboardCatcherRef = useRef<HTMLTextAreaElement | null>(null);
  const lastCopiedTextRef = useRef<string | null>(null);
  const [contextMenu, setContextMenu] = useState<ContextMenuState>({ show: false, x: 0, y: 0, row: 0, col: 0 });
  const [formatPainterActive, setFormatPainterActive] = useState(false);
  const copiedFormat = useRef<CellStyle | null>(null);
  const findIndexRef = useRef(findIndex);

  const [token, setToken] = useState<string | null>(() => {
    if (typeof window === 'undefined') return null;
    return localStorage.getItem(TOKEN_KEY);
  });
  const [user, setUser] = useState<AuthUser | null>(null);
  const [authMode, setAuthMode] = useState<'login' | 'register'>('login');
  const [authForm, setAuthForm] = useState({ name: '', email: '', password: '' });
  const [authError, setAuthError] = useState<string | null>(null);
  const [authLoading, setAuthLoading] = useState(false);
  const [files, setFiles] = useState<SheetFileMeta[]>([]);
  const [currentFileId, setCurrentFileId] = useState<number | null>(null);
  const [fileName, setFileName] = useState<string>('Yangi fayl');
  const [currentAccessRole, setCurrentAccessRole] = useState<'owner' | 'editor' | 'viewer'>('owner');
  const [realtimeClient, setRealtimeClient] = useState<RealtimeClient | null>(null);
  const [pendingDeleteId, setPendingDeleteId] = useState<number | null>(null);
  const [overwriteConfirm, setOverwriteConfirm] = useState<{ show: boolean; action: (() => void) | null }>({ show: false, action: null });
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const remoteQueue = useRef<Record<string, string>>({});
  const [showProfile, setShowProfile] = useState(false);
  const [shareOpen, setShareOpen] = useState(false);
  const [uiDensity, setUiDensity] = useState<'comfortable' | 'compact'>(() => {
    if (typeof window === 'undefined') return 'comfortable';
    return localStorage.getItem('app-density') === 'compact' ? 'compact' : 'comfortable';
  });
  const [saveStatus, setSaveStatus] = useState<'idle' | 'saving' | 'saved' | 'error'>('idle');
  const [lastSavedAt, setLastSavedAt] = useState<number | null>(null);
  const saveSeqRef = useRef(0);
  const [realtimeStatus, setRealtimeStatus] = useState<'disconnected' | 'connecting' | 'connected' | 'error'>('disconnected');
  const currentFileIdRef = useRef<number | null>(null);

  useEffect(() => {
    currentFileIdRef.current = currentFileId;
  }, [currentFileId]);

  const ensureWritable = useCallback((actionLabel?: string) => {
    if (currentAccessRole !== 'viewer') return true;
    setErrorMessage(actionLabel ? `Read-only: ${actionLabel} mumkin emas` : "Read-only: tahrirlash mumkin emas");
    return false;
  }, [currentAccessRole]);

  const focusClipboardCatcher = useCallback(() => {
    const el = clipboardCatcherRef.current;
    if (!el) return;
    if (el.value) el.value = '';
    el.focus({ preventScroll: true });
  }, []);

  const writeToSystemClipboard = useCallback(async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      return;
    } catch (err) {
      // Fallback for browsers that block Clipboard API
      try {
        const textarea = document.createElement('textarea');
        textarea.value = text;
        textarea.setAttribute('readonly', 'true');
        textarea.style.position = 'fixed';
        textarea.style.left = '-9999px';
        textarea.style.top = '0';
        textarea.style.opacity = '0';
        document.body.appendChild(textarea);
        textarea.focus({ preventScroll: true });
        textarea.select();
        const ok = document.execCommand('copy');
        document.body.removeChild(textarea);
        if (!ok) throw new Error('document.execCommand(copy) failed');
      } catch (fallbackErr) {
        console.error('Clipboard write failed:', err, fallbackErr);
        setErrorMessage('Clipboardga nusxalab bo‘lmadi (brauzer ruxsat bermadi).');
      }
    }
  }, []);

  // Initialize theme and font from localStorage on mount
  useEffect(() => {
    const theme = localStorage.getItem('app-theme') || 'light';
    const font = localStorage.getItem('app-font') || 'Inter, sans-serif';
    const density = localStorage.getItem('app-density') === 'compact' ? 'compact' : 'comfortable';

    document.documentElement.setAttribute('data-theme', theme);
    document.documentElement.setAttribute('data-density', density);
    document.documentElement.style.setProperty('--font-family', font);
  }, []);

  useEffect(() => {
    document.documentElement.setAttribute('data-density', uiDensity);
  }, [uiDensity]);

  // Auto save state
  const [autoSaveEnabled, setAutoSaveEnabled] = useState<boolean>(() => {
    const saved = localStorage.getItem('sheetmaster-autosave');
    if (saved === null) return true;
    return saved === 'true';
  });

  // Save auto save preference to localStorage
  useEffect(() => {
    localStorage.setItem('sheetmaster-autosave', String(autoSaveEnabled));
  }, [autoSaveEnabled]);

  const remoteRaf = useRef<number | null>(null);

  const activeSaveRequests = useRef<Set<Promise<any>>>(new Set());

  const scheduleAutoSave = useCallback((nextSheet: SheetState) => {
    if (!token || !fileName || !autoSaveEnabled || currentAccessRole === 'viewer') {
      console.log('Auto save skipped:', { hasToken: !!token, hasFileName: !!fileName, autoSaveEnabled });
      return;
    }

    const seq = ++saveSeqRef.current;
    setSaveStatus('saving');

    // Parallel save - immediate save without debounce
    (async () => {
      try {
        console.log('🚀 Parallel auto save started for:', fileName);
        const payload = { id: currentFileId ?? undefined, name: fileName, state: nextSheet };

        // Create save promise and track it
        const savePromise = saveFile(token, payload);
        activeSaveRequests.current.add(savePromise);

        const saved = await savePromise;

        // Remove from tracking
        activeSaveRequests.current.delete(savePromise);

        console.log('✅ Parallel auto save SUCCESS! File ID:', saved.id);

        if (!currentFileId) {
          setCurrentFileId(saved.id);
          // Only refresh file list if this was a new file
          const fileListPromise = listFiles(token);
          activeSaveRequests.current.add(fileListPromise);
          const files = await fileListPromise;
          activeSaveRequests.current.delete(fileListPromise);
          setFiles(files);
        }

        if (seq === saveSeqRef.current) {
          setSaveStatus('saved');
          setLastSavedAt(Date.now());
        }
      } catch (err) {
        // Log error but don't show to user to avoid spam during rapid typing
        console.error('❌ Parallel auto save ERROR:', err);
        if (seq === saveSeqRef.current) {
          setSaveStatus('error');
        }
        // Only show error if it's the last active save request
        if (activeSaveRequests.current.size === 0) {
          setErrorMessage(`Avtomatik saqlashda xato: ${err instanceof Error ? err.message : 'Noma\'lum xato'}`);
        }
      }
    })();
  }, [token, fileName, currentFileId, autoSaveEnabled, currentAccessRole]);

  // Periodic auto save every 1 minute - use ref to get latest sheet
  const latestSheetRef = useRef(sheet);
  useEffect(() => {
    latestSheetRef.current = sheet;
  }, [sheet]);

  useEffect(() => {
    findIndexRef.current = findIndex;
  }, [findIndex]);

  const findMatches = useMemo(() => {
    const needleRaw = findQuery.trim();
    if (!needleRaw) return [];

    const needle = findMatchCase ? needleRaw : needleRaw.toLowerCase();
    const bounds = findScope === 'selection' && sheet.selection
      ? {
        minR: Math.min(sheet.selection.start.row, sheet.selection.end.row),
        maxR: Math.max(sheet.selection.start.row, sheet.selection.end.row),
        minC: Math.min(sheet.selection.start.col, sheet.selection.end.col),
        maxC: Math.max(sheet.selection.start.col, sheet.selection.end.col),
      }
      : null;

    const matches: Array<{ row: number; col: number }> = [];
    Object.entries(sheet.data).forEach(([id, cell]) => {
      const raw = cell?.value ?? '';
      if (!raw) return;
      const [rowStr, colStr] = id.split(',');
      const row = parseInt(rowStr, 10);
      const col = parseInt(colStr, 10);
      if (Number.isNaN(row) || Number.isNaN(col)) return;

      if (bounds) {
        if (row < bounds.minR || row > bounds.maxR || col < bounds.minC || col > bounds.maxC) return;
      }

      const haystack = findMatchCase ? raw : raw.toLowerCase();
      const ok = findWholeCell ? haystack === needle : haystack.includes(needle);
      if (ok) matches.push({ row, col });
    });

    matches.sort((a, b) => (a.row - b.row) || (a.col - b.col));
    return matches;
  }, [sheet.data, sheet.selection, findQuery, findMatchCase, findWholeCell, findScope]);

  const hasSheetData = useMemo(() => (
    Object.values(sheet.data).some(cell => (cell?.value ?? '').toString().trim() !== '')
  ), [sheet.data]);

  useEffect(() => {
    if (!findOpen) return;
    setFindIndex(prev => {
      if (findMatches.length === 0) return -1;
      if (prev < 0 || prev >= findMatches.length) return 0;
      return prev;
    });
  }, [findOpen, findMatches]);

  const goToCell = useCallback((row: number, col: number) => {
    const safeRow = Math.max(0, row);
    const safeCol = Math.max(0, Math.min(NUM_COLS - 1, col));
    setSheet(prev => {
      let nextRowCount = prev.rowCount;
      if (safeRow >= nextRowCount) {
        nextRowCount = ensureRowCountForIndex(nextRowCount, safeRow);
      }
      const activeCell = { row: safeRow, col: safeCol };
      return {
        ...prev,
        rowCount: nextRowCount,
        activeCell,
        selection: { start: activeCell, end: activeCell }
      };
    });
  }, []);

  const handleGoToCellLabel = useCallback((label: string) => {
    const normalized = label.trim().toUpperCase();
    if (!normalized) return;
    const coords = cellLabelToCoords(normalized);
    if (!coords) {
      setErrorMessage("Katak manzili noto'g'ri. Masalan: A1");
      return;
    }
    if (coords.col < 0 || coords.col >= NUM_COLS) {
      setErrorMessage(`Ustun chegaradan tashqarida. Max: ${getColumnLabel(NUM_COLS - 1)}`);
      return;
    }
    goToCell(coords.row, coords.col);
  }, [goToCell]);

  const goToMatchIndex = useCallback((idx: number) => {
    const match = findMatches[idx];
    if (!match) return;
    goToCell(match.row, match.col);
  }, [findMatches, goToCell]);

  const handleFindNext = useCallback(() => {
    if (!findMatches.length) return;
    const current = findIndexRef.current;
    const next = current < 0 ? 0 : (current + 1) % findMatches.length;
    setFindIndex(next);
    goToMatchIndex(next);
  }, [findMatches.length, goToMatchIndex]);

  const handleFindPrev = useCallback(() => {
    if (!findMatches.length) return;
    const current = findIndexRef.current;
    const prev = current < 0 ? findMatches.length - 1 : (current - 1 + findMatches.length) % findMatches.length;
    setFindIndex(prev);
    goToMatchIndex(prev);
  }, [findMatches.length, goToMatchIndex]);

  const replaceOnce = useCallback((source: string, needle: string, replacement: string) => {
    if (!needle) return source;
    if (findWholeCell) {
      const ok = findMatchCase ? source === needle : source.toLowerCase() === needle.toLowerCase();
      return ok ? replacement : source;
    }
    if (findMatchCase) {
      const idx = source.indexOf(needle);
      if (idx === -1) return source;
      return source.slice(0, idx) + replacement + source.slice(idx + needle.length);
    }
    const lowerSource = source.toLowerCase();
    const lowerNeedle = needle.toLowerCase();
    const idx = lowerSource.indexOf(lowerNeedle);
    if (idx === -1) return source;
    return source.slice(0, idx) + replacement + source.slice(idx + needle.length);
  }, [findMatchCase, findWholeCell]);

  const replaceAllInString = useCallback((source: string, needle: string, replacement: string) => {
    if (!needle) return source;
    if (findWholeCell) {
      const ok = findMatchCase ? source === needle : source.toLowerCase() === needle.toLowerCase();
      return ok ? replacement : source;
    }
    if (findMatchCase) {
      return source.split(needle).join(replacement);
    }
    const escaped = needle.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const re = new RegExp(escaped, 'gi');
    return source.replace(re, () => replacement);
  }, [findMatchCase, findWholeCell]);

  useEffect(() => {
    if (!autoSaveEnabled || !token || !fileName || currentAccessRole === 'viewer') {
      console.log('Periodic auto save disabled:', { autoSaveEnabled, hasToken: !!token, hasFileName: !!fileName });
      return;
    }

    console.log('Periodic auto save enabled - will save every 60 seconds');
    const interval = setInterval(async () => {
      try {
        console.log('Periodic auto save executing...');
        const currentSheet = latestSheetRef.current;
        const payload = { id: currentFileId ?? undefined, name: fileName, state: currentSheet };
        const saved = await saveFile(token, payload);
        console.log('Periodic auto save success:', saved.id);
        if (!currentFileId) {
          setCurrentFileId(saved.id);
          setFiles(await listFiles(token));
        }
      } catch (err) {
        console.error('Periodic auto save error:', err);
        setErrorMessage(`Auto save xato: ${err instanceof Error ? err.message : 'Noma\'lum xato'}`);
      }
    }, 60000); // 1 minute = 60000ms

    return () => {
      console.log('Periodic auto save interval cleared');
      clearInterval(interval);
    };
  }, [autoSaveEnabled, token, fileName, currentFileId, currentAccessRole]);

  const flushRemoteEdits = useCallback(() => {
    const pending = remoteQueue.current;
    remoteQueue.current = {};
    remoteRaf.current = null;

    const entries = Object.entries(pending);
    if (!entries.length) return;

    setSheet(prev => {
      const newData = { ...prev.data };
      let maxRow = -1;
      entries.forEach(([id, value]) => {
        newData[id] = { ...(newData[id] || {}), value };
        const [rowStr] = id.split(',');
        const row = parseInt(rowStr, 10);
        if (!isNaN(row) && row > maxRow) {
          maxRow = row;
        }
      });
      const rowCount = maxRow >= 0 ? ensureRowCountForIndex(prev.rowCount, maxRow) : prev.rowCount;
      return { ...prev, rowCount, data: recomputeSheet(newData) };
    });
  }, []);

  const enqueueRemoteEdits = useCallback((edits: Array<{ row: number; col: number; value: string }>) => {
    edits.forEach(({ row, col, value }) => {
      const id = getCellId(row, col);
      remoteQueue.current[id] = value;
    });
    if (remoteRaf.current === null) {
      remoteRaf.current = requestAnimationFrame(flushRemoteEdits);
    }
  }, [flushRemoteEdits]);

  const cancelRemoteQueue = () => {
    if (remoteRaf.current !== null) {
      cancelAnimationFrame(remoteRaf.current);
      remoteRaf.current = null;
    }
    remoteQueue.current = {};
  };

  // Undo/Redo History
  const [history, setHistory] = useState<SheetState[]>([]);
  const [historyIndex, setHistoryIndex] = useState(-1);
  const historyRef = useRef<SheetState[]>([]);
  const historyIndexRef = useRef(-1);

  // Sync refs with state
  useEffect(() => {
    historyRef.current = history;
    historyIndexRef.current = historyIndex;
  }, [history, historyIndex]);

  const resetHistory = useCallback((initial: SheetState) => {
    historyRef.current = [initial];
    historyIndexRef.current = 0;
    setHistory([initial]);
    setHistoryIndex(0);
  }, []);

  // Auto-save to localStorage
  useEffect(() => {
    const timer = setTimeout(() => {
      localStorage.setItem('sheetmaster-data', JSON.stringify(sheet));
      localStorage.setItem('sheetmaster-current-sheet', JSON.stringify(sheet));
    }, 1000); // Debounce saves by 1 second

    return () => clearTimeout(timer);
  }, [sheet]);

  // Persist auth token
  useEffect(() => {
    if (typeof window === 'undefined') return;
    if (token) {
      localStorage.setItem(TOKEN_KEY, token);
    } else {
      localStorage.removeItem(TOKEN_KEY);
    }
  }, [token, TOKEN_KEY]);

  // Validate existing token
  useEffect(() => {
    if (!token) {
      setUser(null);
      setFiles([]);
      setCurrentFileId(null);
      setRealtimeClient(prev => {
        if (prev) prev.disconnect();
        return null;
      });
      return;
    }
    if (currentFileIdRef.current) {
      return;
    }

    let cancelled = false;
    (async () => {
      try {
        const me = await fetchMe(token);
        if (!cancelled) setUser(me);
        // load user files
        const list = await listFiles(token);
        if (cancelled) return;
        setFiles(list);

        if (!currentFileIdRef.current && list.length > 0) {
          const lastKey = `sheetmaster-last-file-id:${me.id}`;
          const stored = localStorage.getItem(lastKey) || localStorage.getItem('sheetmaster-last-file-id');
          let targetId: number | null = null;
          if (stored) {
            const parsed = parseInt(stored, 10);
            if (!Number.isNaN(parsed) && list.some((f) => f.id === parsed)) {
              targetId = parsed;
            }
          }
          if (!targetId) {
            targetId = list[0]?.id ?? null;
          }

          if (targetId && !currentFileIdRef.current) {
            const file = await getFile(token, targetId);
            if (cancelled) return;
            setCurrentFileId(file.id);
            setFileName(file.name);
            setCurrentAccessRole(file.access_role || 'owner');
            const state = file.state as SheetState;
            const highestRow = getHighestRowIndex(state.data || {});
            const rowCount = ensureRowCountForIndex(state.rowCount ?? NUM_ROWS, highestRow);
            const nextSheet = { ...emptySheet, ...state, rowCount, data: recomputeSheet(state.data || {}) };
            setEditingCell(null);
            setSheet(nextSheet);
            resetHistory(nextSheet);
            setSaveStatus('idle');
            setLastSavedAt(null);
          }
        }
      } catch (err) {
        if (!cancelled) {
          setErrorMessage(`Tokenni tekshirishda xato: ${err instanceof Error ? err.message : 'Noma\'lum xato'}`);
          setUser(null);
          setToken(null);
          setFiles([]);
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [token]);

  useEffect(() => {
    if (!user || !currentFileId) return;
    const key = `sheetmaster-last-file-id:${user.id}`;
    try {
      localStorage.setItem(key, String(currentFileId));
      localStorage.setItem('sheetmaster-last-file-id', String(currentFileId));
    } catch (err) {
      console.error('Failed to persist last file id:', err);
    }
  }, [user, currentFileId]);

  // Connect to realtime channel when a saved file is open
  useEffect(() => {
    if (!token || !user || !currentFileId) {
      setRealtimeClient(prev => {
        if (prev) prev.disconnect();
        return null;
      });
      setRealtimeStatus('disconnected');
      return;
    }

    let cancelled = false;
    let rt: RealtimeClient | null = null;

    (async () => {
      try {
        setRealtimeStatus('connecting');
        const { token: sheetToken } = await getFileRealtimeToken(token, currentFileId);
        if (cancelled) return;

        rt = new RealtimeClient({
          token: sheetToken,
          sheetId: currentFileId,
          userName: user.name,
          handlers: {
            onInitialState: (grid) => {
              setRealtimeStatus('connected');
              cancelRemoteQueue();
              setSheet(prev => {
                const merged = recomputeSheet({ ...prev.data, ...grid });
                const highestRow = getHighestRowIndex(grid);
                const rowCount = highestRow >= 0 ? ensureRowCountForIndex(prev.rowCount, highestRow) : prev.rowCount;
                return { ...prev, rowCount, data: merged };
              });
            },
            onCellUpdate: ({ row, col, value }) => {
              enqueueRemoteEdits([{ row, col, value }]);
            },
            onBatchUpdate: (edits) => {
              enqueueRemoteEdits(edits);
            },
            onDisconnect: () => {
              console.warn('Realtime disconnected');
              setRealtimeStatus('disconnected');
            },
            onError: (reason) => {
              setErrorMessage(`Realtime ulanishda xato: ${reason || 'Noma\'lum xato'}`);
              setRealtimeStatus('error');
            }
          }
        });

        rt.connect();
        if (cancelled) {
          rt.disconnect();
          return;
        }
        setRealtimeClient(rt);
      } catch (err) {
        if (!cancelled) {
          setErrorMessage(`Realtime token olishda xato: ${err instanceof Error ? err.message : 'Noma\'lum xato'}`);
          setRealtimeStatus('error');
        }
      }
    })();

    return () => {
      cancelled = true;
      if (rt) {
        rt.disconnect();
      }
      setRealtimeClient(null);
      cancelRemoteQueue();
      setRealtimeStatus('disconnected');
    };
  }, [token, user, currentFileId]);

  // Sync formula bar with active cell
  useEffect(() => {
    if (sheet.activeCell) {
      if (editingCell && editingCell.row === sheet.activeCell.row && editingCell.col === sheet.activeCell.col) {
        return;
      }
      const id = getCellId(sheet.activeCell.row, sheet.activeCell.col);
      const val = sheet.data[id]?.value || '';
      setFormulaValue(val);
    }
  }, [sheet.activeCell, sheet.data, editingCell]);

  // Save state to history for undo/redo
  const saveState = useCallback((newSheet: SheetState) => {
    setHistory(prev => {
      const currentIndex = historyIndexRef.current;
      const newHistory = prev.slice(0, currentIndex + 1);
      newHistory.push(newSheet);
      // Keep only last 50 states
      if (newHistory.length > 50) {
        newHistory.shift();
        return newHistory;
      }
      return newHistory;
    });
    setHistoryIndex(prev => Math.min(prev + 1, 49));
    setSheet(newSheet);

    // Save to localStorage immediately to prevent data loss on refresh
    try {
      localStorage.setItem('sheetmaster-current-sheet', JSON.stringify(newSheet));
    } catch (e) {
      console.error('Failed to save to localStorage:', e);
    }

    scheduleAutoSave(newSheet);
  }, [scheduleAutoSave]);

  // Undo function
  const undo = useCallback(() => {
    if (!ensureWritable('undo')) return;
    if (historyIndexRef.current > 0) {
      const newIndex = historyIndexRef.current - 1;
      setHistoryIndex(newIndex);
      setSheet(historyRef.current[newIndex]);
    }
  }, [ensureWritable]);

  // Redo function
  const redo = useCallback(() => {
    if (!ensureWritable('redo')) return;
    if (historyIndexRef.current < historyRef.current.length - 1) {
      const newIndex = historyIndexRef.current + 1;
      setHistoryIndex(newIndex);
      setSheet(historyRef.current[newIndex]);
    }
  }, [ensureWritable]);

  const handleAuthFormChange = useCallback((field: 'name' | 'email' | 'password', value: string) => {
    setAuthForm(prev => ({ ...prev, [field]: value }));
  }, []);

  const handleAuthSubmit = useCallback(async () => {
    setAuthError(null);
    if (!authForm.email || !authForm.password || (authMode === 'register' && !authForm.name)) {
      setAuthError('Please fill the required fields.');
      return;
    }

    setAuthLoading(true);
    try {
      const payload = { email: authForm.email, password: authForm.password, name: authForm.name };
      const result = authMode === 'register'
        ? await register(payload)
        : await login({ email: payload.email, password: payload.password });

      setToken(result.token);
      setUser(result.user);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Authentication failed';
      setAuthError(message);
    } finally {
      setAuthLoading(false);
    }
  }, [authForm.email, authForm.name, authForm.password, authMode]);

  const handleLogout = useCallback(() => {
    setToken(null);
    setUser(null);
    setShareOpen(false);
    setCurrentFileId(null);
    setFiles([]);
    setCurrentAccessRole('owner');
    setSaveStatus('idle');
    setLastSavedAt(null);
    setRealtimeStatus('disconnected');
    historyRef.current = [];
    historyIndexRef.current = -1;
    setHistory([]);
    setHistoryIndex(-1);
  }, []);



  const handleNewFile = useCallback(() => {
    setSheet(emptySheet);
    resetHistory(emptySheet);
    setCurrentFileId(null);
    setFileName('Yangi fayl');
    setCurrentAccessRole('owner');
    setSaveStatus('idle');
    setLastSavedAt(null);
  }, [emptySheet, resetHistory]);

  const handleSaveFile = useCallback(async () => {
    if (!ensureWritable('saqlash')) return;
    if (!token) {
      // setFileError(true); // This variable is not defined in the original code, removing.
      // setFileMessage('Token topilmadi. Avval kirish qiling.'); // This variable is not defined in the original code, removing.
      return;
    }
    if (!fileName) {
      // setFileError(true); // This variable is not defined in the original code, removing.
      // setFileMessage('Fayl nomi kerak.'); // This variable is not defined in the original code, removing.
      return;
    }

    // setSavingFile(true); // This variable is not defined in the original code, removing.
    // setFileMessage(null); // This variable is not defined in the original code, removing.
    // setFileError(false); // This variable is not defined in the original code, removing.
    const seq = ++saveSeqRef.current;
    setSaveStatus('saving');
    try {
      const payload = { id: currentFileId ?? undefined, name: fileName, state: sheet };
      const saved = await saveFile(token, payload);
      setCurrentFileId(saved.id);
      // setFileMessage('Saqlash muvaffaqiyatli'); // This variable is not defined in the original code, removing.
      setFiles(await listFiles(token));
      if (seq === saveSeqRef.current) {
        setSaveStatus('saved');
        setLastSavedAt(Date.now());
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Saqlashda xato';
      // setFileError(true); // This variable is not defined in the original code, removing.
      // setFileMessage(msg); // This variable is not defined in the original code, removing.
      setErrorMessage(`Saqlashda xato: ${err instanceof Error ? err.message : 'Noma\'lum xato'}`);
      if (seq === saveSeqRef.current) {
        setSaveStatus('error');
      }
    } finally {
      // setSavingFile(false); // This variable is not defined in the original code, removing.
    }
  }, [token, fileName, currentFileId, sheet, ensureWritable]);

  const handleSelectFile = useCallback(async (id: number) => {
    if (!token) return;
    try {
      setShareOpen(false);
      const file = await getFile(token, id);
      setCurrentFileId(file.id);
      setFileName(file.name);
      setCurrentAccessRole(file.access_role || 'owner');
      const state = file.state as SheetState;
      const highestRow = getHighestRowIndex(state.data || {});
      const rowCount = ensureRowCountForIndex(state.rowCount ?? NUM_ROWS, highestRow);
      const nextSheet = { ...emptySheet, ...state, rowCount, data: recomputeSheet(state.data || {}) };
      setEditingCell(null);
      setSheet(nextSheet);
      resetHistory(nextSheet);
      setSaveStatus('idle');
      setLastSavedAt(null);
    } catch (err) {
      setErrorMessage(`Faylni yuklashda xato: ${err instanceof Error ? err.message : 'Noma\'lum xato'}`);
    }
  }, [token, resetHistory]);

  const handleDeleteFile = useCallback((id: number) => {
    setPendingDeleteId(id);
  }, []);

  const confirmDeleteFile = useCallback(async () => {
    if (!token || pendingDeleteId === null) return;
    try {
      await deleteFile(token, pendingDeleteId);
      const updatedFiles = await listFiles(token);
      setFiles(updatedFiles);
      if (currentFileId === pendingDeleteId) {
        handleNewFile();
      }
    } catch (err) {
      setErrorMessage(`Faylni o'chirishda xato: ${err instanceof Error ? err.message : 'Noma\'lum xato'}`);
      alert('Faylni o\'chirishda xato: ' + (err instanceof Error ? err.message : 'Unknown error'));
    } finally {
      setPendingDeleteId(null);
    }
  }, [token, pendingDeleteId, currentFileId, handleNewFile]);

  const updateCell = useCallback((row: number, col: number, value: string, opts?: { broadcast?: boolean }) => {
    if (!ensureWritable()) return;
    const id = getCellId(row, col);
    const newData = {
      ...sheet.data,
      [id]: { ...sheet.data[id], value }
    };

    // Recompute formulas
    const computedData = recomputeSheet(newData);

    const rowCount = row >= sheet.rowCount ? ensureRowCountForIndex(sheet.rowCount, row) : sheet.rowCount;
    const newSheet = { ...sheet, rowCount, data: computedData };
    saveState(newSheet);

    if (opts?.broadcast !== false && realtimeClient) {
      realtimeClient.sendCellEdit(row, col, value);
    }
  }, [sheet, saveState, realtimeClient, ensureWritable]);

  const handleReplaceCurrent = useCallback(() => {
    if (!ensureWritable("almashtirish")) return;
    const needle = findQuery.trim();
    if (!needle) return;
    const idx = findIndexRef.current;
    if (idx < 0 || idx >= findMatches.length) return;
    const match = findMatches[idx];
    const id = getCellId(match.row, match.col);
    const currentVal = sheet.data[id]?.value ?? '';
    const nextVal = replaceOnce(currentVal, needle, findReplaceText);
    if (nextVal === currentVal) {
      handleFindNext();
      return;
    }
    updateCell(match.row, match.col, nextVal);
    window.setTimeout(() => handleFindNext(), 0);
  }, [findQuery, findReplaceText, findMatches, sheet.data, replaceOnce, handleFindNext, updateCell, ensureWritable]);

  const handleReplaceAll = useCallback(() => {
    if (!ensureWritable("almashtirish")) return;
    const needle = findQuery.trim();
    if (!needle) return;
    if (findMatches.length === 0) {
      setErrorMessage('Topilmadi');
      return;
    }

    const newData = { ...sheet.data };
    const editsToSend: Array<{ row: number; col: number; value: string }> = [];

    findMatches.forEach(({ row, col }) => {
      const id = getCellId(row, col);
      const currentVal = newData[id]?.value ?? '';
      const nextVal = replaceAllInString(currentVal, needle, findReplaceText);
      if (nextVal !== currentVal) {
        newData[id] = { ...(newData[id] || { value: '' }), value: nextVal };
        editsToSend.push({ row, col, value: nextVal });
      }
    });

    if (editsToSend.length === 0) {
      setErrorMessage('Hech qanday o‘zgarish bo‘lmadi');
      return;
    }

    const newSheet = { ...sheet, data: recomputeSheet(newData) };
    saveState(newSheet);

    if (realtimeClient) {
      if (editsToSend.length === 1) {
        const single = editsToSend[0];
        realtimeClient.sendCellEdit(single.row, single.col, single.value);
      } else {
        realtimeClient.sendBatch(editsToSend);
      }
    }

    setErrorMessage(`${editsToSend.length} ta katak almashtirildi`);
  }, [findQuery, findReplaceText, findMatches, replaceAllInString, sheet, saveState, realtimeClient, ensureWritable]);

  const startEditing = useCallback((row: number, col: number, initialValue?: string) => {
    if (currentAccessRole === 'viewer') {
      if (initialValue !== undefined) {
        setErrorMessage("Read-only: tahrirlash mumkin emas");
      }
      return;
    }
    const id = getCellId(row, col);
    const currentValue = initialValue !== undefined ? initialValue : (sheet.data[id]?.value || '');
    setEditingCell({ row, col });
    setFormulaValue(currentValue);
    setSheet(prev => ({
      ...prev,
      activeCell: { row, col },
      selection: { start: { row, col }, end: { row, col } }
    }));
  }, [sheet.data, currentAccessRole]);

  const commitEditing = useCallback((row: number, col: number, move?: { rowDelta?: number; colDelta?: number }) => {
    if (currentAccessRole === 'viewer') {
      setEditingCell(null);
      return;
    }
    updateCell(row, col, formulaValue);
    setEditingCell(null);

    if (move && (move.rowDelta || move.colDelta)) {
      setSheet(prev => {
        const desiredRow = row + (move.rowDelta || 0);
        const desiredCol = col + (move.colDelta || 0);
        let nextRow = Math.max(0, desiredRow);
        let nextCol = Math.max(0, Math.min(NUM_COLS - 1, desiredCol));
        let rowCount = prev.rowCount;
        if (nextRow >= rowCount) {
          rowCount = ensureRowCountForIndex(rowCount, nextRow);
        }
        const nextActive = { row: nextRow, col: nextCol };
        return {
          ...prev,
          rowCount,
          activeCell: nextActive,
          selection: { start: nextActive, end: nextActive }
        };
      });
    }
  }, [formulaValue, updateCell, currentAccessRole]);

  // Column/Row Resize Handlers
  const handleColumnResize = useCallback((col: number, width: number) => {
    if (currentAccessRole === 'viewer') return;
    setSheet(prev => ({
      ...prev,
      columnWidths: { ...prev.columnWidths, [col]: width }
    }));
  }, [currentAccessRole]);

  const handleRowResize = useCallback((row: number, height: number) => {
    if (currentAccessRole === 'viewer') return;
    setSheet(prev => ({
      ...prev,
      rowHeights: { ...prev.rowHeights, [row]: height }
    }));
  }, [currentAccessRole]);

  const handleRequestMoreRows = useCallback(() => {
    setSheet(prev => ({
      ...prev,
      rowCount: prev.rowCount + ROW_BATCH_SIZE
    }));
  }, []);

  // Context Menu Handlers
  const handleContextMenu = useCallback((row: number, col: number, x: number, y: number) => {
    setSheet(prev => {
      const activeCell = { row, col };
      const selection = prev.selection;
      let nextSelection = selection;
      if (!selection) {
        nextSelection = { start: activeCell, end: activeCell };
      } else {
        const minR = Math.min(selection.start.row, selection.end.row);
        const maxR = Math.max(selection.start.row, selection.end.row);
        const minC = Math.min(selection.start.col, selection.end.col);
        const maxC = Math.max(selection.start.col, selection.end.col);
        const inside = row >= minR && row <= maxR && col >= minC && col <= maxC;
        if (!inside) {
          nextSelection = { start: activeCell, end: activeCell };
        }
      }
      return { ...prev, activeCell, selection: nextSelection };
    });
    setContextMenu({ show: true, x, y, row, col });
  }, []);

  const handleCloseContextMenu = useCallback(() => {
    setContextMenu({ show: false, x: 0, y: 0, row: 0, col: 0 });
  }, []);

  const sortSelectionByColumn = useCallback((direction: 'asc' | 'desc') => {
    if (!ensureWritable('sort')) return;
    if (!sheet.selection) {
      setErrorMessage('Avval range tanlang (selection).');
      return;
    }

    const { start, end } = sheet.selection;
    const minR = Math.min(start.row, end.row);
    const maxR = Math.max(start.row, end.row);
    const minC = Math.min(start.col, end.col);
    const maxC = Math.max(start.col, end.col);

    if (minR === maxR) {
      setErrorMessage('Sort uchun kamida 2 qator tanlang.');
      return;
    }

    const preferredCol = sheet.activeCell?.col;
    const fallbackCol = contextMenu.col;
    const sortCol = (preferredCol !== undefined && preferredCol >= minC && preferredCol <= maxC)
      ? preferredCol
      : (fallbackCol >= minC && fallbackCol <= maxC ? fallbackCol : minC);

    const rows: Array<{
      originalRow: number;
      originalIndex: number;
      keyRaw: string;
      keyNum: number | null;
      cells: Array<CellData | undefined>;
    }> = [];

    for (let r = minR; r <= maxR; r++) {
      const keyId = getCellId(r, sortCol);
      const keyRaw = (sheet.data[keyId]?.value ?? '').toString();
      const parsed = parseFloat(keyRaw);
      const keyNum = keyRaw.trim() !== '' && !Number.isNaN(parsed) ? parsed : null;

      const cells: Array<CellData | undefined> = [];
      for (let c = minC; c <= maxC; c++) {
        cells.push(sheet.data[getCellId(r, c)]);
      }

      rows.push({ originalRow: r, originalIndex: r - minR, keyRaw, keyNum, cells });
    }

    const compare = (a: typeof rows[number], b: typeof rows[number]) => {
      if (a.keyNum !== null && b.keyNum !== null) {
        return a.keyNum - b.keyNum;
      }
      const aStr = a.keyRaw.toLowerCase();
      const bStr = b.keyRaw.toLowerCase();
      return aStr.localeCompare(bStr, undefined, { numeric: true, sensitivity: 'base' });
    };

    rows.sort((a, b) => {
      const base = compare(a, b);
      const ordered = direction === 'asc' ? base : -base;
      return ordered !== 0 ? ordered : a.originalIndex - b.originalIndex;
    });

    const oldData = sheet.data;
    const newData: GridData = { ...oldData };
    const editsToSend: Array<{ row: number; col: number; value: string }> = [];

    for (let i = 0; i < rows.length; i++) {
      const targetRow = minR + i;
      const sourceRowCells = rows[i].cells;

      for (let c = minC; c <= maxC; c++) {
        const targetId = getCellId(targetRow, c);
        const sourceCell = sourceRowCells[c - minC];
        const oldVal = oldData[targetId]?.value ?? '';

        if (sourceCell) {
          newData[targetId] = { ...sourceCell };
        } else {
          delete newData[targetId];
        }

        const newVal = sourceCell?.value ?? '';
        if (newVal !== oldVal) {
          editsToSend.push({ row: targetRow, col: c, value: newVal });
        }
      }
    }

    const newSheet = { ...sheet, data: recomputeSheet(newData) };
    saveState(newSheet);

    if (realtimeClient && editsToSend.length > 0) {
      if (editsToSend.length === 1) {
        const single = editsToSend[0];
        realtimeClient.sendCellEdit(single.row, single.col, single.value);
      } else {
        realtimeClient.sendBatch(editsToSend);
      }
    }

    setErrorMessage(direction === 'asc' ? 'Sort: A → Z' : 'Sort: Z → A');
  }, [sheet, contextMenu.col, saveState, realtimeClient, ensureWritable]);

  const handleSortAsc = useCallback(() => sortSelectionByColumn('asc'), [sortSelectionByColumn]);
  const handleSortDesc = useCallback(() => sortSelectionByColumn('desc'), [sortSelectionByColumn]);

  // Clipboard Operations
  const copySelectionToClipboard = useCallback((isCut: boolean) => {
    if (!sheet.selection) return;
    const { start, end } = sheet.selection;
    const minR = Math.min(start.row, end.row);
    const maxR = Math.max(start.row, end.row);
    const minC = Math.min(start.col, end.col);
    const maxC = Math.max(start.col, end.col);

    const cells: ClipboardData['cells'] = [];
    const tsvRows: string[] = [];

	    for (let r = minR; r <= maxR; r++) {
	      const rowVals: string[] = [];
	      for (let c = minC; c <= maxC; c++) {
	        const id = getCellId(r, c);
	        const cell = sheet.data[id];
	        if (cell) {
	          cells.push({ row: r - minR, col: c - minC, data: cell });
	        }
	        rowVals.push(cell?.value ?? '');
	      }
	      tsvRows.push(rowVals.join('\t'));
	    }
	    const text = tsvRows.join('\n');
	    lastCopiedTextRef.current = text;
	    setClipboard({
	      rows: maxR - minR + 1,
	      cols: maxC - minC + 1,
	      cells,
      isCut,
      source: { startRow: minR, startCol: minC, endRow: maxR, endCol: maxC },
    });
    void writeToSystemClipboard(text);
  }, [sheet.selection, sheet.data, writeToSystemClipboard]);

  const handleCopy = useCallback(() => {
    copySelectionToClipboard(false);
  }, [copySelectionToClipboard]);

  const applyImportedRows = useCallback((rows: string[][], name?: string) => {
    if (!ensureWritable('import')) return;
    const data: GridData = {};
    let maxRow = 0;
    rows.forEach((row, r) => {
      maxRow = Math.max(maxRow, r);
      row.forEach((val, c) => {
        data[getCellId(r, c)] = { value: val ?? '', computed: val ?? '' };
      });
    });
    const rowCount = ensureRowCountForIndex(NUM_ROWS, maxRow);
    const newSheet: SheetState = {
      ...sheet,
      data: recomputeSheet(data),
      rowCount,
      activeCell: { row: 0, col: 0 },
      selection: { start: { row: 0, col: 0 }, end: { row: 0, col: 0 } },
    };
    saveState(newSheet);
    if (name) {
      setFileName(name);
    }
    setCurrentFileId(null);
  }, [sheet, saveState, ensureWritable]);

  const handleCut = useCallback(() => {
    if (!ensureWritable('kesish')) return;
    copySelectionToClipboard(true);
  }, [copySelectionToClipboard, ensureWritable]);

	  const pasteTextAtActiveCell = useCallback((clipboardText: string) => {
	    const base = latestSheetRef.current;
	    const activeCell = base.activeCell;
	    if (!activeCell) return;

    const normalized = clipboardText.replace(/\r\n/g, '\n').replace(/\r/g, '\n');
    const text = normalized.endsWith('\n') ? normalized.slice(0, -1) : normalized;
    if (!text.trim()) return;

    const rows = text.includes('\t')
      ? text.split('\n').map(line => line.split('\t'))
      : parseCsv(text);

    if (!rows.length) return;

    const baseRow = activeCell.row;
    const baseCol = activeCell.col;

    let hasDataInTarget = false;
    for (let r = 0; r < rows.length; r++) {
      for (let c = 0; c < rows[r].length; c++) {
        const targetRow = baseRow + r;
        const targetCol = baseCol + c;
        if (targetCol < NUM_COLS) {
          const id = getCellId(targetRow, targetCol);
          if (base.data[id]?.value) {
            hasDataInTarget = true;
            break;
          }
        }
      }
      if (hasDataInTarget) break;
    }

    const performPaste = () => {
      const current = latestSheetRef.current;
      const newData = { ...current.data };
      const editMap = new Map<string, string>();
      let maxTargetRow = -1;
      let maxTargetCol = -1;

      rows.forEach((row, r) => {
        row.forEach((value, c) => {
          const targetRow = baseRow + r;
          const targetCol = baseCol + c;
          if (targetCol >= NUM_COLS) return;
          if (value === undefined) return;

          const id = getCellId(targetRow, targetCol);
          const trimmedValue = value.toString().trim();

          if (trimmedValue) {
            newData[id] = { value: trimmedValue, computed: trimmedValue };
            editMap.set(`${targetRow},${targetCol}`, trimmedValue);
          } else if (newData[id]) {
            newData[id] = { value: '', computed: '' };
            editMap.set(`${targetRow},${targetCol}`, '');
          }

          if (targetRow > maxTargetRow) maxTargetRow = targetRow;
          if (targetCol > maxTargetCol) maxTargetCol = targetCol;
        });
      });

      const rowCount = maxTargetRow >= 0 ? ensureRowCountForIndex(current.rowCount, maxTargetRow) : current.rowCount;
      const computed = recomputeSheet(newData);
      const nextSelectionEnd = {
        row: maxTargetRow >= 0 ? maxTargetRow : baseRow,
        col: maxTargetCol >= 0 ? maxTargetCol : baseCol,
      };

      const nextSheet: SheetState = {
        ...current,
        rowCount,
        data: computed,
        activeCell: { row: baseRow, col: baseCol },
        selection: { start: { row: baseRow, col: baseCol }, end: nextSelectionEnd },
      };
      saveState(nextSheet);

      if (realtimeClient && editMap.size > 0) {
        const editsToSend = Array.from(editMap.entries()).map(([key, value]) => {
          const [rowStr, colStr] = key.split(',');
          return { row: parseInt(rowStr, 10), col: parseInt(colStr, 10), value };
        });
        if (editsToSend.length === 1) {
          const single = editsToSend[0];
          realtimeClient.sendCellEdit(single.row, single.col, single.value);
        } else {
          realtimeClient.sendBatch(editsToSend);
        }
      }
    };

    if (hasDataInTarget) {
      setOverwriteConfirm({ show: true, action: performPaste });
	    } else {
	      performPaste();
	    }
	  }, [saveState, realtimeClient]);

	  const pasteInternalClipboardAtActiveCell = useCallback(() => {
	    if (!clipboard) return;
	    const clip = clipboard;
	    const base = latestSheetRef.current;
	    const activeCell = base.activeCell;
	    if (!activeCell) return;

	    const baseRow = activeCell.row;
	    const baseCol = activeCell.col;
	    const targetEndRow = baseRow + clip.rows - 1;
	    const targetEndCol = baseCol + clip.cols - 1;

	    let hasDataInTarget = false;
	    for (let r = 0; r < clip.rows; r++) {
	      for (let c = 0; c < clip.cols; c++) {
	        const targetRow = baseRow + r;
	        const targetCol = baseCol + c;
	        if (targetCol >= NUM_COLS) continue;
	        const id = getCellId(targetRow, targetCol);
	        if (base.data[id]?.value) {
	          hasDataInTarget = true;
	          break;
	        }
	      }
	      if (hasDataInTarget) break;
	    }

	    const performPaste = () => {
	      const current = latestSheetRef.current;
	      const newData = { ...current.data };
	      const editMap = new Map<string, string>();
	      let maxAffectedRow = -1;

	      // Clear target rectangle first (so empty source cells overwrite)
	      for (let r = 0; r < clip.rows; r++) {
	        for (let c = 0; c < clip.cols; c++) {
	          const targetRow = baseRow + r;
	          const targetCol = baseCol + c;
	          if (targetCol >= NUM_COLS) continue;
	          const id = getCellId(targetRow, targetCol);
	          if (newData[id]) {
	            newData[id] = { value: '', computed: '' };
	            editMap.set(`${targetRow},${targetCol}`, '');
	          }
	          if (targetRow > maxAffectedRow) maxAffectedRow = targetRow;
	        }
	      }

	      // Apply copied cells
	      clip.cells.forEach(({ row, col, data }) => {
	        const targetRow = baseRow + row;
	        const targetCol = baseCol + col;
	        if (targetCol >= NUM_COLS) return;
	        const id = getCellId(targetRow, targetCol);
	        newData[id] = { ...data };
	        editMap.set(`${targetRow},${targetCol}`, data.value);
	        if (targetRow > maxAffectedRow) maxAffectedRow = targetRow;
	      });

	      if (clip.isCut) {
	        const src = clip.source;
	        for (let r = src.startRow; r <= src.endRow; r++) {
	          for (let c = src.startCol; c <= src.endCol; c++) {
	            if (c >= NUM_COLS) continue;
	            const overlapsTarget =
	              r >= baseRow && r <= targetEndRow &&
	              c >= baseCol && c <= targetEndCol;
	            if (overlapsTarget) continue;
	            const id = getCellId(r, c);
	            if (newData[id]) {
	              newData[id] = { value: '', computed: '' };
	              editMap.set(`${r},${c}`, '');
	            }
	            if (r > maxAffectedRow) maxAffectedRow = r;
	          }
	        }
	        setClipboard(null);
	      }

	      const rowCount = maxAffectedRow >= 0 ? ensureRowCountForIndex(current.rowCount, maxAffectedRow) : current.rowCount;
	      const selectionEnd = { row: targetEndRow, col: Math.min(NUM_COLS - 1, targetEndCol) };
	      const nextSheet = {
	        ...current,
	        rowCount,
	        data: recomputeSheet(newData),
	        activeCell: { row: baseRow, col: baseCol },
	        selection: { start: { row: baseRow, col: baseCol }, end: selectionEnd }
	      };
	      saveState(nextSheet);

	      if (realtimeClient && editMap.size > 0) {
	        const editsToSend = Array.from(editMap.entries()).map(([key, value]) => {
	          const [rowStr, colStr] = key.split(',');
	          return { row: parseInt(rowStr, 10), col: parseInt(colStr, 10), value };
	        });
	        if (editsToSend.length === 1) {
	          const single = editsToSend[0];
	          realtimeClient.sendCellEdit(single.row, single.col, single.value);
	        } else {
	          realtimeClient.sendBatch(editsToSend);
	        }
	      }
	    };

	    if (hasDataInTarget) {
	      setOverwriteConfirm({ show: true, action: performPaste });
	    } else {
	      performPaste();
	    }
	  }, [clipboard, saveState, realtimeClient]);

	  const handlePaste = useCallback(async () => {
	    if (!ensureWritable("qo'yish")) return;
	    if (!sheet.activeCell) return;
	    const { row: baseRow, col: baseCol } = sheet.activeCell;
    if (!baseRow && baseRow !== 0) return;
    if (!baseCol && baseCol !== 0) return;

    // Try to read from system clipboard first - check for files, then text
    try {
      // Try to read clipboard items (for files)
      const clipboardItems = await navigator.clipboard.read();

      for (const item of clipboardItems) {
        // Check if clipboard contains a file
        const fileTypes = item.types.filter(type =>
          type === 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' ||
          type === 'application/vnd.ms-excel' ||
          type === 'text/csv' ||
          type.startsWith('text/')
        );

        if (fileTypes.length > 0) {
          for (const fileType of fileTypes) {
            if (fileType === 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' ||
              fileType === 'application/vnd.ms-excel') {
              const blob = await item.getType(fileType);
              const file = new File([blob], 'clipboard.xlsx', { type: fileType });
              console.log('Pasting Excel file from clipboard');

              // Import Excel file directly
              try {
                const { blob: csvBlob } = await convertExcel(file, token || undefined);
                const csvText = await csvBlob.text();
                const rows = parseCsv(csvText);
                const baseName = file.name.replace(/\.(xlsx|xls)$/i, '');
                applyImportedRows(rows, baseName || fileName);
              } catch (err) {
                console.error('Excel import error:', err);
                setErrorMessage('Excel import server mavjud emas. Faqat CSV fayllarni import qilish mumkin.');
              }
              return;
            } else if (fileType === 'text/csv' || fileType.startsWith('text/')) {
              const blob = await item.getType(fileType);
              const text = await blob.text();

              if (text && text.trim()) {
                console.log('Pasting text/CSV from clipboard:', text.substring(0, 100));

                // If clipboard text looks like a local file path, prompt user to select the file
                const looksLikeFilePath = (txt: string) => {
                  const trimmed = txt.trim();
                  const hasNoNewline = !trimmed.includes('\n') && !trimmed.includes('\r');
                  const pathRegex = /^([A-Za-z]:[\\\/]|\/[^\/\s]+\/).+\.(csv|xlsx?|xls|txt|json|xml)$/i;
                  return hasNoNewline && pathRegex.test(trimmed);
                };

	                if (looksLikeFilePath(text)) {
	                  console.log('Detected file path, triggering file selector');
	                  setErrorMessage('Faylni tanlash uchun Import tugmasini bosing yoki faylni shu yerga sudrab olib keling (drag & drop).');
	                  // Trigger file input click to allow user to select the file
	                  const fileInput = document.getElementById('file-input') as HTMLInputElement;
	                  if (fileInput) {
	                    fileInput.click();
	                  }
	                  return;
	                }
	                pasteTextAtActiveCell(text);
	                return;
	              }
	            }
	          }
	        }
      }
    } catch (err) {
      console.log('Clipboard.read() failed, trying readText():', err);
    }

    // Fallback to readText() for simple text paste
    try {
      const clipboardText = await navigator.clipboard.readText();

      if (clipboardText && clipboardText.trim()) {
        console.log('Pasting from clipboard text:', clipboardText.substring(0, 100));

        // If clipboard text looks like a local file path, prompt user to select the file
        const looksLikeFilePath = (txt: string) => {
          const trimmed = txt.trim();
          const hasNoNewline = !trimmed.includes('\n') && !trimmed.includes('\r');
          const pathRegex = /^([A-Za-z]:[\\\/]|\/[^\/\s]+\/).+\.(csv|xlsx?|xls|txt|json|xml)$/i;
          return hasNoNewline && pathRegex.test(trimmed);
        };

	        if (looksLikeFilePath(clipboardText)) {
	          console.log('Detected file path in readText(), triggering file selector');
	          setErrorMessage('Faylni tanlash uchun Import tugmasini bosing yoki faylni shu yerga sudrab olib keling (drag & drop).');
	          // Trigger file input click to allow user to select the file
	          const fileInput = document.getElementById('file-input') as HTMLInputElement;
	          if (fileInput) {
	            fileInput.click();
	          }
	          return;
	        }
	        pasteTextAtActiveCell(clipboardText);
	        return;
	      }
		    } catch (err) {
		      console.log('System clipboard read failed, falling back to internal clipboard:', err);
		    }

		    // Fallback to internal clipboard
		    if (!clipboard) return;
		    pasteInternalClipboardAtActiveCell();
		    return;
	  }, [clipboard, sheet, saveState, realtimeClient, token, fileName, applyImportedRows, ensureWritable, pasteTextAtActiveCell, pasteInternalClipboardAtActiveCell]);

  const handleDelete = useCallback(() => {
    if (!ensureWritable("o'chirish")) return;
    if (!sheet.selection) return;
    const { start, end } = sheet.selection;
    const minR = Math.min(start.row, end.row);
    const maxR = Math.max(start.row, end.row);
    const minC = Math.min(start.col, end.col);
    const maxC = Math.max(start.col, end.col);

    const newData = { ...sheet.data };
    const editsToSend: Array<{ row: number; col: number; value: string }> = [];
    for (let r = minR; r <= maxR; r++) {
      for (let c = minC; c <= maxC; c++) {
        const id = getCellId(r, c);
        if (newData[id]) {
          newData[id] = { value: '', computed: '' };
          editsToSend.push({ row: r, col: c, value: '' });
        }
      }
    }
    const newSheet = { ...sheet, data: recomputeSheet(newData) };
    saveState(newSheet);
    if (realtimeClient && editsToSend.length > 0) {
      if (editsToSend.length === 1) {
        const single = editsToSend[0];
        realtimeClient.sendCellEdit(single.row, single.col, single.value);
      } else {
        realtimeClient.sendBatch(editsToSend);
      }
    }
  }, [sheet, saveState, realtimeClient, ensureWritable]);

  const handleMoveSelection = useCallback((targetRow: number, targetCol: number) => {
    if (!ensureWritable("ko'chirish")) return;
    if (!sheet.selection) return;

    const { start, end } = sheet.selection;
    const minR = Math.min(start.row, end.row);
    const maxR = Math.max(start.row, end.row);
    const minC = Math.min(start.col, end.col);
    const maxC = Math.max(start.col, end.col);

    // Calculate offset
    const rowOffset = targetRow - minR;
    const colOffset = targetCol - minC;

    // Don't move if target is same as source
    if (rowOffset === 0 && colOffset === 0) return;

    // Check if target cells have data
    let hasDataInTarget = false;
    for (let r = minR; r <= maxR; r++) {
      for (let c = minC; c <= maxC; c++) {
        const targetR = r + rowOffset;
        const targetC = c + colOffset;
        const targetId = getCellId(targetR, targetC);
        if (sheet.data[targetId]?.value) {
          hasDataInTarget = true;
          break;
        }
      }
      if (hasDataInTarget) break;
    }

    const performMove = () => {
      // Collect source cells data
      const cellsData: Array<{ row: number; col: number; data: CellData }> = [];
      for (let r = minR; r <= maxR; r++) {
        for (let c = minC; c <= maxC; c++) {
          const id = getCellId(r, c);
          cellsData.push({ row: r - minR, col: c - minC, data: sheet.data[id] || { value: '' } });
        }
      }

      const newData = { ...sheet.data };
      const editsToSend: Array<{ row: number; col: number; value: string }> = [];

      // Clear source cells
      for (let r = minR; r <= maxR; r++) {
        for (let c = minC; c <= maxC; c++) {
          const id = getCellId(r, c);
          newData[id] = { value: '', computed: '' };
          editsToSend.push({ row: r, col: c, value: '' });
        }
      }

      // Paste to target
      let maxTargetRow = -1;
      cellsData.forEach(({ row, col, data }) => {
        const targetR = targetRow + row;
        const targetC = targetCol + col;
        if (targetC < NUM_COLS) {
          const id = getCellId(targetR, targetC);
          newData[id] = { ...data };
          editsToSend.push({ row: targetR, col: targetC, value: data.value });
          if (targetR > maxTargetRow) maxTargetRow = targetR;
        }
      });

      const newRowCount = ensureRowCountForIndex(sheet.rowCount, maxTargetRow);
      const newSheet = {
        ...sheet,
        data: recomputeSheet(newData),
        rowCount: newRowCount,
        activeCell: { row: targetRow, col: targetCol },
        selection: {
          start: { row: targetRow, col: targetCol },
          end: { row: targetRow + (maxR - minR), col: targetCol + (maxC - minC) }
        }
      };
      saveState(newSheet);

      if (realtimeClient && editsToSend.length > 0) {
        realtimeClient.sendBatch(editsToSend);
      }
    };

    // Show modal if target has data
    if (hasDataInTarget) {
      setOverwriteConfirm({ show: true, action: performMove });
    } else {
      performMove();
    }
  }, [sheet, saveState, realtimeClient, ensureWritable]);

  const cancelEditing = useCallback(() => {
    if (sheet.activeCell) {
      const id = getCellId(sheet.activeCell.row, sheet.activeCell.col);
      const val = sheet.data[id]?.value || '';
      setFormulaValue(val);
    }
    setEditingCell(null);
  }, [sheet.activeCell, sheet.data]);

  const handleCellClick = (row: number, col: number, isShift: boolean) => {
    if (editingCell && (editingCell.row !== row || editingCell.col !== col)) {
      commitEditing(editingCell.row, editingCell.col);
    } else {
      setEditingCell(null);
    }

    // Apply format painter if active
    if (formatPainterActive && copiedFormat.current) {
      if (currentAccessRole === 'viewer') {
        setErrorMessage("Read-only: format qo'llash mumkin emas");
        setFormatPainterActive(false);
        copiedFormat.current = null;
      } else {
      const id = getCellId(row, col);
      const currentCell = sheet.data[id] || { value: '' };
      const newData = {
        ...sheet.data,
        [id]: {
          ...currentCell,
          style: { ...copiedFormat.current }
        }
      };
      const newSheet = { ...sheet, data: newData };
      saveState(newSheet);

      // Deactivate format painter after one use
      setFormatPainterActive(false);
      copiedFormat.current = null;
      }
    }

    setSheet(prev => {
      const newActive = { row, col };
      let newSelection = { start: newActive, end: newActive };

      if (isShift && prev.activeCell) {
        newSelection = {
          start: prev.activeCell,
          end: { row, col }
        };
        // Active cell doesn't change on shift click usually, but selection does
        return { ...prev, selection: newSelection };
      }

      return {
        ...prev,
        activeCell: newActive,
        selection: newSelection
      };
    });
  };

  const handleSelectionDrag = (row: number, col: number) => {
    setEditingCell(null);

    setSheet(prev => {
      if (!prev.activeCell) return prev;
      const currentEnd = prev.selection?.end;
      if (currentEnd && currentEnd.row === row && currentEnd.col === col) {
        return prev;
      }
      console.log('Selection drag:', {
        start: prev.activeCell,
        end: { row, col }
      });
      return {
        ...prev,
        selection: {
          start: prev.activeCell,
          end: { row, col }
        }
      };
    });
  };

  const handleFormulaSubmit = () => {
    if (sheet.activeCell) {
      updateCell(sheet.activeCell.row, sheet.activeCell.col, formulaValue);
      setEditingCell(null);
    }
  };

  const handleStyleChange = (styleChange: any) => {
    if (!ensureWritable('format')) return;
    console.log('🎨 Style change requested:', styleChange);
    if (!sheet.selection) {
      console.log('❌ No selection, aborting style change');
      return;
    }
    const { __reset, ...rest } = styleChange || {};

    const { start, end } = sheet.selection;
    const minR = Math.min(start.row, end.row);
    const maxR = Math.max(start.row, end.row);
    const minC = Math.min(start.col, end.col);
    const maxC = Math.max(start.col, end.col);

    console.log('📍 Applying style to cells:', { minR, maxR, minC, maxC, style: rest });

    const newData = { ...sheet.data };
    const editsToSend: Array<{ row: number; col: number; value: string }> = [];

    for (let r = minR; r <= maxR; r++) {
      for (let c = minC; c <= maxC; c++) {
        const id = getCellId(r, c);
        const currentCell = newData[id] || { value: '' };
        if (__reset) {
          // Clear both style AND value/computed
          newData[id] = { value: '', computed: '', style: {} };
          editsToSend.push({ row: r, col: c, value: '' });
        } else {
          newData[id] = {
            ...currentCell,
            style: { ...currentCell.style, ...rest }
          };
          console.log(`✅ Cell ${id} new style:`, newData[id].style);
        }
      }
    }

    const newSheet = { ...sheet, data: __reset ? recomputeSheet(newData) : newData };
    saveState(newSheet);
    console.log('💾 Style saved to state');

    // Broadcast clear operation if needed
    if (__reset && realtimeClient && editsToSend.length > 0) {
      if (editsToSend.length === 1) {
        const single = editsToSend[0];
        realtimeClient.sendCellEdit(single.row, single.col, single.value);
      } else {
        realtimeClient.sendBatch(editsToSend);
      }
    }
  };

  const handlePrint = () => {
    window.print();
  };

  const handleFormatPainter = () => {
    if (!ensureWritable('format')) return;
    if (formatPainterActive) {
      // Deactivate
      setFormatPainterActive(false);
      copiedFormat.current = null;
    } else {
      // Activate and copy format from active cell
      if (sheet.activeCell) {
        const id = getCellId(sheet.activeCell.row, sheet.activeCell.col);
        const cellStyle = sheet.data[id]?.style || {};
        copiedFormat.current = { ...cellStyle };
        setFormatPainterActive(true);
      }
    }
  };

  const handleMergeCells = () => {
    if (!ensureWritable('merge')) return;
    if (!sheet.selection) return;
    const { start, end } = sheet.selection;
    const minR = Math.min(start.row, end.row);
    const maxR = Math.max(start.row, end.row);
    const minC = Math.min(start.col, end.col);
    const maxC = Math.max(start.col, end.col);

    // Don't merge single cell
    if (minR === maxR && minC === maxC) return;

    const newMerge = {
      startRow: minR,
      startCol: minC,
      endRow: maxR,
      endCol: maxC
    };

    const newSheet = {
      ...sheet,
      mergedCells: [...(sheet.mergedCells || []), newMerge]
    };
    saveState(newSheet);
  };

  const handleUnmergeCells = () => {
    if (!ensureWritable('merge')) return;
    if (!sheet.selection || !sheet.mergedCells) return;
    const { start } = sheet.selection;

    // Find merge that contains this cell
    const mergeIndex = sheet.mergedCells.findIndex(m =>
      start.row >= m.startRow && start.row <= m.endRow &&
      start.col >= m.startCol && start.col <= m.endCol
    );

    if (mergeIndex === -1) return;

    const newMergedCells = [...sheet.mergedCells];
    newMergedCells.splice(mergeIndex, 1);

    const newSheet = {
      ...sheet,
      mergedCells: newMergedCells
    };
    saveState(newSheet);
  };

  const isCellMerged = () => {
    if (!sheet.selection || !sheet.mergedCells) return false;
    const { start } = sheet.selection;
    return sheet.mergedCells.some(m =>
      start.row >= m.startRow && start.row <= m.endRow &&
      start.col >= m.startCol && start.col <= m.endCol
    );
  };

  const handleInsertRowAbove = useCallback(() => {
    if (!ensureWritable("qator qo'shish")) return;
    if (!contextMenu.show) return;
    const targetRow = contextMenu.row;

    const newData = { ...sheet.data };
    const editsToSend: Array<{ row: number; col: number; value: string }> = [];

    // Shift all cells from targetRow downwards
    for (let r = sheet.rowCount - 1; r >= targetRow; r--) {
      for (let c = 0; c < NUM_COLS; c++) {
        const oldId = getCellId(r, c);
        const newId = getCellId(r + 1, c);
        if (newData[oldId]) {
          newData[newId] = { ...newData[oldId] };
          editsToSend.push({ row: r + 1, col: c, value: newData[oldId].value });
        }
      }
    }

    // Clear the target row
    for (let c = 0; c < NUM_COLS; c++) {
      const id = getCellId(targetRow, c);
      newData[id] = { value: '', computed: '' };
      editsToSend.push({ row: targetRow, col: c, value: '' });
    }

    const newSheet = { ...sheet, data: recomputeSheet(newData), rowCount: sheet.rowCount + 1 };
    saveState(newSheet);
    if (realtimeClient && editsToSend.length > 0) {
      realtimeClient.sendBatch(editsToSend);
    }
    handleCloseContextMenu();
  }, [contextMenu, sheet, saveState, realtimeClient, ensureWritable]);

  const handleInsertRowBelow = useCallback(() => {
    if (!ensureWritable("qator qo'shish")) return;
    if (!contextMenu.show) return;
    const targetRow = contextMenu.row + 1;

    const newData = { ...sheet.data };
    const editsToSend: Array<{ row: number; col: number; value: string }> = [];

    // Shift all cells from targetRow downwards
    for (let r = sheet.rowCount - 1; r >= targetRow; r--) {
      for (let c = 0; c < NUM_COLS; c++) {
        const oldId = getCellId(r, c);
        const newId = getCellId(r + 1, c);
        if (newData[oldId]) {
          newData[newId] = { ...newData[oldId] };
          editsToSend.push({ row: r + 1, col: c, value: newData[oldId].value });
        }
      }
    }

    // Clear the target row
    for (let c = 0; c < NUM_COLS; c++) {
      const id = getCellId(targetRow, c);
      newData[id] = { value: '', computed: '' };
      editsToSend.push({ row: targetRow, col: c, value: '' });
    }

    const newSheet = { ...sheet, data: recomputeSheet(newData), rowCount: sheet.rowCount + 1 };
    saveState(newSheet);
    if (realtimeClient && editsToSend.length > 0) {
      realtimeClient.sendBatch(editsToSend);
    }
    handleCloseContextMenu();
  }, [contextMenu, sheet, saveState, realtimeClient, ensureWritable]);

  const handleDeleteRow = useCallback(() => {
    if (!ensureWritable("qator o'chirish")) return;
    if (!contextMenu.show) return;
    const targetRow = contextMenu.row;

    const newData = { ...sheet.data };
    const editsToSend: Array<{ row: number; col: number; value: string }> = [];

    // Shift all cells from targetRow+1 upwards
    for (let r = targetRow + 1; r < sheet.rowCount; r++) {
      for (let c = 0; c < NUM_COLS; c++) {
        const oldId = getCellId(r, c);
        const newId = getCellId(r - 1, c);
        if (newData[oldId]) {
          newData[newId] = { ...newData[oldId] };
          editsToSend.push({ row: r - 1, col: c, value: newData[oldId].value });
        } else {
          newData[newId] = { value: '', computed: '' };
          editsToSend.push({ row: r - 1, col: c, value: '' });
        }
      }
    }

    // Clear the last row
    for (let c = 0; c < NUM_COLS; c++) {
      const id = getCellId(sheet.rowCount - 1, c);
      delete newData[id];
    }

    const newSheet = { ...sheet, data: recomputeSheet(newData), rowCount: Math.max(NUM_ROWS, sheet.rowCount - 1) };
    saveState(newSheet);
    if (realtimeClient && editsToSend.length > 0) {
      realtimeClient.sendBatch(editsToSend);
    }
    handleCloseContextMenu();
  }, [contextMenu, sheet, saveState, realtimeClient, ensureWritable]);

  const handleInsertColLeft = useCallback(() => {
    if (!ensureWritable("ustun qo'shish")) return;
    if (!contextMenu.show) return;
    const targetCol = contextMenu.col;

    const newData = { ...sheet.data };
    const editsToSend: Array<{ row: number; col: number; value: string }> = [];

    // Shift all cells from targetCol rightwards
    for (let c = NUM_COLS - 1; c >= targetCol; c--) {
      for (let r = 0; r < sheet.rowCount; r++) {
        const oldId = getCellId(r, c);
        const newId = getCellId(r, c + 1);
        if (newData[oldId] && c + 1 < NUM_COLS) {
          newData[newId] = { ...newData[oldId] };
          editsToSend.push({ row: r, col: c + 1, value: newData[oldId].value });
        }
      }
    }

    // Clear the target column
    for (let r = 0; r < sheet.rowCount; r++) {
      const id = getCellId(r, targetCol);
      newData[id] = { value: '', computed: '' };
      editsToSend.push({ row: r, col: targetCol, value: '' });
    }

    const newSheet = { ...sheet, data: recomputeSheet(newData) };
    saveState(newSheet);
    if (realtimeClient && editsToSend.length > 0) {
      realtimeClient.sendBatch(editsToSend);
    }
    handleCloseContextMenu();
  }, [contextMenu, sheet, saveState, realtimeClient, ensureWritable]);

  const handleInsertColRight = useCallback(() => {
    if (!ensureWritable("ustun qo'shish")) return;
    if (!contextMenu.show) return;
    const targetCol = contextMenu.col + 1;

    const newData = { ...sheet.data };
    const editsToSend: Array<{ row: number; col: number; value: string }> = [];

    // Shift all cells from targetCol rightwards
    for (let c = NUM_COLS - 1; c >= targetCol; c--) {
      for (let r = 0; r < sheet.rowCount; r++) {
        const oldId = getCellId(r, c);
        const newId = getCellId(r, c + 1);
        if (newData[oldId] && c + 1 < NUM_COLS) {
          newData[newId] = { ...newData[oldId] };
          editsToSend.push({ row: r, col: c + 1, value: newData[oldId].value });
        }
      }
    }

    // Clear the target column
    for (let r = 0; r < sheet.rowCount; r++) {
      const id = getCellId(r, targetCol);
      newData[id] = { value: '', computed: '' };
      editsToSend.push({ row: r, col: targetCol, value: '' });
    }

    const newSheet = { ...sheet, data: recomputeSheet(newData) };
    saveState(newSheet);
    if (realtimeClient && editsToSend.length > 0) {
      realtimeClient.sendBatch(editsToSend);
    }
    handleCloseContextMenu();
  }, [contextMenu, sheet, saveState, realtimeClient, ensureWritable]);

  const handleDeleteCol = useCallback(() => {
    if (!ensureWritable("ustun o'chirish")) return;
    if (!contextMenu.show) return;
    const targetCol = contextMenu.col;

    const newData = { ...sheet.data };
    const editsToSend: Array<{ row: number; col: number; value: string }> = [];

    // Shift all cells from targetCol+1 leftwards
    for (let c = targetCol + 1; c < NUM_COLS; c++) {
      for (let r = 0; r < sheet.rowCount; r++) {
        const oldId = getCellId(r, c);
        const newId = getCellId(r, c - 1);
        if (newData[oldId]) {
          newData[newId] = { ...newData[oldId] };
          editsToSend.push({ row: r, col: c - 1, value: newData[oldId].value });
        } else {
          newData[newId] = { value: '', computed: '' };
          editsToSend.push({ row: r, col: c - 1, value: '' });
        }
      }
    }

    // Clear the last column
    for (let r = 0; r < sheet.rowCount; r++) {
      const id = getCellId(r, NUM_COLS - 1);
      delete newData[id];
    }

    const newSheet = { ...sheet, data: recomputeSheet(newData) };
    saveState(newSheet);
    if (realtimeClient && editsToSend.length > 0) {
      realtimeClient.sendBatch(editsToSend);
    }
    handleCloseContextMenu();
  }, [contextMenu, sheet, saveState, realtimeClient, ensureWritable]);

  const handleExport = () => {
    // Optimized CSV export - only export rows with data
    let csvContent = "data:text/csv;charset=utf-8,";
    const maxRow = getHighestRowIndex(sheet.data) + 1;
    const maxCol = NUM_COLS;

    for (let r = 0; r < Math.max(maxRow, 1); r++) {
      const rowData = [];
      for (let c = 0; c < maxCol; c++) {
        const id = getCellId(r, c);
        // Escape quotes
        const val = (sheet.data[id]?.computed || '').toString().replace(/"/g, '""');
        rowData.push(`"${val}"`);
      }
      csvContent += rowData.join(",") + "\r\n";
    }

    const encodedUri = encodeURI(csvContent);
    const link = document.createElement("a");
    link.setAttribute("href", encodedUri);
    link.setAttribute("download", "sheet_data.csv");
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  const handleImportFile = useCallback(async (file: File) => {
    if (!ensureWritable('import')) return;
    try {
      console.log('Import started:', file.name, file.type);
      setErrorMessage(null);

      // Check file type
      const ext = file.name.split('.').pop()?.toLowerCase();
      console.log('File extension:', ext);

      if (ext === 'csv') {
        // Handle CSV directly in browser
        const csvText = await file.text();
        console.log('CSV text length:', csvText.length);
        console.log('CSV preview:', csvText.substring(0, 200));

        const rows = parseCsv(csvText);
        console.log('Parsed rows:', rows.length);
        console.log('First 3 rows:', rows.slice(0, 3));

        const baseName = file.name.replace(/\.csv$/i, '');
        console.log('Applying rows with name:', baseName);

        applyImportedRows(rows, baseName || fileName);
        console.log('Import completed');
      } else if (ext === 'xlsx' || ext === 'xls') {
        // Try backend for Excel files
        try {
          const { blob } = await convertExcel(file, token || undefined);
          const csvText = await blob.text();
          const rows = parseCsv(csvText);
          const baseName = file.name.replace(/\.(xlsx|xls)$/i, '');
          applyImportedRows(rows, baseName || fileName);
        } catch (backendErr) {
          console.error('Excel import error:', backendErr);
          setErrorMessage('Excel import server mavjud emas. Faqat CSV fayllarni import qilish mumkin.');
        }
      } else {
        setErrorMessage('Faqat CSV, XLS, XLSX fayllar qo\'llab-quvvatlanadi');
      }
    } catch (err) {
      console.error('Import error:', err);
      const msg = err instanceof Error ? err.message : 'Import failed';
      setErrorMessage(`Import xatosi: ${msg}`);
    }
  }, [token, applyImportedRows, fileName, ensureWritable]);

  // Keyboard Navigation
	  useEffect(() => {
	    const handleKeyDown = (e: KeyboardEvent) => {
	      const keyLower = typeof e.key === 'string' ? e.key.toLowerCase() : '';
	      const activeEl = document.activeElement as HTMLElement | null;
	      const isClipboardCatcher = activeEl?.dataset?.sheetmasterClipboard === 'true';
	      const isTextInput = !!activeEl && (
	        activeEl.tagName === 'INPUT' ||
	        activeEl.tagName === 'TEXTAREA' ||
	        activeEl.isContentEditable
	      );
	      const allowNativeClipboard = isTextInput && !isClipboardCatcher;
	      // Handle Undo/Redo globally
	      if ((e.ctrlKey || e.metaKey) && e.key === 'z') {
	        e.preventDefault();
	        undo();
	        return;
      }
      if ((e.ctrlKey || e.metaKey) && e.key === 'y') {
        e.preventDefault();
        redo();
        return;
      }
      if ((e.ctrlKey || e.metaKey) && keyLower === 'f') {
        e.preventDefault();
        setFindMode('find');
        setFindOpen(true);
        return;
      }
      if ((e.ctrlKey || e.metaKey) && keyLower === 'h') {
        e.preventDefault();
        if (!ensureWritable("almashtirish")) {
          setFindMode('find');
        } else {
          setFindMode('replace');
        }
        setFindOpen(true);
        return;
      }
      if ((e.ctrlKey || e.metaKey) && keyLower === 'g') {
        e.preventDefault();
        const el = document.getElementById(NAME_BOX_ID) as HTMLInputElement | null;
        el?.focus();
        el?.select();
        return;
      }
      if ((e.ctrlKey || e.metaKey) && e.shiftKey && keyLower === 'n') {
        e.preventDefault();
        if (!ensureWritable("template qo'llash")) return;
        setTemplatesOpen(true);
	        return;
	      }
	      // Handle clipboard operations
	      if ((e.ctrlKey || e.metaKey) && keyLower === 'c') {
	        if (allowNativeClipboard) return;
	        e.preventDefault();
	        handleCopy();
	        return;
	      }
	      if ((e.ctrlKey || e.metaKey) && keyLower === 'x') {
	        if (allowNativeClipboard) return;
	        e.preventDefault();
	        handleCut();
	        return;
	      }
	      if ((e.ctrlKey || e.metaKey) && keyLower === 'v') {
	        if (allowNativeClipboard) return;
	        // Let the native paste event fire (we handle it via `window.addEventListener('paste', ...)`).
	        focusClipboardCatcher();
	        return;
	      }

	      // Don't capture if focus is in input
	      if (isTextInput && !isClipboardCatcher) return;
	      if (!sheet.activeCell) return;

      let { row, col } = sheet.activeCell;

      const key = typeof e.key === 'string' ? e.key : '';
      if (!key) return;

      switch (key) {
        case 'ArrowUp': row = Math.max(0, row - 1); break;
        case 'ArrowDown': row = row + 1; break;
        case 'ArrowLeft': col = Math.max(0, col - 1); break;
        case 'ArrowRight': col = Math.min(NUM_COLS - 1, col + 1); break;
        case 'Delete':
        case 'Backspace':
          e.preventDefault();
          handleDelete();
          return;
        case 'Enter':
          e.preventDefault();
          startEditing(row, col);
          return;
        default:
          break;
      }

      if (key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
        e.preventDefault();
        startEditing(row, col, key);
        return;
      }

      // Only prevent default for arrow keys
      if (['ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'].includes(key)) {
        e.preventDefault();
        const targetRow = Math.max(0, row);
        const targetCol = Math.max(0, Math.min(NUM_COLS - 1, col));
        setSheet(prev => ({
          ...prev,
          rowCount: targetRow >= prev.rowCount ? ensureRowCountForIndex(prev.rowCount, targetRow) : prev.rowCount,
          activeCell: { row: targetRow, col: targetCol },
          selection: e.shiftKey && prev.activeCell
            ? { start: prev.selection?.start || { row: targetRow, col: targetCol }, end: { row: targetRow, col: targetCol } }
            : { start: { row: targetRow, col: targetCol }, end: { row: targetRow, col: targetCol } }
        }));
      }
    };

	    window.addEventListener('keydown', handleKeyDown);
	    return () => window.removeEventListener('keydown', handleKeyDown);
	  }, [sheet.activeCell, undo, redo, startEditing, handleCopy, handleCut, handleDelete, ensureWritable, focusClipboardCatcher]);

  // Handle paste event for clipboard text/files (Ctrl/Cmd+V)
  useEffect(() => {
    const handlePasteEvent = async (e: ClipboardEvent) => {
      const activeEl = document.activeElement as HTMLElement | null;
      const isClipboardCatcher = activeEl?.dataset?.sheetmasterClipboard === 'true';
      const isTextInput = !!activeEl && (
        activeEl.tagName === 'INPUT' ||
        activeEl.tagName === 'TEXTAREA' ||
        activeEl.isContentEditable
      );

      // Don't hijack paste inside real inputs (formula bar, modals, etc.)
      if (isTextInput && !isClipboardCatcher) return;
      // We only rely on paste events when the hidden catcher is focused.
      if (!isClipboardCatcher) return;

      // Files pasted from clipboard (e.g., Excel/CSV)
      if (e.clipboardData?.files?.length) {
        e.preventDefault();
        const file = e.clipboardData.files[0];
        const ext = file.name.split('.').pop()?.toLowerCase();
        if (ext === 'csv' || ext === 'xlsx' || ext === 'xls') {
          await handleImportFile(file);
        } else {
          setErrorMessage('Faqat CSV yoki Excel fayllarni import qilish mumkin.');
        }
        return;
      }

      const text = e.clipboardData?.getData('text/plain') ?? '';
      if (!text.trim()) return;

      e.preventDefault();
      if (!ensureWritable("qo'yish")) return;
      const normalized = text.replace(/\r\n/g, '\n').replace(/\r/g, '\n');
      const normalizedText = normalized.endsWith('\n') ? normalized.slice(0, -1) : normalized;
      const lastCopied = lastCopiedTextRef.current;
      const shouldUseInternalClipboard = !!clipboard && (
        clipboard.isCut ||
        (!!lastCopied && normalizedText === lastCopied)
      );

      if (shouldUseInternalClipboard) {
        pasteInternalClipboardAtActiveCell();
      } else {
        pasteTextAtActiveCell(normalizedText);
      }
    };

    window.addEventListener('paste', handlePasteEvent);
    return () => window.removeEventListener('paste', handlePasteEvent);
  }, [handleImportFile, ensureWritable, pasteTextAtActiveCell, pasteInternalClipboardAtActiveCell, clipboard]);

  const getActiveCellStyle = (): any => {
    if (!sheet.activeCell) return {};
    const id = getCellId(sheet.activeCell.row, sheet.activeCell.col);
    return sheet.data[id]?.style || {};
  };

  const applyCellEdits = useCallback((edits: Array<{ row: number; col: number; value: string }>, sourceLabel = "AI o'zgarish") => {
    if (!ensureWritable(sourceLabel)) return;
    if (!edits.length) return;

    const base = latestSheetRef.current;
    const nextData: GridData = { ...base.data };
    let maxRow = -1;

    edits.forEach(({ row, col, value }) => {
      if (row > maxRow) maxRow = row;
      const id = getCellId(row, col);
      const currentCell = nextData[id] || { value: '' };
      nextData[id] = { ...currentCell, value };
    });

    const computed = recomputeSheet(nextData);
    const rowCount = maxRow >= 0 ? ensureRowCountForIndex(base.rowCount, maxRow) : base.rowCount;
    const nextSheet = { ...base, rowCount, data: computed };
    saveState(nextSheet);

    if (realtimeClient) {
      if (edits.length === 1) {
        const single = edits[0];
        realtimeClient.sendCellEdit(single.row, single.col, single.value);
      } else {
        realtimeClient.sendBatch(edits);
      }
    }
  }, [ensureWritable, saveState, realtimeClient]);

  const applyAiAction = useCallback((action: any) => {
    if (!action || typeof action !== 'object') return;
    const kind = action.action;
    if (typeof kind !== 'string') return;

    const base = latestSheetRef.current;

    const toInt = (v: any): number | null => {
      const n = Number(v);
      if (!Number.isFinite(n)) return null;
      return Math.trunc(n);
    };

    const countBefore = (sorted: number[], value: number): number => {
      let lo = 0;
      let hi = sorted.length;
      while (lo < hi) {
        const mid = (lo + hi) >> 1;
        if (sorted[mid] < value) lo = mid + 1;
        else hi = mid;
      }
      return lo;
    };

    const diffValues = (oldData: GridData, newData: GridData): Array<{ row: number; col: number; value: string }> => {
      const edits: Array<{ row: number; col: number; value: string }> = [];
      const keys = new Set<string>([...Object.keys(oldData), ...Object.keys(newData)]);
      keys.forEach((id) => {
        const oldVal = (oldData[id]?.value ?? '').toString();
        const newVal = (newData[id]?.value ?? '').toString();
        if (oldVal === newVal) return;
        const [rowStr, colStr] = id.split(',');
        const row = parseInt(rowStr, 10);
        const col = parseInt(colStr, 10);
        if (!Number.isFinite(row) || !Number.isFinite(col) || row < 0 || col < 0) return;
        edits.push({ row, col, value: newVal });
      });
      return edits;
    };

    const broadcast = (edits: Array<{ row: number; col: number; value: string }>) => {
      if (!realtimeClient || edits.length === 0) return;
      if (edits.length === 1) {
        const single = edits[0];
        realtimeClient.sendCellEdit(single.row, single.col, single.value);
      } else {
        realtimeClient.sendBatch(edits);
      }
    };

    if (kind === 'clear_range') {
      if (!ensureWritable("AI tozalash")) return;
      const startRow = toInt(action.startRow);
      const endRow = toInt(action.endRow);
      const startCol = toInt(action.startCol);
      const endCol = toInt(action.endCol);
      if (startRow === null || endRow === null || startCol === null || endCol === null) {
        setErrorMessage('AI: clear_range parametrlari noto‘g‘ri');
        return;
      }

      const minR = Math.max(0, Math.min(startRow, endRow));
      const maxR = Math.min(Math.max(startRow, endRow), Math.max(0, base.rowCount - 1));
      const minC = Math.max(0, Math.min(startCol, endCol));
      const maxC = Math.min(Math.max(startCol, endCol), NUM_COLS - 1);

      const edits: Array<{ row: number; col: number; value: string }> = [];
      Object.entries(base.data).forEach(([id, cell]) => {
        const [rowStr, colStr] = id.split(',');
        const row = parseInt(rowStr, 10);
        const col = parseInt(colStr, 10);
        if (!Number.isFinite(row) || !Number.isFinite(col)) return;
        if (row < minR || row > maxR || col < minC || col > maxC) return;
        const oldVal = (cell?.value ?? '').toString();
        if (oldVal === '') return;
        edits.push({ row, col, value: '' });
      });
      applyCellEdits(edits, "AI tozalash");
      return;
    }

    if (kind === 'sort_range') {
      if (!ensureWritable('AI sort')) return;
      const startRow = toInt(action.startRow);
      const endRow = toInt(action.endRow);
      const startCol = toInt(action.startCol);
      const endCol = toInt(action.endCol);
      const sortColRaw = toInt(action.sortCol ?? action.sort_col ?? action.sortColumn);
      const direction = action.direction === 'desc' ? 'desc' : 'asc';
      const hasHeader = !!action.hasHeader;

      if (startRow === null || endRow === null || startCol === null || endCol === null) {
        setErrorMessage('AI: sort_range parametrlari noto‘g‘ri');
        return;
      }

      const minR = Math.max(0, Math.min(startRow, endRow));
      const maxR = Math.min(Math.max(startRow, endRow), Math.max(0, base.rowCount - 1));
      const minC = Math.max(0, Math.min(startCol, endCol));
      const maxC = Math.min(Math.max(startCol, endCol), NUM_COLS - 1);
      const sortCol = sortColRaw !== null && sortColRaw >= minC && sortColRaw <= maxC ? sortColRaw : minC;

      const bodyStart = hasHeader ? minR + 1 : minR;
      if (bodyStart >= maxR) {
        setErrorMessage('AI: sort uchun kamida 2 qator kerak');
        return;
      }

      const rows: Array<{
        originalIndex: number;
        keyRaw: string;
        keyNum: number | null;
        cells: Array<CellData | undefined>;
      }> = [];

      for (let r = bodyStart; r <= maxR; r++) {
        const keyId = getCellId(r, sortCol);
        const keyCell = base.data[keyId];
        const keyRaw = (keyCell?.computed ?? keyCell?.value ?? '').toString();
        const parsed = parseFloat(keyRaw);
        const keyNum = keyRaw.trim() !== '' && !Number.isNaN(parsed) ? parsed : null;

        const cells: Array<CellData | undefined> = [];
        for (let c = minC; c <= maxC; c++) {
          cells.push(base.data[getCellId(r, c)]);
        }
        rows.push({ originalIndex: r - bodyStart, keyRaw, keyNum, cells });
      }

      const compare = (a: typeof rows[number], b: typeof rows[number]) => {
        if (a.keyNum !== null && b.keyNum !== null) return a.keyNum - b.keyNum;
        const aStr = a.keyRaw.toLowerCase();
        const bStr = b.keyRaw.toLowerCase();
        return aStr.localeCompare(bStr, undefined, { numeric: true, sensitivity: 'base' });
      };

      rows.sort((a, b) => {
        const baseCmp = compare(a, b);
        const ordered = direction === 'asc' ? baseCmp : -baseCmp;
        return ordered !== 0 ? ordered : a.originalIndex - b.originalIndex;
      });

      const oldData = base.data;
      const newData: GridData = { ...oldData };
      const editsToSend: Array<{ row: number; col: number; value: string }> = [];

      for (let i = 0; i < rows.length; i++) {
        const targetRow = bodyStart + i;
        const sourceRowCells = rows[i].cells;
        for (let c = minC; c <= maxC; c++) {
          const targetId = getCellId(targetRow, c);
          const sourceCell = sourceRowCells[c - minC];
          const oldVal = oldData[targetId]?.value ?? '';

          if (sourceCell) newData[targetId] = { ...sourceCell };
          else delete newData[targetId];

          const newVal = sourceCell?.value ?? '';
          if (newVal !== oldVal) editsToSend.push({ row: targetRow, col: c, value: newVal });
        }
      }

      const nextSheet = { ...base, data: recomputeSheet(newData) };
      saveState(nextSheet);
      broadcast(editsToSend);
      return;
    }

    if (kind === 'delete_rows') {
      if (!ensureWritable("AI qator o'chirish")) return;
      const raw = Array.isArray(action.rows) ? action.rows : [];
      const rows = Array.from(new Set(raw.map(toInt).filter((v): v is number => v !== null)))
        .map((v) => Math.max(0, v))
        .filter((v) => v < base.rowCount)
        .sort((a, b) => a - b);

      if (rows.length === 0) {
        setErrorMessage('AI: delete_rows uchun rowlar topilmadi');
        return;
      }

      const deleteSet = new Set(rows);
      const newData: GridData = {};
      Object.entries(base.data).forEach(([id, cell]) => {
        const [rowStr, colStr] = id.split(',');
        const row = parseInt(rowStr, 10);
        const col = parseInt(colStr, 10);
        if (!Number.isFinite(row) || !Number.isFinite(col)) return;
        if (deleteSet.has(row)) return;
        const shift = countBefore(rows, row);
        const nextRow = row - shift;
        if (nextRow < 0) return;
        newData[getCellId(nextRow, col)] = { ...cell };
      });

      const baseRowCount = Math.max(NUM_ROWS, base.rowCount - rows.length);
      const highest = getHighestRowIndex(newData);
      const rowCount = highest >= 0 ? ensureRowCountForIndex(baseRowCount, highest) : baseRowCount;

      const shiftRow = (row: number) => row - countBefore(rows, row);
      const nextActive = base.activeCell
        ? {
            row: Math.max(0, Math.min(rowCount - 1, shiftRow(base.activeCell.row))),
            col: Math.max(0, Math.min(NUM_COLS - 1, base.activeCell.col)),
          }
        : null;
      const nextSelection = base.selection
        ? {
            start: {
              row: Math.max(0, Math.min(rowCount - 1, shiftRow(base.selection.start.row))),
              col: Math.max(0, Math.min(NUM_COLS - 1, base.selection.start.col)),
            },
            end: {
              row: Math.max(0, Math.min(rowCount - 1, shiftRow(base.selection.end.row))),
              col: Math.max(0, Math.min(NUM_COLS - 1, base.selection.end.col)),
            },
          }
        : null;

      const editsToSend = diffValues(base.data, newData);
      const nextSheet = {
        ...base,
        rowCount,
        data: recomputeSheet(newData),
        activeCell: nextActive,
        selection: nextSelection,
      };
      saveState(nextSheet);
      broadcast(editsToSend);
      return;
    }

    if (kind === 'delete_cols') {
      if (!ensureWritable("AI ustun o'chirish")) return;
      const raw = Array.isArray(action.cols) ? action.cols : [];
      const cols = Array.from(new Set(raw.map(toInt).filter((v): v is number => v !== null)))
        .map((v) => Math.max(0, v))
        .filter((v) => v < NUM_COLS)
        .sort((a, b) => a - b);

      if (cols.length === 0) {
        setErrorMessage('AI: delete_cols uchun collar topilmadi');
        return;
      }

      const deleteSet = new Set(cols);
      const newData: GridData = {};
      Object.entries(base.data).forEach(([id, cell]) => {
        const [rowStr, colStr] = id.split(',');
        const row = parseInt(rowStr, 10);
        const col = parseInt(colStr, 10);
        if (!Number.isFinite(row) || !Number.isFinite(col)) return;
        if (deleteSet.has(col)) return;
        const shift = countBefore(cols, col);
        const nextCol = col - shift;
        if (nextCol < 0 || nextCol >= NUM_COLS) return;
        newData[getCellId(row, nextCol)] = { ...cell };
      });

      const shiftCol = (col: number) => col - countBefore(cols, col);
      const nextActive = base.activeCell
        ? {
            row: Math.max(0, Math.min(Math.max(0, base.rowCount - 1), base.activeCell.row)),
            col: Math.max(0, Math.min(NUM_COLS - 1, shiftCol(base.activeCell.col))),
          }
        : null;
      const nextSelection = base.selection
        ? {
            start: {
              row: Math.max(0, Math.min(Math.max(0, base.rowCount - 1), base.selection.start.row)),
              col: Math.max(0, Math.min(NUM_COLS - 1, shiftCol(base.selection.start.col))),
            },
            end: {
              row: Math.max(0, Math.min(Math.max(0, base.rowCount - 1), base.selection.end.row)),
              col: Math.max(0, Math.min(NUM_COLS - 1, shiftCol(base.selection.end.col))),
            },
          }
        : null;

      const editsToSend = diffValues(base.data, newData);
      const nextSheet = {
        ...base,
        data: recomputeSheet(newData),
        activeCell: nextActive,
        selection: nextSelection,
      };
      saveState(nextSheet);
      broadcast(editsToSend);
      return;
    }

    setErrorMessage(`AI action qo‘llab-quvvatlanmaydi: ${kind}`);
  }, [applyCellEdits, ensureWritable, realtimeClient, saveState]);

  const activeCellLabel = sheet.activeCell ? `${getColumnLabel(sheet.activeCell.col)}${sheet.activeCell.row + 1}` : '';
  const selectionLabel = (() => {
    if (!sheet.selection) return '';
    const { start, end } = sheet.selection;
    const minR = Math.min(start.row, end.row);
    const maxR = Math.max(start.row, end.row);
    const minC = Math.min(start.col, end.col);
    const maxC = Math.max(start.col, end.col);
    const range = `${getColumnLabel(minC)}${minR + 1}:${getColumnLabel(maxC)}${maxR + 1}`;
    const rows = maxR - minR + 1;
    const cols = maxC - minC + 1;
    const cells = rows * cols;
    if (cells === 1) return `Range: ${range}`;
    return `Range: ${range} (${rows}×${cols}, ${cells})`;
  })();


  return (
    <div className="flex flex-col h-screen w-screen overflow-hidden" style={{ background: 'var(--bg-gradient)' }}>
      <Header
        fileName={fileName}
        currentFileId={currentFileId}
        currentAccessRole={currentAccessRole}
        user={user}
        files={files}
        apiBase={API_BASE}
        authToken={token}
        onFileNameChange={setFileName}
        onNewFile={handleNewFile}
        onOpenTemplates={currentAccessRole === 'viewer' ? undefined : () => setTemplatesOpen(true)}
        onSaveFile={handleSaveFile}
        onShareFile={() => setShareOpen(true)}
        onOpenFile={handleSelectFile}
        onDeleteFile={handleDeleteFile}
        onLogout={handleLogout}
        onExport={handleExport}
        onImportFile={currentAccessRole === 'viewer' ? undefined : handleImportFile}
        onShowProfile={() => setShowProfile(true)}
      />

      <Profile
        isOpen={showProfile}
        onClose={() => setShowProfile(false)}
        onSaved={(prefs) => setUiDensity(prefs.density)}
      />

      {token && currentFileId !== null && (
        <ShareModal
          isOpen={shareOpen}
          fileId={currentFileId}
          fileName={fileName}
          token={token}
          onClose={() => setShareOpen(false)}
        />
      )}

      <AuthWall
        isOpen={!user}
        user={user}
        authMode={authMode}
        authForm={authForm}
        authError={authError}
        authLoading={authLoading}
        onChangeAuthForm={handleAuthFormChange}
        onSubmitAuth={handleAuthSubmit}
        onToggleMode={() => setAuthMode(authMode === 'login' ? 'register' : 'login')}
      />

      {user && (
        <>
          <Toolbar
            activeStyle={getActiveCellStyle()}
            onStyleChange={handleStyleChange}
            isAiOpen={isAiOpen}
            onToggleAi={() => setIsAiOpen(!isAiOpen)}
            onOpenFindReplace={() => {
              setFindMode('find');
              setFindOpen(true);
            }}
            onUndo={undo}
            onRedo={redo}
            canUndo={currentAccessRole !== 'viewer' && historyIndex > 0}
            canRedo={currentAccessRole !== 'viewer' && historyIndex < history.length - 1}
            onPrint={handlePrint}
            onFormatPainter={handleFormatPainter}
            formatPainterActive={formatPainterActive}
            onMergeCells={handleMergeCells}
            onUnmergeCells={handleUnmergeCells}
            isMerged={isCellMerged()}
            autoSaveEnabled={autoSaveEnabled}
            onToggleAutoSave={() => setAutoSaveEnabled(!autoSaveEnabled)}
            density={uiDensity}
          />

          <FormulaBar
            activeCellLabel={activeCellLabel}
            value={formulaValue}
            onChange={setFormulaValue}
            onSubmit={handleFormulaSubmit}
            onGoToCell={handleGoToCellLabel}
            readOnly={currentAccessRole === 'viewer'}
            density={uiDensity}
	          />

	          <div className="flex-1 flex overflow-hidden relative">
	            <textarea
	              ref={clipboardCatcherRef}
	              data-sheetmaster-clipboard="true"
	              aria-hidden="true"
	              tabIndex={-1}
	              style={{
	                position: 'fixed',
	                top: 0,
	                left: 0,
	                width: 1,
	                height: 1,
	                opacity: 0,
	                pointerEvents: 'none',
	                zIndex: -1,
	              }}
	            />
	            <Grid
	              data={sheet.data}
	              activeCell={sheet.activeCell}
	              selection={sheet.selection}
	              editingCell={editingCell}
              editingValue={formulaValue}
              columnWidths={sheet.columnWidths}
              rowHeights={sheet.rowHeights}
              rowCount={sheet.rowCount}
              freezePosition={sheet.freezePosition}
              density={uiDensity}
              onCellClick={handleCellClick}
              onSelectionDrag={handleSelectionDrag}
              onEditStart={startEditing}
              onEditChange={setFormulaValue}
              onEditCommit={commitEditing}
              onEditCancel={cancelEditing}
              onColumnResize={handleColumnResize}
              onRowResize={handleRowResize}
              onContextMenu={handleContextMenu}
              onRequestMoreRows={handleRequestMoreRows}
              onMoveSelection={handleMoveSelection}
            />

            {isAiOpen && (
              <Suspense
                fallback={
                  <div
                    className="w-80 flex items-center justify-center text-sm"
                    style={{ borderLeft: '1px solid var(--border-color)', background: 'var(--bg-light)', color: 'var(--text-secondary)' }}
                  >
                    AI assistant yuklanmoqda...
                  </div>
                }
              >
                <GeminiSidebar
                  isOpen={isAiOpen}
                  onClose={() => setIsAiOpen(false)}
                  authToken={token}
                  sheetState={sheet}
                  onApplyChanges={applyCellEdits}
                  onApplyAiAction={applyAiAction}
                />
              </Suspense>
            )}

            <ContextMenu
              show={contextMenu.show}
              x={contextMenu.x}
              y={contextMenu.y}
              onCut={handleCut}
              onCopy={handleCopy}
              onPaste={handlePaste}
              onDelete={handleDelete}
              onInsertRowAbove={handleInsertRowAbove}
              onInsertRowBelow={handleInsertRowBelow}
              onInsertColLeft={handleInsertColLeft}
              onInsertColRight={handleInsertColRight}
              onDeleteRow={handleDeleteRow}
              onDeleteCol={handleDeleteCol}
              onSortAsc={handleSortAsc}
              onSortDesc={handleSortDesc}
              onClose={handleCloseContextMenu}
            />

            <ErrorToast message={errorMessage} onClose={() => setErrorMessage(null)} />
          </div>

          <StatusBar
            density={uiDensity}
            activeCellLabel={activeCellLabel}
            selectionLabel={selectionLabel}
            accessRole={currentAccessRole}
            autoSaveEnabled={autoSaveEnabled}
            saveStatus={saveStatus}
            lastSavedAt={lastSavedAt}
            realtimeStatus={realtimeStatus}
          />
        </>
      )}

      <DeleteConfirmModal
        isOpen={pendingDeleteId !== null}
        fileName={
          pendingDeleteId !== null
            ? (files.find((f) => f.id === pendingDeleteId)?.name ?? 'Fayl')
            : ''
        }
        onConfirm={confirmDeleteFile}
        onCancel={() => setPendingDeleteId(null)}
      />

      <OverwriteConfirmModal
        isOpen={overwriteConfirm.show}
        onConfirm={() => {
          if (overwriteConfirm.action) {
            overwriteConfirm.action();
          }
          setOverwriteConfirm({ show: false, action: null });
        }}
        onCancel={() => setOverwriteConfirm({ show: false, action: null })}
      />

      <FindReplaceModal
        isOpen={findOpen}
        mode={findMode}
        query={findQuery}
        replaceText={findReplaceText}
        matchCase={findMatchCase}
        wholeCell={findWholeCell}
        scope={findScope}
        matchCount={findMatches.length}
        currentIndex={findIndex}
        onChangeMode={setFindMode}
        onChangeQuery={setFindQuery}
        onChangeReplaceText={setFindReplaceText}
        onChangeMatchCase={setFindMatchCase}
        onChangeWholeCell={setFindWholeCell}
        onChangeScope={setFindScope}
        onFindNext={handleFindNext}
        onFindPrev={handleFindPrev}
        onReplace={handleReplaceCurrent}
        onReplaceAll={handleReplaceAll}
        onClose={() => setFindOpen(false)}
      />

      <TemplatePickerModal
        isOpen={templatesOpen}
        hasExistingData={hasSheetData}
        onApply={applyImportedRows}
        onClose={() => setTemplatesOpen(false)}
      />
    </div>
  );
};

export default App;
