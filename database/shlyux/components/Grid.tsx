import React, { useState, useEffect, memo, useRef, useCallback, useMemo } from 'react';
import { GridData, SelectionRange, FreezePosition, CellStyle } from '../types';
import { getColumnLabel, getCellId, NUM_COLS } from '../utils/spreadsheetUtils';

interface GridProps {
  data: GridData;
  activeCell: { row: number; col: number } | null;
  selection: SelectionRange | null;
  editingCell: { row: number; col: number } | null;
  editingValue: string;
  columnWidths: Record<number, number>;
  rowHeights: Record<number, number>;
  rowCount: number;
  freezePosition?: FreezePosition;
  density?: 'comfortable' | 'compact';
  onCellClick: (row: number, col: number, isShift: boolean) => void;
  onSelectionDrag: (row: number, col: number) => void;
  onEditStart: (row: number, col: number, initialValue?: string) => void;
  onEditChange: (value: string) => void;
  onEditCommit: (row: number, col: number, move?: { rowDelta?: number; colDelta?: number }) => void;
  onEditCancel: () => void;
  onColumnResize: (col: number, width: number) => void;
  onRowResize: (row: number, height: number) => void;
  onContextMenu: (row: number, col: number, x: number, y: number) => void;
  onRequestMoreRows: () => void;
  onMoveSelection?: (targetRow: number, targetCol: number) => void;
}

const DEFAULT_COL_WIDTH = 100;
const OVERSCAN = 5; // Render extra rows/cols outside viewport

const formatDisplayValue = (value: string | number, style: CellStyle | undefined): string | number => {
  if (typeof value !== 'number' || !style) return value;
  const decimals =
    typeof style.decimalPlaces === 'number'
      ? Math.max(0, style.decimalPlaces)
      : style.numberFormat === 'percent'
        ? 2
        : style.numberFormat === 'currency'
          ? 2
          : undefined;

  const options: Intl.NumberFormatOptions = {};
  if (typeof decimals === 'number') {
    options.minimumFractionDigits = decimals;
    options.maximumFractionDigits = decimals;
  }

  switch (style.numberFormat) {
    case 'number':
      return value.toLocaleString(undefined, options);
    case 'currency':
      return value.toLocaleString(undefined, {
        style: 'currency',
        currency: style.currencyCode || 'USD',
        ...options
      });
    case 'percent':
      return (value * 100).toLocaleString(undefined, options) + '%';
    default:
      return typeof decimals === 'number'
        ? value.toFixed(decimals)
        : value;
  }
};

