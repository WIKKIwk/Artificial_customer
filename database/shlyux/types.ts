export type CellStyle = {
  bold?: boolean;
  italic?: boolean;
  underline?: boolean;
  textAlign?: 'left' | 'center' | 'right';
  color?: string;
  backgroundColor?: string;
  fontSize?: number;
  fontFamily?: string;
  numberFormat?: 'general' | 'number' | 'currency' | 'percent';
  decimalPlaces?: number;
  currencyCode?: string;
  verticalAlign?: 'top' | 'middle' | 'bottom';
  wrapMode?: 'overflow' | 'wrap' | 'clip';
  borders?: {
    top?: boolean;
    right?: boolean;
    bottom?: boolean;
    left?: boolean;
    color?: string;
    style?: 'solid' | 'dashed' | 'dotted';
  };
  rotation?: number; // degrees: 0, 90, -90, etc.
};

export interface CellData {
  value: string; // The raw input (e.g., "=SUM(A1:A5)" or "100")
  computed?: string | number; // The display value
  style?: CellStyle;
}

// Map key is "row,col" e.g., "0,0" for A1
export type GridData = Record<string, CellData>;

export interface SelectionRange {
  start: { row: number; col: number };
  end: { row: number; col: number };
}

export interface ClipboardData {
  cells: Array<{ row: number; col: number; data: CellData }>;
  isCut: boolean;
}

export interface FreezePosition {
  rows: number; // Number of rows to freeze from top
  cols: number; // Number of columns to freeze from left
}

export interface ContextMenuState {
  show: boolean;
  x: number;
  y: number;
  row: number;
  col: number;
}

export interface MergedCell {
  startRow: number;
  startCol: number;
  endRow: number;
  endCol: number;
}

export interface SheetState {
  data: GridData;
  activeCell: { row: number; col: number } | null;
  selection: SelectionRange | null;
  columnWidths: Record<number, number>;
  rowHeights: Record<number, number>;
  rowCount: number;
  freezePosition?: FreezePosition;
  mergedCells?: MergedCell[];
}