const Cell = memo(({
  row,
  col,
  data,
  isActive,
  isEditing,
  editValue,
  width,
  height,
  defaultBackgroundColor,
  onClick,
  onMouseDown,
  onMouseOver,
  onEditStart,
  onEditChange,
  onEditCommit,
  onEditCancel,
  onContextMenu
}: {
  row: number;
  col: number;
  data: GridData;
  isActive: boolean;
  isEditing: boolean;
  editValue: string;
  width: number;
  height: number;
  defaultBackgroundColor: string;
  onClick: (row: number, col: number, isShift: boolean) => void;
  onMouseDown: (row: number, col: number) => void;
  onMouseOver: (row: number, col: number, e?: React.MouseEvent) => void;
  onEditStart: (row: number, col: number) => void;
  onEditChange: (value: string) => void;
  onEditCommit: (row: number, col: number, move?: { rowDelta?: number; colDelta?: number }) => void;
  onEditCancel: () => void;
  onContextMenu: (row: number, col: number, e: React.MouseEvent) => void;
}) => {
  const cellId = getCellId(row, col);
  const cellData = data[cellId];
  const style = cellData?.style || {};
  const inputRef = useRef<HTMLInputElement>(null);

  let className = "relative px-1 text-sm cursor-cell select-none ";

  const wrapMode = style.wrapMode || 'overflow';

  const inlineStyle: React.CSSProperties = {
    fontWeight: style.bold ? 'bold' : 'normal',
    fontStyle: style.italic ? 'italic' : 'normal',
    textDecoration: style.underline ? 'underline' : 'none',
    color: style.color || 'var(--text-primary)',
    backgroundColor: style.backgroundColor || defaultBackgroundColor,
    fontFamily: style.fontFamily || 'Inter, system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
    fontSize: `${style.fontSize || 13}px`,
    width: `${width}px`,
    height: `${height}px`,
    border: '1px solid var(--sheet-grid-line)',
    boxSizing: 'border-box',
    overflow: wrapMode === 'clip' ? 'hidden' : 'visible'
  };

  // Apply custom borders if defined
  if (style.borders) {
    const borderColor = style.borders.color || '#000000';
    const borderStyle = style.borders.style || 'solid';
    const borderWidth = '2px';

    if (style.borders.top) {
      inlineStyle.borderTop = `${borderWidth} ${borderStyle} ${borderColor}`;
    }
    if (style.borders.right) {
      inlineStyle.borderRight = `${borderWidth} ${borderStyle} ${borderColor}`;
    }
    if (style.borders.bottom) {
      inlineStyle.borderBottom = `${borderWidth} ${borderStyle} ${borderColor}`;
    }
    if (style.borders.left) {
      inlineStyle.borderLeft = `${borderWidth} ${borderStyle} ${borderColor}`;
    }
  }

  if (isActive) {
    className += "z-10 ";
    inlineStyle.outline = '2px solid var(--sheet-active-cell-outline)';
    inlineStyle.outlineOffset = '-2px';
  }

  const rawDisplayValue = cellData?.computed !== undefined ? cellData.computed : '';
  const displayValue = typeof rawDisplayValue === 'number' || typeof rawDisplayValue === 'string'
    ? formatDisplayValue(rawDisplayValue, style)
    : '';
  const contentWrapperStyle: React.CSSProperties = {
    width: '100%',
    height: '100%',
    display: 'flex',
    flexDirection: 'column',
    justifyContent: style.verticalAlign === 'top'
      ? 'flex-start'
      : style.verticalAlign === 'bottom'
        ? 'flex-end'
        : 'center',
    overflow: wrapMode === 'clip' ? 'hidden' : 'visible'
  };
  const textStyle: React.CSSProperties = {
    width: '100%',
    textAlign: style.textAlign || 'left',
    whiteSpace: wrapMode === 'wrap' ? 'normal' : 'nowrap',
    overflow: wrapMode === 'clip' ? 'hidden' : 'visible',
    textOverflow: wrapMode === 'clip' ? 'clip' : 'unset',
    wordBreak: wrapMode === 'wrap' ? 'break-word' : 'normal'
  };

  useEffect(() => {
    if (isEditing && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [isEditing]);

  return (
    <div
      className={className}
      style={inlineStyle}
      onMouseDown={(e) => {
        if (isEditing) return;
        if (e.shiftKey) {
          onClick(row, col, true);
        } else {
          onMouseDown(row, col);
        }
      }}
      onMouseOver={(e) => {
        if (!isEditing) {
          onMouseOver(row, col, e);
        }
      }}
      onDoubleClick={() => onEditStart(row, col)}
      onContextMenu={(e) => {
        e.preventDefault();
        onContextMenu(row, col, e);
      }}
    >
      {isEditing ? (
        <input
          ref={inputRef}
          value={editValue}
          onChange={(e) => onEditChange(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault();
              onEditCommit(row, col, { rowDelta: e.shiftKey ? -1 : 1 });
            } else if (e.key === 'Tab') {
              e.preventDefault();
              onEditCommit(row, col, { colDelta: e.shiftKey ? -1 : 1 });
            } else if (e.key === 'Escape') {
              e.preventDefault();
              onEditCancel();
            }
          }}
          onBlur={() => onEditCommit(row, col)}
          className="px-1 text-sm focus:outline-none select-text"
          style={{
            position: 'absolute',
            inset: 0,
            width: '100%',
            height: '100%',
            border: 'none',
            backgroundColor: 'var(--card-bg)',
            color: 'var(--text-primary)',
            fontWeight: style.bold ? 'bold' : 'normal',
            fontStyle: style.italic ? 'italic' : 'normal',
            textDecoration: style.underline ? 'underline' : 'none',
            fontFamily: style.fontFamily || 'Inter, system-ui, -apple-system, BlinkMacSystemFont, sans-serif',
            fontSize: `${style.fontSize || 13}px`,
            textAlign: style.textAlign || 'left',
            boxSizing: 'border-box'
          }}
        />
      ) : (
        <div style={contentWrapperStyle}>
          <div style={textStyle}>
            {displayValue as React.ReactNode}
          </div>
        </div>
      )}
    </div>
  );
});

const Grid: React.FC<GridProps> = ({
  data,
  activeCell,
  selection,
  editingCell,
  editingValue,
  columnWidths,
  rowHeights,
  rowCount,
  density,
  onCellClick,
  onSelectionDrag,
  onEditStart,
  onEditChange,
  onEditCommit,
  onEditCancel,
  onColumnResize,
  onRowResize,
  onContextMenu,
  onRequestMoreRows,
  onMoveSelection
}) => {
  const isCompact = density === 'compact';
  const defaultRowHeight = isCompact ? 22 : 26;
  const columnHeaderHeight = isCompact ? 28 : 32;
  const rowHeaderWidth = 40;

  const [isDragging, setIsDragging] = useState(false);
  const [isMovingSelection, setIsMovingSelection] = useState(false);
  const [dragPreviewPos, setDragPreviewPos] = useState<{ row: number; col: number } | null>(null);
  const [dragStartPos, setDragStartPos] = useState<{ x: number; y: number } | null>(null);
  const [dragConfirmed, setDragConfirmed] = useState(false);
  const [lastClickTime, setLastClickTime] = useState(0); // Ikki marta bosishni aniqlash
  const [resizingCol, setResizingCol] = useState<number | null>(null);
  const [resizingRow, setResizingRow] = useState<number | null>(null);
  const [resizeStartX, setResizeStartX] = useState(0);
  const [resizeStartY, setResizeStartY] = useState(0);
  const [resizeStartWidth, setResizeStartWidth] = useState(0);
  const [resizeStartHeight, setResizeStartHeight] = useState(0);
  const [hoveredCol, setHoveredCol] = useState<number | null>(null);
  const [hoveredRow, setHoveredRow] = useState<number | null>(null);
  const [scrollState, setScrollState] = useState({ scrollTop: 0, scrollLeft: 0, clientHeight: 0, clientWidth: 0 });

  const containerRef = useRef<HTMLDivElement>(null);
  const lastDragRef = useRef<{ row: number; col: number } | null>(null);
  const mousePosRef = useRef<{ x: number, y: number } | null>(null);
  const rafRef = useRef<number | null>(null);

  const getColWidth = useCallback((col: number) => columnWidths[col] || DEFAULT_COL_WIDTH, [columnWidths]);
  const getRowHeight = useCallback((row: number) => rowHeights[row] || defaultRowHeight, [rowHeights, defaultRowHeight]);

  // Precompute offsets for fast lookup
  const colOffsets = useMemo(() => {
    const cols: number[] = [rowHeaderWidth];
    for (let c = 0; c < NUM_COLS; c++) {
      cols.push(cols[c] + getColWidth(c));
    }
    return cols;
  }, [getColWidth, rowHeaderWidth]); // Remove columnWidths dependency

  const rowOffsets = useMemo(() => {
    const rows: number[] = [columnHeaderHeight];
    for (let r = 0; r < rowCount; r++) {
      rows.push(rows[r] + getRowHeight(r));
    }
    return rows;
  }, [rowHeights, rowCount, getRowHeight, columnHeaderHeight]);

  // Stable references to prevent useEffect from constantly re-running
  const rowOffsetsRef = useRef(rowOffsets);
  const colOffsetsRef = useRef(colOffsets);
  const onSelectionDragRef = useRef(onSelectionDrag);
  const rowCountRef = useRef(rowCount);

  // Update refs when values change
  useEffect(() => {
    rowOffsetsRef.current = rowOffsets;
    colOffsetsRef.current = colOffsets;
    onSelectionDragRef.current = onSelectionDrag;
    rowCountRef.current = rowCount;
  }, [rowOffsets, colOffsets, onSelectionDrag, rowCount]);

  // Calculate visible range (only for ROWS and COLS - we render all column headers)
  const visibleRange = useMemo(() => {
    const { scrollTop, scrollLeft } = scrollState;
    const containerWidth = containerRef.current?.clientWidth || 1200;
    const containerHeight = containerRef.current?.clientHeight || 800;

    // Find visible row range
    let startRow = 0;
    let endRow = Math.min(rowCount - 1, Math.max(0, Math.floor(containerHeight / defaultRowHeight) + OVERSCAN));

    for (let i = 0; i < rowCount; i++) {
      if (rowOffsets[i + 1] > scrollTop) {
        startRow = Math.max(0, i - OVERSCAN);
        break;
      }
    }

    for (let i = startRow; i < rowCount; i++) {
      if (rowOffsets[i] > scrollTop + containerHeight) {
        endRow = Math.min(rowCount - 1, i + OVERSCAN);
        break;
      }
    }

    // Find visible col range for cells
    let startCol = 0;
    let endCol = NUM_COLS - 1; // Default to all columns

    for (let i = 0; i < NUM_COLS; i++) {
      if (colOffsets[i + 1] > scrollLeft) {
        startCol = Math.max(0, i - OVERSCAN);
        break;
      }
    }

    // Calculate endCol based on visible area
    let foundEndCol = false;
    for (let i = startCol; i < NUM_COLS; i++) {
      if (colOffsets[i] > scrollLeft + containerWidth) {
        endCol = Math.min(NUM_COLS - 1, i + OVERSCAN);
        foundEndCol = true;
        break;
      }
    }
    // If no break occurred, all columns from startCol to end are visible
    if (!foundEndCol) {
      endCol = NUM_COLS - 1;
    }

    return { startRow, endRow, startCol, endCol };
  }, [scrollState, rowCount, rowOffsets, colOffsets]);

  const handleMouseDown = useCallback((row: number, col: number) => {
    if (resizingCol === null && resizingRow === null) {
      setIsDragging(true);
      onCellClick(row, col, false);
    }
  }, [onCellClick, resizingCol, resizingRow]);

  const handleMouseOver = useCallback((row: number, col: number) => {
    // Disabled: Now using global handleWindowMouseMove for better tracking
    // Cell-level mouseOver conflicts with global listener causing selection issues
  }, []);

  const handleMouseUp = useCallback(() => {
    setIsDragging(false);
    setResizingCol(null);
    setResizingRow(null);
  }, []);

  const handleCellContextMenu = useCallback((row: number, col: number, e: React.MouseEvent) => {
    onContextMenu(row, col, e.clientX, e.clientY);
  }, [onContextMenu]);

  const handleColResizeStart = useCallback((col: number, e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
    setResizingCol(col);
    setResizeStartX(e.clientX);
    setResizeStartWidth(getColWidth(col));
  }, [getColWidth]);

  const handleColResizeMove = useCallback((e: MouseEvent) => {
    if (resizingCol === null) return;
    e.preventDefault();
    const delta = e.clientX - resizeStartX;
    const newWidth = Math.max(50, resizeStartWidth + delta);
    onColumnResize(resizingCol, newWidth);
  }, [resizingCol, resizeStartX, resizeStartWidth, onColumnResize]);

  const handleRowResizeStart = useCallback((row: number, e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDragging(false);
    setResizingRow(row);
    setResizeStartY(e.clientY);
    setResizeStartHeight(getRowHeight(row));
  }, [getRowHeight]);

  const handleRowResizeMove = useCallback((e: MouseEvent) => {
    if (resizingRow === null) return;
    e.preventDefault();
    const delta = e.clientY - resizeStartY;
    const newHeight = Math.max(20, resizeStartHeight + delta);
    onRowResize(resizingRow, newHeight);
  }, [resizingRow, resizeStartY, resizeStartHeight, onRowResize]);

  const handleResizeEnd = useCallback(() => {
    setResizingCol(null);
    setResizingRow(null);
  }, []);

  useEffect(() => {
    if (resizingCol !== null) {
      window.addEventListener('mousemove', handleColResizeMove);
      window.addEventListener('mouseup', handleResizeEnd);
      return () => {
        window.removeEventListener('mousemove', handleColResizeMove);
        window.removeEventListener('mouseup', handleResizeEnd);
      };
    }
  }, [resizingCol, handleColResizeMove, handleResizeEnd]);

  useEffect(() => {
    if (resizingRow !== null) {
      window.addEventListener('mousemove', handleRowResizeMove);
      window.addEventListener('mouseup', handleResizeEnd);
      return () => {
        window.removeEventListener('mousemove', handleRowResizeMove);
        window.removeEventListener('mouseup', handleResizeEnd);
      };
    }
  }, [resizingRow, handleRowResizeMove, handleResizeEnd]);

  useEffect(() => {
    window.addEventListener('mouseup', handleMouseUp);
    return () => window.removeEventListener('mouseup', handleMouseUp);
  }, [handleMouseUp]);

  // Auto-scroll logic
  useEffect(() => {
    if (!isDragging) {
      if (rafRef.current) {
        cancelAnimationFrame(rafRef.current);
        rafRef.current = null;
      }
      return;
    }

    const scrollThreshold = 60;
    const maxScrollSpeed = 25;

    const handleWindowMouseMove = (e: MouseEvent) => {
      mousePosRef.current = { x: e.clientX, y: e.clientY };

      // Calculate which cell the mouse is over based on coordinates
      if (containerRef.current) {
        const rect = containerRef.current.getBoundingClientRect();
        const relativeX = e.clientX - rect.left + containerRef.current.scrollLeft;
        const relativeY = e.clientY - rect.top + containerRef.current.scrollTop;

        // Use ref values to avoid stale closures
        const currentRowOffsets = rowOffsetsRef.current;
        const currentColOffsets = colOffsetsRef.current;
        const currentRowCount = rowCountRef.current;

        // Find row
        let targetRow = -1;
        for (let r = 0; r < currentRowCount; r++) {
          const rowTop = currentRowOffsets[r] || 0;
          const rowBottom = currentRowOffsets[r + 1] || rowTop + defaultRowHeight;
          if (relativeY >= rowTop && relativeY < rowBottom) {
            targetRow = r;
            break;
          }
        }

        // Find column
        let targetCol = -1;
        for (let c = 0; c < NUM_COLS; c++) {
          const colLeft = currentColOffsets[c] || 0;
          const colRight = currentColOffsets[c + 1] || colLeft + DEFAULT_COL_WIDTH;
          if (relativeX >= colLeft && relativeX < colRight) {
            targetCol = c;
            break;
          }
        }

        // Trigger selection drag if we found a valid cell
        if (targetRow >= 0 && targetCol >= 0) {
          const last = lastDragRef.current;
          if (!last || last.row !== targetRow || last.col !== targetCol) {
            lastDragRef.current = { row: targetRow, col: targetCol };
            onSelectionDragRef.current(targetRow, targetCol);
          }
        }
      }
    };

    const scrollLoop = () => {
      if (!containerRef.current || !mousePosRef.current) return;

      const { x, y } = mousePosRef.current;
      const { top, bottom, left, right } = containerRef.current.getBoundingClientRect();

      let scrollX = 0;
      let scrollY = 0;

      if (y < top + scrollThreshold) {
        const intensity = Math.max(0, (top + scrollThreshold) - y) / scrollThreshold;
        scrollY = -Math.round(intensity * maxScrollSpeed);
      } else if (y > bottom - scrollThreshold) {
        const intensity = Math.max(0, y - (bottom - scrollThreshold)) / scrollThreshold;
        scrollY = Math.round(intensity * maxScrollSpeed);
      }

      if (x < left + scrollThreshold) {
        const intensity = Math.max(0, (left + scrollThreshold) - x) / scrollThreshold;
        scrollX = -Math.round(intensity * maxScrollSpeed);
      } else if (x > right - scrollThreshold) {
        const intensity = Math.max(0, x - (right - scrollThreshold)) / scrollThreshold;
        scrollX = Math.round(intensity * maxScrollSpeed);
      }

      if (scrollX !== 0 || scrollY !== 0) {
        containerRef.current.scrollBy(scrollX, scrollY);
      }

      rafRef.current = requestAnimationFrame(scrollLoop);
    };

    window.addEventListener('mousemove', handleWindowMouseMove);
    rafRef.current = requestAnimationFrame(scrollLoop);

    return () => {
      window.removeEventListener('mousemove', handleWindowMouseMove);
      if (rafRef.current) {
        cancelAnimationFrame(rafRef.current);
      }
    };
  }, [isDragging, defaultRowHeight]); // Only isDragging - refs keep other values fresh

  // Scroll to active cell only when selection changes (avoid jumping on resize)
  useEffect(() => {
    if (!activeCell || !containerRef.current) return;

    const cellTop = rowOffsets[activeCell.row] || 0;
    const cellLeft = colOffsets[activeCell.col] || 0;
    const cellBottom = rowOffsets[activeCell.row + 1] || cellTop + getRowHeight(activeCell.row);
    const cellRight = colOffsets[activeCell.col + 1] || cellLeft + getColWidth(activeCell.col);

    const { scrollTop, scrollLeft, clientHeight, clientWidth } = containerRef.current;

    if (cellTop < scrollTop) {
      containerRef.current.scrollTop = cellTop;
    } else if (cellBottom > scrollTop + clientHeight) {
      containerRef.current.scrollTop = cellBottom - clientHeight;
    }

    if (cellLeft < scrollLeft) {
      containerRef.current.scrollLeft = cellLeft;
    } else if (cellRight > scrollLeft + clientWidth) {
      containerRef.current.scrollLeft = cellRight - clientWidth;
    }
  }, [activeCell, rowOffsets, colOffsets, getRowHeight, getColWidth]);

  const handleScroll = useCallback((e: React.UIEvent<HTMLDivElement>) => {
    const target = e.currentTarget;
    setScrollState({
      scrollTop: target.scrollTop,
      scrollLeft: target.scrollLeft,
      clientHeight: target.clientHeight,
      clientWidth: target.clientWidth
    });

    // Trigger load more rows
    const totalHeight = rowOffsets[rowCount] || rowCount * defaultRowHeight;
    const distanceFromBottom = totalHeight - (target.scrollTop + target.clientHeight);
    if (distanceFromBottom < 500) {
      onRequestMoreRows();
    }
  }, [rowOffsets, rowCount, onRequestMoreRows, defaultRowHeight]);

  // Selection overlay
  const renderSelectionOverlay = () => {
    if (!selection) return null;
    const minR = Math.min(selection.start.row, selection.end.row);
    const maxR = Math.max(selection.start.row, selection.end.row);
    const minC = Math.min(selection.start.col, selection.end.col);
    const maxC = Math.max(selection.start.col, selection.end.col);

    const left = colOffsets[minC] || rowHeaderWidth;
    const top = rowOffsets[minR] || columnHeaderHeight;
    const width = (colOffsets[maxC + 1] || colOffsets[minC]) - (colOffsets[minC] || rowHeaderWidth);
    const height = (rowOffsets[maxR + 1] || rowOffsets[minR]) - (rowOffsets[minR] || columnHeaderHeight);

    const handleSelectionMouseDown = (e: React.MouseEvent) => {
      if (!onMoveSelection || editingCell) return;

      const now = Date.now();
      const timeSinceLastClick = now - lastClickTime;

      // Ikkinchi marta bosish (300ms ichida) - drag boshlanadi
      if (timeSinceLastClick < 300 && timeSinceLastClick > 0) {
        e.preventDefault();
        e.stopPropagation();
        setIsMovingSelection(true);
        setDragStartPos({ x: e.clientX, y: e.clientY });
        setDragConfirmed(false);
        setLastClickTime(0); // Reset
      } else {
        // Birinchi marta bosish - event ni o'tkazib yuborish (cell tanlanishi uchun)
        // Faqat vaqtni saqlash va event ostidagi elementga o'tkazish
        setLastClickTime(now);
        // Event ni bloklash yo'q - ostidagi cell tanlanadi
      }
    };

    const handleDragMouseMove = useCallback((e: MouseEvent) => {
      if (!isMovingSelection || !containerRef.current || !selection) return;

      // Check if drag is confirmed (moved at least 5px)
      if (!dragConfirmed && dragStartPos) {
        const deltaX = Math.abs(e.clientX - dragStartPos.x);
        const deltaY = Math.abs(e.clientY - dragStartPos.y);
        if (deltaX < 5 && deltaY < 5) {
          return; // Not enough movement yet
        }
        setDragConfirmed(true);
      }

      const rect = containerRef.current.getBoundingClientRect();
      const x = e.clientX - rect.left + containerRef.current.scrollLeft - rowHeaderWidth;
      const y = e.clientY - rect.top + containerRef.current.scrollTop - columnHeaderHeight;

      // Find target cell
      let targetCol = 0;
      for (let c = 0; c < NUM_COLS; c++) {
        if (colOffsets[c + 1] > x) {
          targetCol = c;
          break;
        }
      }

      let targetRow = 0;
      for (let r = 0; r < rowCount; r++) {
        if (rowOffsets[r + 1] > y) {
          targetRow = r;
          break;
        }
      }

      lastDragRef.current = { row: targetRow, col: targetCol };
      setDragPreviewPos({ row: targetRow, col: targetCol });
    }, [isMovingSelection, selection, rowCount, dragConfirmed, dragStartPos]);

    const handleDragMouseUp = useCallback(() => {
      // Only move if drag was confirmed (user moved mouse at least 5px)
      if (isMovingSelection && dragConfirmed && lastDragRef.current && onMoveSelection) {
        onMoveSelection(lastDragRef.current.row, lastDragRef.current.col);
      }
      setIsMovingSelection(false);
      setDragPreviewPos(null);
      setDragStartPos(null);
      setDragConfirmed(false);
      lastDragRef.current = null;
    }, [isMovingSelection, dragConfirmed, onMoveSelection]);

    useEffect(() => {
      if (isMovingSelection) {
        window.addEventListener('mousemove', handleDragMouseMove);
        window.addEventListener('mouseup', handleDragMouseUp);
        return () => {
          window.removeEventListener('mousemove', handleDragMouseMove);
          window.removeEventListener('mouseup', handleDragMouseUp);
        };
      }
    }, [isMovingSelection, handleDragMouseMove, handleDragMouseUp]);

    // Calculate drag preview position
    let previewLeft = left;
    let previewTop = top;
    let hasDataInTarget = false;

    if (dragPreviewPos && isMovingSelection) {
      previewLeft = colOffsets[dragPreviewPos.col] || rowHeaderWidth;
      previewTop = rowOffsets[dragPreviewPos.row] || columnHeaderHeight;

      // Check if target cells have data
      const rowOffset = dragPreviewPos.row - minR;
      const colOffset = dragPreviewPos.col - minC;

      for (let r = minR; r <= maxR; r++) {
        for (let c = minC; c <= maxC; c++) {
          const targetRow = r + rowOffset;
          const targetCol = c + colOffset;
          const targetId = getCellId(targetRow, targetCol);
          if (data[targetId]?.value) {
            hasDataInTarget = true;
            break;
          }
        }
        if (hasDataInTarget) break;
      }
    }

    return (
      <>
        {/* Original selection */}
        <div
          className="absolute"
          style={{
            left,
            top,
            width,
            height,
            border: '2px solid var(--sheet-selection-border)',
            background: isMovingSelection ? 'var(--sheet-selection-fill-weak)' : 'var(--sheet-selection-fill)',
            boxSizing: 'border-box',
            zIndex: 15,
            cursor: isMovingSelection ? 'grabbing' : 'default',
            pointerEvents: 'auto',
            opacity: isMovingSelection ? 0.5 : 1
          }}
          onMouseDown={handleSelectionMouseDown}
        />

        {/* Drag preview - shows where selection will move */}
        {isMovingSelection && dragConfirmed && dragPreviewPos && (
          <div
            className="absolute pointer-events-none"
            style={{
              left: previewLeft,
              top: previewTop,
              width,
              height,
              border: hasDataInTarget
                ? '2px dashed var(--sheet-danger-border)'
                : '2px dashed var(--sheet-selection-border)',
              background: hasDataInTarget ? 'var(--sheet-danger-fill)' : 'var(--sheet-selection-fill)',
              boxSizing: 'border-box',
              zIndex: 20
            }}
          />
        )}
      </>
    );
  };

  const totalWidth = colOffsets[NUM_COLS] || NUM_COLS * DEFAULT_COL_WIDTH;
  const totalHeight = rowOffsets[rowCount] || rowCount * defaultRowHeight;

  return (
    <div
      className="flex-1 overflow-auto relative"
      ref={containerRef}
      style={{ background: 'var(--bg-light)' }}
      onScroll={handleScroll}
    >
      <div style={{ width: totalWidth, height: totalHeight, position: 'relative' }}>
        {/* Column Headers - Position at the very top */}
        <div style={{ position: 'sticky', top: 0, left: 0, zIndex: 10, height: columnHeaderHeight, display: 'flex' }}>
          {/* Corner */}
          <div
            style={{
              width: rowHeaderWidth,
              height: columnHeaderHeight,
              border: '1px solid var(--border-color)',
              background: 'var(--bg-light)',
              position: 'sticky',
              left: 0,
              zIndex: 11,
              flexShrink: 0
            }}
          />

          {/* Column headers container - positioned relative to allow absolute positioning inside */}
          <div style={{ position: 'relative', flex: 1, height: columnHeaderHeight, background: 'transparent' }}>
            {Array.from({ length: visibleRange.endCol - visibleRange.startCol + 1 }).map((_, colIdx) => {
              const c = visibleRange.startCol + colIdx;
              const width = getColWidth(c);
              const isActive = activeCell?.col === c;
              const isHovered = hoveredCol === c;
              const background = isActive
                ? 'var(--sheet-header-active-bg)'
                : isHovered
                  ? 'var(--sheet-header-hover-bg)'
                  : 'var(--bg-light)';
              return (
                <div
                  key={c}
                  style={{
                    position: 'absolute',
                    left: colOffsets[c] - rowHeaderWidth,
                    width,
                    height: columnHeaderHeight,
                    border: '1px solid var(--border-color)',
                    background,
                    color: 'var(--text-primary)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: '12px',
                    fontWeight: 'bold',
                    userSelect: 'none'
                  }}
                  onMouseEnter={() => setHoveredCol(c)}
                  onMouseLeave={() => setHoveredCol(null)}
                >
                  {getColumnLabel(c)}
                  <div
                    className="absolute right-0 top-0 w-1 h-full cursor-col-resize hover:bg-blue-500"
                    onMouseDown={(e) => handleColResizeStart(c, e)}
                    style={{ zIndex: 30 }}
                  />
                </div>
              );
            })}
          </div>
        </div>

        {renderSelectionOverlay()}

        {/* Rows */}
        {Array.from({ length: visibleRange.endRow - visibleRange.startRow + 1 }).map((_, rowIdx) => {
          const r = visibleRange.startRow + rowIdx;
          const rowHeight = getRowHeight(r);
          const isRowActive = activeCell?.row === r;
          const isRowHovered = hoveredRow === r;
          const rowHeaderBackground = isRowActive
            ? 'var(--sheet-header-active-bg)'
            : isRowHovered
              ? 'var(--sheet-header-hover-bg)'
              : 'var(--bg-light)';

          return (
            <div
              key={r}
              style={{
                position: 'absolute',
                top: rowOffsets[r],
                left: 0,
                width: totalWidth,
                height: rowHeight,
                display: 'flex'
              }}
            >
              {/* Row Header */}
              <div
                style={{
                  position: 'sticky',
                  left: 0,
                  width: rowHeaderWidth,
                  height: rowHeight,
                  border: '1px solid var(--border-color)',
                  background: rowHeaderBackground,
                  color: 'var(--text-primary)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: '12px',
                  fontWeight: 'bold',
                  userSelect: 'none',
                  zIndex: 10
                }}
                onMouseEnter={() => setHoveredRow(r)}
                onMouseLeave={() => setHoveredRow(null)}
              >
                {r + 1}
                <div
                  className="absolute bottom-0 left-0 w-full h-1 cursor-row-resize hover:bg-blue-500"
                  onMouseDown={(e) => handleRowResizeStart(r, e)}
                  style={{ zIndex: 30 }}
                />
              </div>

              {/* Cells - Render VISIBLE columns only */}
              {Array.from({ length: visibleRange.endCol - visibleRange.startCol + 1 }).map((_, colIdx) => {
                const c = visibleRange.startCol + colIdx;
                const width = getColWidth(c);
                const isEditing = editingCell?.row === r && editingCell?.col === c;
                const isActive = activeCell?.row === r && activeCell?.col === c;
                const defaultBackgroundColor = r % 2 === 0 ? 'var(--sheet-cell-bg)' : 'var(--sheet-cell-bg-alt)';

                return (
                  <div
                    key={`${r}-${c}`}
                    style={{
                      position: 'absolute',
                      left: colOffsets[c],
                      width,
                      height: rowHeight
                    }}
                  >
                    <Cell
                      row={r}
                      col={c}
                      data={data}
                      isActive={isActive}
                      isEditing={isEditing}
                      editValue={isEditing ? editingValue : ''}
                      width={width}
                      height={rowHeight}
                      defaultBackgroundColor={defaultBackgroundColor}
                      onClick={onCellClick}
                      onMouseDown={handleMouseDown}
                      onMouseOver={handleMouseOver}
                      onEditStart={onEditStart}
                      onEditChange={onEditChange}
                      onEditCommit={onEditCommit}
                      onEditCancel={onEditCancel}
                      onContextMenu={handleCellContextMenu}
                    />
                  </div>
                );
              })}
            </div>
          );
        })}
      </div>
    </div>
  );
};

export default Grid;
