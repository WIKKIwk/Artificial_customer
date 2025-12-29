import { GridData } from '../types';

export const NUM_ROWS = 100;
export const NUM_COLS = 52; // A-Z, AA-AZ (expanded to 52 columns)

export const getColumnLabel = (index: number): string => {
  let label = '';
  let i = index + 1; // Convert to 1-based
  while (i > 0) {
    const remainder = (i - 1) % 26;
    label = String.fromCharCode(remainder + 65) + label;
    i = Math.floor((i - 1) / 26);
  }
  return label;
};

export const getCellId = (row: number, col: number): string => `${row},${col}`;

export const parseCellId = (id: string): { row: number; col: number } => {
  const [row, col] = id.split(',').map(Number);
  return { row, col };
};

export const cellLabelToCoords = (label: string): { row: number; col: number } | null => {
  const match = label.match(/^([A-Z]+)([0-9]+)$/);
  if (!match) return null;

  const colStr = match[1];
  const rowStr = match[2];

  let col = 0;
  for (let i = 0; i < colStr.length; i++) {
    col = col * 26 + (colStr.charCodeAt(i) - 65 + 1);
  }

  return {
    col: col - 1,
    row: parseInt(rowStr, 10) - 1
  };
};

const getCellValue = (row: number, col: number, data: GridData): number => {
  const cell = data[getCellId(row, col)];
  if (!cell) return 0;
  const val = parseFloat(cell.computed?.toString() || cell.value);
  return isNaN(val) ? 0 : val;
};

const getCellRawValue = (row: number, col: number, data: GridData): string => {
  const cell = data[getCellId(row, col)];
  if (!cell) return '';
  return cell.computed?.toString() || cell.value;
};

const isNumeric = (val: any): boolean => {
  return !isNaN(parseFloat(val)) && isFinite(val);
};

// Basic formula parser with 100+ Google Sheets functions
export const evaluateFormula = (formula: string, data: GridData): string | number => {
  if (!formula.startsWith('=')) return formula;

  const expression = formula.substring(1).toUpperCase();

  try {
    // Helper to expand range to array of values
    const expandRange = (start: string, end: string): number[] => {
      const startCoords = cellLabelToCoords(start);
      const endCoords = cellLabelToCoords(end);
      if (!startCoords || !endCoords) return [];

      const values: number[] = [];
      const minRow = Math.min(startCoords.row, endCoords.row);
      const maxRow = Math.max(startCoords.row, endCoords.row);
      const minCol = Math.min(startCoords.col, endCoords.col);
      const maxCol = Math.max(startCoords.col, endCoords.col);

      for (let r = minRow; r <= maxRow; r++) {
        for (let c = minCol; c <= maxCol; c++) {
          values.push(getCellValue(r, c, data));
        }
      }
      return values;
    };

    // Helper to expand range to array of raw string values
    const expandRangeRaw = (start: string, end: string): string[] => {
      const startCoords = cellLabelToCoords(start);
      const endCoords = cellLabelToCoords(end);
      if (!startCoords || !endCoords) return [];

      const values: string[] = [];
      const minRow = Math.min(startCoords.row, endCoords.row);
      const maxRow = Math.max(startCoords.row, endCoords.row);
      const minCol = Math.min(startCoords.col, endCoords.col);
      const maxCol = Math.max(startCoords.col, endCoords.col);

      for (let r = minRow; r <= maxRow; r++) {
        for (let c = minCol; c <= maxCol; c++) {
          values.push(getCellRawValue(r, c, data));
        }
      }
      return values;
    };

    const stripWrappingQuotes = (value: string): string => {
      const trimmed = value.trim();
      if ((trimmed.startsWith('"') && trimmed.endsWith('"')) || (trimmed.startsWith("'") && trimmed.endsWith("'"))) {
        return trimmed.substring(1, trimmed.length - 1);
      }
      return trimmed;
    };

    const splitArgs = (args: string): string[] => {
      const parts: string[] = [];
      let current = '';
      let depth = 0;
      let inQuote: string | null = null;

      for (let i = 0; i < args.length; i++) {
        const char = args[i];
        if ((char === '"' || char === "'") && args[i - 1] !== '\\') {
          if (inQuote === char) {
            inQuote = null;
          } else if (!inQuote) {
            inQuote = char;
          }
        }
        if (!inQuote) {
          if (char === '(') depth++;
          if (char === ')') depth = Math.max(0, depth - 1);
          if (char === ',' && depth === 0) {
            parts.push(current.trim());
            current = '';
            continue;
          }
        }
        current += char;
      }
      if (current.trim()) {
        parts.push(current.trim());
      }
      return parts;
    };

    const getNumericValueFromArg = (arg: string): number => {
      const trimmed = arg.trim();
      if (trimmed === '') return 0;
      const coords = cellLabelToCoords(trimmed);
      if (coords) return getCellValue(coords.row, coords.col, data);
      if ((trimmed.startsWith('"') && trimmed.endsWith('"')) || (trimmed.startsWith("'") && trimmed.endsWith("'"))) {
        const parsedNumber = parseFloat(trimmed.substring(1, trimmed.length - 1));
        return isNaN(parsedNumber) ? 0 : parsedNumber;
      }
      const num = parseFloat(trimmed);
      return isNaN(num) ? 0 : num;
    };

    const getRawValueFromArg = (arg: string): string => {
      const trimmed = arg.trim();
      const coords = cellLabelToCoords(trimmed);
      if (coords) return getCellRawValue(coords.row, coords.col, data);
      return stripWrappingQuotes(trimmed);
    };

    const getRangeNumbersGeneral = (rangeArg: string): number[] => {
      const trimmed = rangeArg.trim();
      if (trimmed.includes(':')) {
        const [start, end] = trimmed.split(':');
        return expandRange(start.trim(), end.trim());
      }
      const coords = cellLabelToCoords(trimmed);
      if (coords) return [getCellValue(coords.row, coords.col, data)];
      const num = parseFloat(stripWrappingQuotes(trimmed));
      return isNaN(num) ? [] : [num];
    };

    const getRangeRawGeneral = (rangeArg: string): string[] => {
      const trimmed = rangeArg.trim();
      if (trimmed.includes(':')) {
        const [start, end] = trimmed.split(':');
        return expandRangeRaw(start.trim(), end.trim());
      }
      const coords = cellLabelToCoords(trimmed);
      if (coords) return [getCellRawValue(coords.row, coords.col, data)];
      return [stripWrappingQuotes(trimmed)];
    };

    const matchesCriteria = (candidate: string | number, criteria: string): boolean => {
      const cleanCriteria = stripWrappingQuotes(criteria);
      if (cleanCriteria === '') return false;
      let operator = '=';
      let value = cleanCriteria;
      const match = cleanCriteria.match(/^([><=!]+)(.+)$/);
      if (match) {
        operator = match[1];
        value = match[2];
      }
      const candidateNum = typeof candidate === 'number' ? candidate : parseFloat(candidate);
      const valueNum = parseFloat(value);
      const bothNumeric = !isNaN(candidateNum) && !isNaN(valueNum);
      const candidateStr = typeof candidate === 'string' ? candidate : candidate.toString();
      const valueStr = value;

      const compare = () => {
        if (bothNumeric) {
          switch (operator) {
            case '>': return candidateNum > valueNum;
            case '<': return candidateNum < valueNum;
            case '>=': return candidateNum >= valueNum;
            case '<=': return candidateNum <= valueNum;
            case '!=':
            case '<>': return candidateNum !== valueNum;
            default: return candidateNum === valueNum;
          }
        }
        switch (operator) {
          case '!=':
          case '<>': return candidateStr !== valueStr;
          case '=': return candidateStr === valueStr;
          default:
            return candidateStr === valueStr;
        }
      };

      return compare();
    };

    const evaluateCondition = (condition: string): boolean => {
      const trimmed = condition.trim();
      const condMatch = trimmed.match(/^(.*?)(>=|<=|<>|!=|=|>|<)(.*)$/);
      if (condMatch) {
        const left = condMatch[1];
        const operator = condMatch[2];
        const right = condMatch[3];
        const leftRaw = getRawValueFromArg(left);
        const rightRaw = getRawValueFromArg(right);
        const leftNum = parseFloat(leftRaw);
        const rightNum = parseFloat(rightRaw);
        const leftIsNumeric = !isNaN(leftNum);
        const rightIsNumeric = !isNaN(rightNum);

        if (leftIsNumeric && rightIsNumeric) {
          switch (operator) {
            case '>': return leftNum > rightNum;
            case '<': return leftNum < rightNum;
            case '>=': return leftNum >= rightNum;
            case '<=': return leftNum <= rightNum;
            case '!=':
            case '<>': return leftNum !== rightNum;
            default: return leftNum === rightNum;
          }
        }

        const leftStr = leftRaw.toString();
        const rightStr = rightRaw.toString();
        switch (operator) {
          case '!=':
          case '<>': return leftStr !== rightStr;
          case '=': return leftStr === rightStr;
          case '>': return leftStr > rightStr;
          case '<': return leftStr < rightStr;
          case '>=': return leftStr >= rightStr;
          case '<=': return leftStr <= rightStr;
          default: return leftStr === rightStr;
        }
      }

      const value = getRawValueFromArg(trimmed);
      if (value.toUpperCase() === 'TRUE') return true;
      if (value.toUpperCase() === 'FALSE') return false;
      const numeric = parseFloat(value);
      if (!isNaN(numeric)) {
        return numeric !== 0;
      }
      return value !== '';
    };

    // Replace functions with calculated values
    let parsed = expression;

    // ============ BASIC MATH FUNCTIONS ============

    // SUM(A1:B2) or SUM(A1,A2,A3)
    parsed = parsed.replace(/SUM\(([^)]+)\)/g, (_, args) => {
      const vals: number[] = [];
      args.split(',').forEach((arg: string) => {
        if (arg.includes(':')) {
          const [start, end] = arg.split(':');
          vals.push(...expandRange(start.trim(), end.trim()));
        } else {
          const coords = cellLabelToCoords(arg.trim());
          if (coords) vals.push(getCellValue(coords.row, coords.col, data));
        }
      });
      return vals.reduce((a, b) => a + b, 0).toString();
    });

    // AVERAGE(A1:B2) or AVERAGE(A1,A2,A3)
    parsed = parsed.replace(/AVERAGE\(([^)]+)\)/g, (_, args) => {
      const vals: number[] = [];
      args.split(',').forEach((arg: string) => {
        if (arg.includes(':')) {
          const [start, end] = arg.split(':');
          vals.push(...expandRange(start.trim(), end.trim()));
        } else {
          const coords = cellLabelToCoords(arg.trim());
          if (coords) vals.push(getCellValue(coords.row, coords.col, data));
        }
      });
      return vals.length ? (vals.reduce((a, b) => a + b, 0) / vals.length).toString() : '0';
    });

    // MAX(A1:B2)
    parsed = parsed.replace(/MAX\(([^)]+)\)/g, (_, args) => {
      const vals: number[] = [];
      args.split(',').forEach((arg: string) => {
        if (arg.includes(':')) {
          const [start, end] = arg.split(':');
          vals.push(...expandRange(start.trim(), end.trim()));
        } else {
          const coords = cellLabelToCoords(arg.trim());
          if (coords) vals.push(getCellValue(coords.row, coords.col, data));
        }
      });
      return vals.length ? Math.max(...vals).toString() : '0';
    });

    // MIN(A1:B2)
    parsed = parsed.replace(/MIN\(([^)]+)\)/g, (_, args) => {
      const vals: number[] = [];
      args.split(',').forEach((arg: string) => {
        if (arg.includes(':')) {
          const [start, end] = arg.split(':');
          vals.push(...expandRange(start.trim(), end.trim()));
        } else {
          const coords = cellLabelToCoords(arg.trim());
          if (coords) vals.push(getCellValue(coords.row, coords.col, data));
        }
      });
      return vals.length ? Math.min(...vals).toString() : '0';
    });

    // PRODUCT(A1:A5) - Ko'paytirish
    parsed = parsed.replace(/PRODUCT\(([^)]+)\)/g, (_, args) => {
      const vals: number[] = [];
      args.split(',').forEach((arg: string) => {
        if (arg.includes(':')) {
          const [start, end] = arg.split(':');
          vals.push(...expandRange(start.trim(), end.trim()));
        } else {
          const coords = cellLabelToCoords(arg.trim());
          if (coords) vals.push(getCellValue(coords.row, coords.col, data));
        }
      });
      return vals.length ? vals.reduce((a, b) => a * b, 1).toString() : '1';
    });

    // COUNT(A1:A10) - Raqamlarni sanash
    parsed = parsed.replace(/COUNT\(([^)]+)\)/g, (_, args) => {
      let count = 0;
      args.split(',').forEach((arg: string) => {
        if (arg.includes(':')) {
          const [start, end] = arg.split(':');
          expandRangeRaw(start.trim(), end.trim()).forEach(val => {
            if (isNumeric(val)) count++;
          });
        } else {
          const coords = cellLabelToCoords(arg.trim());
          if (coords && isNumeric(getCellRawValue(coords.row, coords.col, data))) count++;
        }
      });
      return count.toString();
    });

    // COUNTA(A1:A10) - Bo'sh bo'lmagan katakchalarni sanash
    parsed = parsed.replace(/COUNTA\(([^)]+)\)/g, (_, args) => {
      let count = 0;
      args.split(',').forEach((arg: string) => {
        if (arg.includes(':')) {
          const [start, end] = arg.split(':');
          expandRangeRaw(start.trim(), end.trim()).forEach(val => {
            if (val !== '') count++;
          });
        } else {
          const coords = cellLabelToCoords(arg.trim());
          if (coords && getCellRawValue(coords.row, coords.col, data) !== '') count++;
        }
      });
      return count.toString();
    });

    // COUNTBLANK(A1:A10) - Bo'sh katakchalarni sanash
    parsed = parsed.replace(/COUNTBLANK\(([^)]+)\)/g, (_, args) => {
      let count = 0;
      args.split(',').forEach((arg: string) => {
        if (arg.includes(':')) {
          const [start, end] = arg.split(':');
          expandRangeRaw(start.trim(), end.trim()).forEach(val => {
            if (val === '') count++;
          });
        }
      });
      return count.toString();
    });

    // ABS(A1) - Mutlaq qiymat
    parsed = parsed.replace(/ABS\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(arg);
      return Math.abs(val).toString();
    });

    // ROUND(A1,2) - Yaxlitlash
    parsed = parsed.replace(/ROUND\(([^,]+),([^)]+)\)/g, (_, numArg, decimals) => {
      const coords = cellLabelToCoords(numArg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(numArg);
      const dec = parseInt(decimals.trim(), 10);
      return val.toFixed(dec);
    });

    // ROUNDUP(A1,2) - Yuqoriga yaxlitlash
    parsed = parsed.replace(/ROUNDUP\(([^,]+),([^)]+)\)/g, (_, numArg, decimals) => {
      const coords = cellLabelToCoords(numArg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(numArg);
      const dec = parseInt(decimals.trim(), 10);
      const multiplier = Math.pow(10, dec);
      return (Math.ceil(val * multiplier) / multiplier).toString();
    });

    // ROUNDDOWN(A1,2) - Pastga yaxlitlash
    parsed = parsed.replace(/ROUNDDOWN\(([^,]+),([^)]+)\)/g, (_, numArg, decimals) => {
      const coords = cellLabelToCoords(numArg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(numArg);
      const dec = parseInt(decimals.trim(), 10);
      const multiplier = Math.pow(10, dec);
      return (Math.floor(val * multiplier) / multiplier).toString();
    });

    // CEILING(A1) - Yuqoridagi butun songa yaxlitlash
    parsed = parsed.replace(/CEILING\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(arg);
      return Math.ceil(val).toString();
    });

    // FLOOR(A1) - Pastdagi butun songa yaxlitlash
    parsed = parsed.replace(/FLOOR\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(arg);
      return Math.floor(val).toString();
    });

    // SQRT(A1) - Kvadrat ildiz
    parsed = parsed.replace(/SQRT\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(arg);
      return Math.sqrt(val).toString();
    });

    // POWER(A1,2) - Darajaga ko'tarish
    parsed = parsed.replace(/POWER\(([^,]+),([^)]+)\)/g, (_, base, exp) => {
      const coordsBase = cellLabelToCoords(base.trim());
      const coordsExp = cellLabelToCoords(exp.trim());
      const baseVal = coordsBase ? getCellValue(coordsBase.row, coordsBase.col, data) : parseFloat(base);
      const expVal = coordsExp ? getCellValue(coordsExp.row, coordsExp.col, data) : parseFloat(exp);
      return Math.pow(baseVal, expVal).toString();
    });

    // MOD(A1,B1) - Qoldiq
    parsed = parsed.replace(/MOD\(([^,]+),([^)]+)\)/g, (_, dividend, divisor) => {
      const coordsDiv = cellLabelToCoords(dividend.trim());
      const coordsDivisor = cellLabelToCoords(divisor.trim());
      const divVal = coordsDiv ? getCellValue(coordsDiv.row, coordsDiv.col, data) : parseFloat(dividend);
      const divisorVal = coordsDivisor ? getCellValue(coordsDivisor.row, coordsDivisor.col, data) : parseFloat(divisor);
      return (divVal % divisorVal).toString();
    });

    // QUOTIENT(A1,B1) - Butun bo'linma
    parsed = parsed.replace(/QUOTIENT\(([^,]+),([^)]+)\)/g, (_, dividend, divisor) => {
      const coordsDiv = cellLabelToCoords(dividend.trim());
      const coordsDivisor = cellLabelToCoords(divisor.trim());
      const divVal = coordsDiv ? getCellValue(coordsDiv.row, coordsDiv.col, data) : parseFloat(dividend);
      const divisorVal = coordsDivisor ? getCellValue(coordsDivisor.row, coordsDivisor.col, data) : parseFloat(divisor);
      return Math.floor(divVal / divisorVal).toString();
    });

    // SIGN(A1) - Sonning ishorasi (-1, 0, 1)
    parsed = parsed.replace(/SIGN\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(arg);
      return Math.sign(val).toString();
    });

    // ============ STATISTICAL FUNCTIONS ============

    // MEDIAN(A1:A10) - Mediana
    parsed = parsed.replace(/MEDIAN\(([^)]+)\)/g, (_, args) => {
      const vals: number[] = [];
      args.split(',').forEach((arg: string) => {
        if (arg.includes(':')) {
          const [start, end] = arg.split(':');
          vals.push(...expandRange(start.trim(), end.trim()));
        } else {
          const coords = cellLabelToCoords(arg.trim());
          if (coords) vals.push(getCellValue(coords.row, coords.col, data));
        }
      });
      if (vals.length === 0) return '0';
      vals.sort((a, b) => a - b);
      const mid = Math.floor(vals.length / 2);
      return vals.length % 2 !== 0 ? vals[mid].toString() : ((vals[mid - 1] + vals[mid]) / 2).toString();
    });

    // MODE(A1:A10) - Eng ko'p uchraydigan qiymat
    parsed = parsed.replace(/MODE\(([^)]+)\)/g, (_, args) => {
      const vals: number[] = [];
      args.split(',').forEach((arg: string) => {
        if (arg.includes(':')) {
          const [start, end] = arg.split(':');
          vals.push(...expandRange(start.trim(), end.trim()));
        }
      });
      if (vals.length === 0) return '#N/A';
      const freq: Record<number, number> = {};
      vals.forEach(v => freq[v] = (freq[v] || 0) + 1);
      let maxFreq = 0;
      let mode = vals[0];
      Object.entries(freq).forEach(([val, count]) => {
        if (count > maxFreq) {
          maxFreq = count;
          mode = parseFloat(val);
        }
      });
      return mode.toString();
    });

    // STDEV(A1:A10) - Standart og'ish
    parsed = parsed.replace(/STDEV\(([^)]+)\)/g, (_, args) => {
      const vals: number[] = [];
      args.split(',').forEach((arg: string) => {
        if (arg.includes(':')) {
          const [start, end] = arg.split(':');
          vals.push(...expandRange(start.trim(), end.trim()));
        }
      });
      if (vals.length < 2) return '#DIV/0!';
      const avg = vals.reduce((a, b) => a + b, 0) / vals.length;
      const variance = vals.reduce((sum, val) => sum + Math.pow(val - avg, 2), 0) / (vals.length - 1);
      return Math.sqrt(variance).toString();
    });

    // VAR(A1:A10) - Dispersiya
    parsed = parsed.replace(/VAR\(([^)]+)\)/g, (_, args) => {
      const vals: number[] = [];
      args.split(',').forEach((arg: string) => {
        if (arg.includes(':')) {
          const [start, end] = arg.split(':');
          vals.push(...expandRange(start.trim(), end.trim()));
        }
      });
      if (vals.length < 2) return '#DIV/0!';
      const avg = vals.reduce((a, b) => a + b, 0) / vals.length;
      const variance = vals.reduce((sum, val) => sum + Math.pow(val - avg, 2), 0) / (vals.length - 1);
      return variance.toString();
    });

    // SUMIF(A1:A10,">5") - Shartli yig'indi
    parsed = parsed.replace(/SUMIF\(([^,]+),([^)]+)\)/g, (_, rangeArg, criteria) => {
      const [start, end] = rangeArg.trim().split(':');
      const vals = expandRange(start, end);
      const criteriaStr = criteria.trim().replace(/["']/g, '');

      let operator = '=';
      let value = criteriaStr;

      const match = criteriaStr.match(/^([><=!]+)(.+)$/);
      if (match) {
        operator = match[1];
        value = match[2];
      }

      const numValue = parseFloat(value);
      let sum = 0;

      vals.forEach(v => {
        let condition = false;
        switch (operator) {
          case '>': condition = v > numValue; break;
          case '<': condition = v < numValue; break;
          case '>=': condition = v >= numValue; break;
          case '<=': condition = v <= numValue; break;
          case '=': condition = v === numValue; break;
          case '!=': condition = v !== numValue; break;
          default: condition = v === numValue;
        }
        if (condition) sum += v;
      });

      return sum.toString();
    });

    // COUNTIF(A1:A10,">5") - Shartli sanash
    parsed = parsed.replace(/COUNTIF\(([^,]+),([^)]+)\)/g, (_, rangeArg, criteria) => {
      const [start, end] = rangeArg.trim().split(':');
      const vals = expandRange(start, end);
      const criteriaStr = criteria.trim().replace(/["']/g, '');

      let operator = '=';
      let value = criteriaStr;

      const match = criteriaStr.match(/^([><=!]+)(.+)$/);
      if (match) {
        operator = match[1];
        value = match[2];
      }

      const numValue = parseFloat(value);
      let count = 0;

      vals.forEach(v => {
        let condition = false;
        switch (operator) {
          case '>': condition = v > numValue; break;
          case '<': condition = v < numValue; break;
          case '>=': condition = v >= numValue; break;
          case '<=': condition = v <= numValue; break;
          case '=': condition = v === numValue; break;
          case '!=': condition = v !== numValue; break;
          default: condition = v === numValue;
        }
        if (condition) count++;
      });

      return count.toString();
    });

    // AVERAGEIF(A1:A10,">5") - Shartli o'rtacha
    parsed = parsed.replace(/AVERAGEIF\(([^,]+),([^)]+)\)/g, (_, rangeArg, criteria) => {
      const [start, end] = rangeArg.trim().split(':');
      const vals = expandRange(start, end);
      const criteriaStr = criteria.trim().replace(/["']/g, '');

      let operator = '=';
      let value = criteriaStr;

      const match = criteriaStr.match(/^([><=!]+)(.+)$/);
      if (match) {
        operator = match[1];
        value = match[2];
      }

      const numValue = parseFloat(value);
      let sum = 0;
      let count = 0;

      vals.forEach(v => {
        let condition = false;
        switch (operator) {
          case '>': condition = v > numValue; break;
          case '<': condition = v < numValue; break;
          case '>=': condition = v >= numValue; break;
          case '<=': condition = v <= numValue; break;
          case '=': condition = v === numValue; break;
          case '!=': condition = v !== numValue; break;
          default: condition = v === numValue;
        }
        if (condition) {
          sum += v;
          count++;
        }
      });

      return count > 0 ? (sum / count).toString() : '0';
    });

    // SUMIFS(sum_range, criteria_range1, criteria1, ...)
    parsed = parsed.replace(/SUMIFS\(([^)]+)\)/g, (_, args) => {
      const parts = splitArgs(args);
      if (parts.length < 3 || parts.length % 2 === 0) return '#ERROR';
      const sumRange = getRangeNumbersGeneral(parts[0]);
      const criteriaPairs = [] as { range: string[]; criteria: string }[];
      for (let i = 1; i < parts.length; i += 2) {
        criteriaPairs.push({ range: getRangeRawGeneral(parts[i]), criteria: parts[i + 1] });
      }
      const minLength = Math.min(sumRange.length, ...criteriaPairs.map(p => p.range.length));
      if (!isFinite(minLength) || minLength <= 0) return '0';
      let total = 0;
      for (let idx = 0; idx < minLength; idx++) {
        let matchesAll = true;
        for (const pair of criteriaPairs) {
          if (!matchesCriteria(pair.range[idx] ?? '', pair.criteria)) {
            matchesAll = false;
            break;
          }
        }
        if (matchesAll) {
          total += sumRange[idx];
        }
      }
      return total.toString();
    });

    // COUNTIFS(range1, criteria1, ...)
    parsed = parsed.replace(/COUNTIFS\(([^)]+)\)/g, (_, args) => {
      const parts = splitArgs(args);
      if (parts.length < 2 || parts.length % 2 !== 0) return '#ERROR';
      const criteriaPairs = [] as { range: string[]; criteria: string }[];
      for (let i = 0; i < parts.length; i += 2) {
        criteriaPairs.push({ range: getRangeRawGeneral(parts[i]), criteria: parts[i + 1] });
      }
      const minLength = Math.min(...criteriaPairs.map(p => p.range.length));
      if (!isFinite(minLength) || minLength <= 0) return '0';
      let count = 0;
      for (let idx = 0; idx < minLength; idx++) {
        let matchesAll = true;
        for (const pair of criteriaPairs) {
          if (!matchesCriteria(pair.range[idx] ?? '', pair.criteria)) {
            matchesAll = false;
            break;
          }
        }
        if (matchesAll) count++;
      }
      return count.toString();
    });

    // AVERAGEIFS(range, criteria_range1, criteria1, ...)
    parsed = parsed.replace(/AVERAGEIFS\(([^)]+)\)/g, (_, args) => {
      const parts = splitArgs(args);
      if (parts.length < 3 || parts.length % 2 === 0) return '#ERROR';
      const valueRange = getRangeNumbersGeneral(parts[0]);
      const criteriaPairs = [] as { range: string[]; criteria: string }[];
      for (let i = 1; i < parts.length; i += 2) {
        criteriaPairs.push({ range: getRangeRawGeneral(parts[i]), criteria: parts[i + 1] });
      }
      const minLength = Math.min(valueRange.length, ...criteriaPairs.map(p => p.range.length));
      if (!isFinite(minLength) || minLength <= 0) return '0';
      let sum = 0;
      let count = 0;
      for (let idx = 0; idx < minLength; idx++) {
        let matchesAll = true;
        for (const pair of criteriaPairs) {
          if (!matchesCriteria(pair.range[idx] ?? '', pair.criteria)) {
            matchesAll = false;
            break;
          }
        }
        if (matchesAll) {
          sum += valueRange[idx];
          count++;
        }
      }
      return count ? (sum / count).toString() : '0';
    });

    // SUMPRODUCT(range1, range2, ...)
    parsed = parsed.replace(/SUMPRODUCT\(([^)]+)\)/g, (_, args) => {
      const parts = splitArgs(args);
      if (!parts.length) return '0';
      const ranges = parts.map(part => getRangeNumbersGeneral(part));
      const length = Math.min(...ranges.map(arr => arr.length));
      if (!isFinite(length) || length === 0) return '0';
      let total = 0;
      for (let i = 0; i < length; i++) {
        let product = 1;
        ranges.forEach(arr => {
          product *= arr[i];
        });
        total += product;
      }
      return total.toString();
    });

    // SUBTOTAL(function_code, range)
    parsed = parsed.replace(/SUBTOTAL\(([^,]+),([^)]+)\)/g, (_, funcCode, rangeArg) => {
      const code = parseInt(funcCode.trim(), 10);
      const values = getRangeNumbersGeneral(rangeArg);
      if (!values.length) return '0';
      switch (code) {
        case 1: // AVERAGE
          return (values.reduce((a, b) => a + b, 0) / values.length).toString();
        case 2: // COUNT
          return values.length.toString();
        case 3: // COUNTA
          return values.filter(v => v !== 0).length.toString();
        case 9: // SUM
          return values.reduce((a, b) => a + b, 0).toString();
        default:
          return '#VALUE!';
      }
    });

    // LARGE(A1:A10,2)
    parsed = parsed.replace(/LARGE\(([^,]+),([^)]+)\)/g, (_, rangeArg, kArg) => {
      const values = getRangeNumbersGeneral(rangeArg).sort((a, b) => b - a);
      const k = Math.max(1, Math.floor(getNumericValueFromArg(kArg)));
      return values[k - 1] !== undefined ? values[k - 1].toString() : '#NUM!';
    });

    // SMALL(A1:A10,2)
    parsed = parsed.replace(/SMALL\(([^,]+),([^)]+)\)/g, (_, rangeArg, kArg) => {
      const values = getRangeNumbersGeneral(rangeArg).sort((a, b) => a - b);
      const k = Math.max(1, Math.floor(getNumericValueFromArg(kArg)));
      return values[k - 1] !== undefined ? values[k - 1].toString() : '#NUM!';
    });

    // PERCENTILE(range, k)
    parsed = parsed.replace(/PERCENTILE\(([^,]+),([^)]+)\)/g, (_, rangeArg, kArg) => {
      const values = getRangeNumbersGeneral(rangeArg).sort((a, b) => a - b);
      const k = parseFloat(kArg.trim());
      if (!values.length) return '#N/A';
      const pos = (values.length - 1) * k;
      const lower = Math.floor(pos);
      const upper = Math.ceil(pos);
      if (values[lower] === undefined || values[upper] === undefined) return '#N/A';
      const result = values[lower] + (pos - lower) * (values[upper] - values[lower]);
      return result.toString();
    });

    // PERCENTRANK(range, value)
    parsed = parsed.replace(/PERCENTRANK\(([^,]+),([^)]+)\)/g, (_, rangeArg, valueArg) => {
      const values = getRangeNumbersGeneral(rangeArg).sort((a, b) => a - b);
      if (values.length < 2) return '0';
      const value = getNumericValueFromArg(valueArg);
      let lowerCount = 0;
      values.forEach(v => { if (v < value) lowerCount++; });
      const rank = lowerCount + 1;
      return ((rank - 1) / (values.length - 1)).toString();
    });

    // RANK(number, range, [order])
    parsed = parsed.replace(/RANK\(([^,]+),([^,]+)(?:,([^)]+))?\)/g, (_, numberArg, rangeArg, orderArg) => {
      const number = getNumericValueFromArg(numberArg);
      const values = getRangeNumbersGeneral(rangeArg).sort((a, b) => a - b);
      if (!values.length) return '#N/A';
      const order = orderArg ? getNumericValueFromArg(orderArg) : 0;
      if (order === 0) {
        values.reverse();
      }
      const rank = values.findIndex(v => v === number);
      return rank >= 0 ? (rank + 1).toString() : '#N/A';
    });

    // INT(A1)
    parsed = parsed.replace(/INT\(([^)]+)\)/g, (_, arg) => {
      return Math.floor(getNumericValueFromArg(arg)).toString();
    });

    // TRUNC(A1, [digits])
    parsed = parsed.replace(/TRUNC\(([^,]+)(?:,([^)]+))?\)/g, (_, arg, digits) => {
      const value = getNumericValueFromArg(arg);
      if (!digits) return (value < 0 ? Math.ceil(value) : Math.floor(value)).toString();
      const places = Math.max(0, parseInt(digits.trim(), 10));
      const factor = Math.pow(10, places);
      return (value < 0
        ? Math.ceil(value * factor) / factor
        : Math.floor(value * factor) / factor).toString();
    });

    // MROUND(number, multiple)
    parsed = parsed.replace(/MROUND\(([^,]+),([^)]+)\)/g, (_, numberArg, multipleArg) => {
      const num = getNumericValueFromArg(numberArg);
      const multiple = getNumericValueFromArg(multipleArg);
      if (multiple === 0) return '0';
      return (Math.round(num / multiple) * multiple).toString();
    });

    // ============ LOGICAL FUNCTIONS ============

    // IF(A1>5,"Yes","No") - Shart operatori
    parsed = parsed.replace(/IF\(([^,]+),([^,]+),([^)]+)\)/g, (_, condition, trueVal, falseVal) => {
      return evaluateCondition(condition)
        ? stripWrappingQuotes(trueVal)
        : stripWrappingQuotes(falseVal);
    });

    // AND(condition1, condition2, ...)
    parsed = parsed.replace(/AND\(([^)]+)\)/g, (_, args) => {
      const parts = splitArgs(args);
      const result = parts.every(part => evaluateCondition(part));
      return result ? 'TRUE' : 'FALSE';
    });

    // OR(condition1, condition2, ...)
    parsed = parsed.replace(/OR\(([^)]+)\)/g, (_, args) => {
      const parts = splitArgs(args);
      const result = parts.some(part => evaluateCondition(part));
      return result ? 'TRUE' : 'FALSE';
    });

    // NOT(condition)
    parsed = parsed.replace(/NOT\(([^)]+)\)/g, (_, arg) => {
      return evaluateCondition(arg) ? 'FALSE' : 'TRUE';
    });

    // XOR(condition1, condition2, ...)
    parsed = parsed.replace(/XOR\(([^)]+)\)/g, (_, args) => {
      const parts = splitArgs(args);
      let trueCount = 0;
      parts.forEach(part => {
        if (evaluateCondition(part)) trueCount++;
      });
      return trueCount % 2 === 1 ? 'TRUE' : 'FALSE';
    });

    // IFS(condition1, value1, ...)
    parsed = parsed.replace(/IFS\(([^)]+)\)/g, (_, args) => {
      const parts = splitArgs(args);
      for (let i = 0; i < parts.length - 1; i += 2) {
        if (evaluateCondition(parts[i])) {
          return stripWrappingQuotes(parts[i + 1]);
        }
      }
      return '#N/A';
    });

    // SWITCH(expression, value1, result1, ..., [default])
    parsed = parsed.replace(/SWITCH\(([^)]+)\)/g, (_, args) => {
      const parts = splitArgs(args);
      if (parts.length < 3) return '#VALUE!';
      const expression = getRawValueFromArg(parts[0]);
      for (let i = 1; i < parts.length - 1; i += 2) {
        if (getRawValueFromArg(parts[i]) === expression) {
          return stripWrappingQuotes(parts[i + 1]);
        }
      }
      return parts.length % 2 === 0 ? stripWrappingQuotes(parts[parts.length - 1]) : '#N/A';
    });

    // IFERROR(A1,"Error") - Xato bo'lsa boshqa qiymat qaytarish
    parsed = parsed.replace(/IFERROR\(([^,]+),([^)]+)\)/g, (_, value, errorVal) => {
      const coords = cellLabelToCoords(value.trim());
      const val = coords ? getCellRawValue(coords.row, coords.col, data) : value.trim();
      return val.startsWith('#') ? errorVal.trim().replace(/["']/g, '') : val;
    });

    // CHOOSE(index, value1, value2, ...)
    parsed = parsed.replace(/CHOOSE\(([^)]+)\)/g, (_, args) => {
      const parts = splitArgs(args);
      if (parts.length < 2) return '#VALUE!';
      const index = Math.floor(getNumericValueFromArg(parts[0]));
      if (index < 1 || index >= parts.length) return '#VALUE!';
      return stripWrappingQuotes(parts[index]);
    });

    // ============ TEXT FUNCTIONS ============

    // LEN(A1) - Matn uzunligi
    parsed = parsed.replace(/LEN\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellRawValue(coords.row, coords.col, data) : arg.trim();
      return val.length.toString();
    });

    // UPPER(A1) - Katta harflarga o'tkazish
    parsed = parsed.replace(/UPPER\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellRawValue(coords.row, coords.col, data) : arg.trim().replace(/["']/g, '');
      return `"${val.toUpperCase()}"`;
    });

    // LOWER(A1) - Kichik harflarga o'tkazish
    parsed = parsed.replace(/LOWER\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellRawValue(coords.row, coords.col, data) : arg.trim().replace(/["']/g, '');
      return `"${val.toLowerCase()}"`;
    });

    // PROPER(A1) - Har bir so'zning birinchi harfini katta qilish
    parsed = parsed.replace(/PROPER\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellRawValue(coords.row, coords.col, data) : arg.trim().replace(/["']/g, '');
      const proper = val.toLowerCase().replace(/\b\w/g, (char) => char.toUpperCase());
      return `"${proper}"`;
    });

    // TRIM(A1) - Bo'sh joylarni olib tashlash
    parsed = parsed.replace(/TRIM\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellRawValue(coords.row, coords.col, data) : arg.trim().replace(/["']/g, '');
      return `"${val.trim()}"`;
    });

    // CONCATENATE(A1,B1) yoki CONCAT(A1,B1) - Matnlarni birlashtirish
    parsed = parsed.replace(/(?:CONCATENATE|CONCAT)\(([^)]+)\)/g, (_, args) => {
      const parts = args.split(',').map((arg: string) => {
        const trimmed = arg.trim();
        const coords = cellLabelToCoords(trimmed);
        return coords ? getCellRawValue(coords.row, coords.col, data) : trimmed.replace(/["']/g, '');
      });
      return `"${parts.join('')}"`;
    });

    // LEFT(A1,3) - Chapdan belgilar olish
    parsed = parsed.replace(/LEFT\(([^,]+),([^)]+)\)/g, (_, text, length) => {
      const coords = cellLabelToCoords(text.trim());
      const val = coords ? getCellRawValue(coords.row, coords.col, data) : text.trim().replace(/["']/g, '');
      const len = parseInt(length.trim(), 10);
      return `"${val.substring(0, len)}"`;
    });

    // RIGHT(A1,3) - O'ngdan belgilar olish
    parsed = parsed.replace(/RIGHT\(([^,]+),([^)]+)\)/g, (_, text, length) => {
      const coords = cellLabelToCoords(text.trim());
      const val = coords ? getCellRawValue(coords.row, coords.col, data) : text.trim().replace(/["']/g, '');
      const len = parseInt(length.trim(), 10);
      return `"${val.substring(val.length - len)}"`;
    });

    // MID(A1,2,3) - O'rtadan belgilar olish
    parsed = parsed.replace(/MID\(([^,]+),([^,]+),([^)]+)\)/g, (_, text, start, length) => {
      const coords = cellLabelToCoords(text.trim());
      const val = coords ? getCellRawValue(coords.row, coords.col, data) : text.trim().replace(/["']/g, '');
      const startPos = parseInt(start.trim(), 10) - 1; // 1-based to 0-based
      const len = parseInt(length.trim(), 10);
      return `"${val.substring(startPos, startPos + len)}"`;
    });

    // FIND(search,text) - Matnda qidirish (case-sensitive)
    parsed = parsed.replace(/FIND\(([^,]+),([^)]+)\)/g, (_, search, text) => {
      const searchStr = search.trim().replace(/["']/g, '');
      const coords = cellLabelToCoords(text.trim());
      const val = coords ? getCellRawValue(coords.row, coords.col, data) : text.trim().replace(/["']/g, '');
      const pos = val.indexOf(searchStr);
      return pos >= 0 ? (pos + 1).toString() : '#VALUE!';
    });

    // SUBSTITUTE(text,old,new) - Matnni almashtirish
    parsed = parsed.replace(/SUBSTITUTE\(([^,]+),([^,]+),([^)]+)\)/g, (_, text, oldText, newText) => {
      const coords = cellLabelToCoords(text.trim());
      const val = coords ? getCellRawValue(coords.row, coords.col, data) : text.trim().replace(/["']/g, '');
      const oldStr = oldText.trim().replace(/["']/g, '');
      const newStr = newText.trim().replace(/["']/g, '');
      return `"${val.split(oldStr).join(newStr)}"`;
    });

    // TEXT(value, format)
    parsed = parsed.replace(/TEXT\(([^,]+),([^)]+)\)/g, (_, valueArg, formatArg) => {
      const value = getNumericValueFromArg(valueArg);
      const format = stripWrappingQuotes(formatArg);
      const decimals = format.includes('.') ? format.split('.')[1].replace(/[^0#]/g, '').length : 0;
      const formatted = decimals > 0 ? value.toFixed(decimals) : Math.round(value).toString();
      return `"${formatted}"`;
    });

    // VALUE(text)
    parsed = parsed.replace(/VALUE\(([^)]+)\)/g, (_, arg) => {
      const val = getRawValueFromArg(arg);
      const num = parseFloat(val);
      return isNaN(num) ? '#VALUE!' : num.toString();
    });

    // REPT(text, number)
    parsed = parsed.replace(/REPT\(([^,]+),([^)]+)\)/g, (_, textArg, numberArg) => {
      const text = getRawValueFromArg(textArg);
      const count = Math.max(0, Math.floor(getNumericValueFromArg(numberArg)));
      return `"${text.repeat(count)}"`;
    });

    // CLEAN(text)
    parsed = parsed.replace(/CLEAN\(([^)]+)\)/g, (_, arg) => {
      const text = getRawValueFromArg(arg);
      return `"${text.replace(/[\x00-\x1F]/g, '')}"`;
    });

    // SEARCH(search,text)
    parsed = parsed.replace(/SEARCH\(([^,]+),([^)]+)\)/g, (_, searchArg, textArg) => {
      const search = getRawValueFromArg(searchArg).toLowerCase();
      const text = getRawValueFromArg(textArg).toLowerCase();
      const index = text.indexOf(search);
      return index >= 0 ? (index + 1).toString() : '#VALUE!';
    });

    // EXACT(text1, text2)
    parsed = parsed.replace(/EXACT\(([^,]+),([^)]+)\)/g, (_, text1, text2) => {
      return getRawValueFromArg(text1) === getRawValueFromArg(text2) ? 'TRUE' : 'FALSE';
    });

    // TEXTJOIN(delimiter, ignore_empty, value1, ...)
    parsed = parsed.replace(/TEXTJOIN\(([^)]+)\)/g, (_, args) => {
      const parts = splitArgs(args);
      if (parts.length < 3) return '#VALUE!';
      const delimiter = stripWrappingQuotes(parts[0]);
      const ignoreEmpty = stripWrappingQuotes(parts[1]).toUpperCase() === 'TRUE';
      const values = parts.slice(2).map(p => getRawValueFromArg(p)).filter(val => (ignoreEmpty ? val !== '' : true));
      return `"${values.join(delimiter)}"`;
    });

    // SPLIT(text, delimiter)
    parsed = parsed.replace(/SPLIT\(([^,]+),([^)]+)\)/g, (_, textArg, delimiterArg) => {
      const text = getRawValueFromArg(textArg);
      const delimiter = stripWrappingQuotes(delimiterArg);
      return `"${text.split(delimiter).join(', ')}"`;
    });

    // CHAR(number)
    parsed = parsed.replace(/CHAR\(([^)]+)\)/g, (_, arg) => {
      const code = Math.max(0, Math.floor(getNumericValueFromArg(arg)));
      return `"${String.fromCharCode(code)}"`;
    });

    // CODE(text)
    parsed = parsed.replace(/CODE\(([^)]+)\)/g, (_, arg) => {
      const text = getRawValueFromArg(arg);
      return text.length ? text.charCodeAt(0).toString() : '0';
    });

    // ============ DATE/TIME FUNCTIONS ============

    // TODAY() - Bugungi sana
    parsed = parsed.replace(/TODAY\(\)/g, () => {
      const today = new Date();
      return `"${today.toLocaleDateString()}"`;
    });

    // NOW() - Hozirgi sana va vaqt
    parsed = parsed.replace(/NOW\(\)/g, () => {
      const now = new Date();
      return `"${now.toLocaleString()}"`;
    });

    // YEAR(date) - Yil
    parsed = parsed.replace(/YEAR\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellRawValue(coords.row, coords.col, data) : arg.trim().replace(/["']/g, '');
      const date = new Date(val);
      return isNaN(date.getTime()) ? '#VALUE!' : date.getFullYear().toString();
    });

    // MONTH(date) - Oy
    parsed = parsed.replace(/MONTH\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellRawValue(coords.row, coords.col, data) : arg.trim().replace(/["']/g, '');
      const date = new Date(val);
      return isNaN(date.getTime()) ? '#VALUE!' : (date.getMonth() + 1).toString();
    });

    // DAY(date) - Kun
    parsed = parsed.replace(/DAY\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellRawValue(coords.row, coords.col, data) : arg.trim().replace(/["']/g, '');
      const date = new Date(val);
      return isNaN(date.getTime()) ? '#VALUE!' : date.getDate().toString();
    });

    // HOUR(time) - Soat
    parsed = parsed.replace(/HOUR\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellRawValue(coords.row, coords.col, data) : arg.trim().replace(/["']/g, '');
      const date = new Date(val);
      return isNaN(date.getTime()) ? '#VALUE!' : date.getHours().toString();
    });

    // MINUTE(time) - Daqiqa
    parsed = parsed.replace(/MINUTE\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellRawValue(coords.row, coords.col, data) : arg.trim().replace(/["']/g, '');
      const date = new Date(val);
      return isNaN(date.getTime()) ? '#VALUE!' : date.getMinutes().toString();
    });

    // SECOND(time) - Soniya
    parsed = parsed.replace(/SECOND\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellRawValue(coords.row, coords.col, data) : arg.trim().replace(/["']/g, '');
      const date = new Date(val);
      return isNaN(date.getTime()) ? '#VALUE!' : date.getSeconds().toString();
    });

    // DATE(year, month, day)
    parsed = parsed.replace(/DATE\(([^,]+),([^,]+),([^)]+)\)/g, (_, yearArg, monthArg, dayArg) => {
      const year = Math.floor(getNumericValueFromArg(yearArg));
      const month = Math.floor(getNumericValueFromArg(monthArg)) - 1;
      const day = Math.floor(getNumericValueFromArg(dayArg));
      const date = new Date(year, month, day);
      return isNaN(date.getTime()) ? '#VALUE!' : `"${date.toISOString().split('T')[0]}"`;
    });

    // TIME(hour, minute, second)
    parsed = parsed.replace(/TIME\(([^,]+),([^,]+),([^)]+)\)/g, (_, hourArg, minArg, secArg) => {
      const hours = Math.floor(getNumericValueFromArg(hourArg));
      const minutes = Math.floor(getNumericValueFromArg(minArg));
      const seconds = Math.floor(getNumericValueFromArg(secArg));
      const date = new Date();
      date.setHours(hours, minutes, seconds, 0);
      return `"${date.toTimeString().split(' ')[0]}"`;
    });

    // EDATE(date, months)
    parsed = parsed.replace(/EDATE\(([^,]+),([^)]+)\)/g, (_, dateArg, monthsArg) => {
      const base = new Date(getRawValueFromArg(dateArg));
      const months = Math.floor(getNumericValueFromArg(monthsArg));
      if (isNaN(base.getTime())) return '#VALUE!';
      base.setMonth(base.getMonth() + months);
      return `"${base.toISOString().split('T')[0]}"`;
    });

    // EOMONTH(date, months)
    parsed = parsed.replace(/EOMONTH\(([^,]+),([^)]+)\)/g, (_, dateArg, monthsArg) => {
      const base = new Date(getRawValueFromArg(dateArg));
      const months = Math.floor(getNumericValueFromArg(monthsArg));
      if (isNaN(base.getTime())) return '#VALUE!';
      base.setMonth(base.getMonth() + months + 1, 0);
      return `"${base.toISOString().split('T')[0]}"`;
    });

    // DAYS(end_date, start_date)
    parsed = parsed.replace(/DAYS\(([^,]+),([^)]+)\)/g, (_, endArg, startArg) => {
      const end = new Date(getRawValueFromArg(endArg));
      const start = new Date(getRawValueFromArg(startArg));
      if (isNaN(end.getTime()) || isNaN(start.getTime())) return '#VALUE!';
      const diff = (end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24);
      return Math.round(diff).toString();
    });

    // DATEDIF(start_date, end_date, unit)
    parsed = parsed.replace(/DATEDIF\(([^,]+),([^,]+),([^)]+)\)/g, (_, startArg, endArg, unitArg) => {
      const start = new Date(getRawValueFromArg(startArg));
      const end = new Date(getRawValueFromArg(endArg));
      const unit = stripWrappingQuotes(unitArg).toUpperCase();
      if (isNaN(start.getTime()) || isNaN(end.getTime())) return '#VALUE!';
      const diffDays = (end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24);
      switch (unit) {
        case 'Y':
          return Math.floor(diffDays / 365).toString();
        case 'M':
          return Math.floor(diffDays / 30).toString();
        default:
          return Math.floor(diffDays).toString();
      }
    });

    // WEEKDAY(date, [type])
    parsed = parsed.replace(/WEEKDAY\(([^,]+)(?:,([^)]+))?\)/g, (_, dateArg, typeArg) => {
      const date = new Date(getRawValueFromArg(dateArg));
      if (isNaN(date.getTime())) return '#VALUE!';
      const type = typeArg ? Math.floor(getNumericValueFromArg(typeArg)) : 1;
      let day = date.getDay(); // Sunday 0
      if (type === 1) {
        return ((day || 7)).toString();
      } else if (type === 2) {
        return (day === 0 ? 7 : day).toString();
      }
      return (day + 1).toString();
    });

    // WEEKNUM(date, [type])
    parsed = parsed.replace(/WEEKNUM\(([^,]+)(?:,([^)]+))?\)/g, (_, dateArg, typeArg) => {
      const date = new Date(getRawValueFromArg(dateArg));
      if (isNaN(date.getTime())) return '#VALUE!';
      const type = typeArg ? Math.floor(getNumericValueFromArg(typeArg)) : 1;
      const firstDay = new Date(date.getFullYear(), 0, 1);
      const diff = (date.getTime() - firstDay.getTime()) / (1000 * 60 * 60 * 24);
      const week = Math.ceil((diff + firstDay.getDay() + (type === 2 ? 0 : 1)) / 7);
      return week.toString();
    });

    // NETWORKDAYS(start_date, end_date, [holidays...])
    parsed = parsed.replace(/NETWORKDAYS\(([^)]+)\)/g, (_, args) => {
      const parts = splitArgs(args);
      if (parts.length < 2) return '#VALUE!';
      const start = new Date(getRawValueFromArg(parts[0]));
      const end = new Date(getRawValueFromArg(parts[1]));
      if (isNaN(start.getTime()) || isNaN(end.getTime())) return '#VALUE!';
      const startDate = start <= end ? new Date(start) : new Date(end);
      const endDate = start <= end ? new Date(end) : new Date(start);
      const holidayDates = parts.slice(2).map(p => new Date(getRawValueFromArg(p)).toDateString());
      let count = 0;
      for (let d = new Date(startDate); d <= endDate; d.setDate(d.getDate() + 1)) {
        const day = d.getDay();
        const isWeekend = day === 0 || day === 6;
        if (!isWeekend && !holidayDates.includes(d.toDateString())) {
          count++;
        }
      }
      return count.toString();
    });

    // ============ LOOKUP FUNCTIONS ============

    // LOOKUP(lookup_value, lookup_vector, [result_vector])
    parsed = parsed.replace(/LOOKUP\(([^,]+),([^,]+)(?:,([^)]+))?\)/g, (_, lookupVal, lookupRange, resultRange) => {
      const search = getRawValueFromArg(lookupVal);
      const lookupValues = getRangeRawGeneral(lookupRange);
      const resultValues = resultRange ? getRangeRawGeneral(resultRange) : lookupValues;
      const length = Math.min(lookupValues.length, resultValues.length);
      for (let i = 0; i < length; i++) {
        if (lookupValues[i].toString() === search.toString()) {
          return resultValues[i];
        }
      }
      return '#N/A';
    });

    // VLOOKUP(lookup_value, table_range, col_index, [is_sorted])
    parsed = parsed.replace(/VLOOKUP\(([^,]+),([^,]+),([^,)]+)(?:,([^)]+))?\)/g, (_, lookupVal, tableRange, colIndex, isSorted) => {
      const lookupCoords = cellLabelToCoords(lookupVal.trim());
      const searchValue = lookupCoords ? getCellRawValue(lookupCoords.row, lookupCoords.col, data) : lookupVal.trim().replace(/["']/g, '');

      const [start, end] = tableRange.trim().split(':');
      const startCoords = cellLabelToCoords(start);
      const endCoords = cellLabelToCoords(end);

      if (!startCoords || !endCoords) return '#REF!';

      const colIdx = parseInt(colIndex.trim(), 10) - 1;

      for (let r = startCoords.row; r <= endCoords.row; r++) {
        const cellVal = getCellRawValue(r, startCoords.col, data);
        if (cellVal.toString() === searchValue.toString()) {
          const resultCol = startCoords.col + colIdx;
          if (resultCol > endCoords.col) return '#REF!';
          return getCellRawValue(r, resultCol, data);
        }
      }

      return '#N/A';
    });

    // HLOOKUP(lookup_value, table_range, row_index, [is_sorted])
    parsed = parsed.replace(/HLOOKUP\(([^,]+),([^,]+),([^,)]+)(?:,([^)]+))?\)/g, (_, lookupVal, tableRange, rowIndex, isSorted) => {
      const lookupCoords = cellLabelToCoords(lookupVal.trim());
      const searchValue = lookupCoords ? getCellRawValue(lookupCoords.row, lookupCoords.col, data) : lookupVal.trim().replace(/["']/g, '');

      const [start, end] = tableRange.trim().split(':');
      const startCoords = cellLabelToCoords(start);
      const endCoords = cellLabelToCoords(end);

      if (!startCoords || !endCoords) return '#REF!';

      const rowIdx = parseInt(rowIndex.trim(), 10) - 1;

      for (let c = startCoords.col; c <= endCoords.col; c++) {
        const cellVal = getCellRawValue(startCoords.row, c, data);
        if (cellVal.toString() === searchValue.toString()) {
          const resultRow = startCoords.row + rowIdx;
          if (resultRow > endCoords.row) return '#REF!';
          return getCellRawValue(resultRow, c, data);
        }
      }

      return '#N/A';
    });

    // INDEX(range, row, col) - Berilgan pozitsiyadagi qiymatni qaytarish
    parsed = parsed.replace(/INDEX\(([^,]+),([^,]+),([^)]+)\)/g, (_, rangeArg, rowArg, colArg) => {
      const [start, end] = rangeArg.trim().split(':');
      const startCoords = cellLabelToCoords(start);
      const endCoords = cellLabelToCoords(end);

      if (!startCoords || !endCoords) return '#REF!';

      const rowOffset = parseInt(rowArg.trim(), 10) - 1;
      const colOffset = parseInt(colArg.trim(), 10) - 1;

      const targetRow = startCoords.row + rowOffset;
      const targetCol = startCoords.col + colOffset;

      if (targetRow > endCoords.row || targetCol > endCoords.col) return '#REF!';

      return getCellRawValue(targetRow, targetCol, data);
    });

    // MATCH(lookup_value, range, [match_type]) - Qiymatning pozitsiyasini topish
    parsed = parsed.replace(/MATCH\(([^,]+),([^,)]+)(?:,([^)]+))?\)/g, (_, lookupVal, rangeArg, matchType) => {
      const lookupCoords = cellLabelToCoords(lookupVal.trim());
      const searchValue = lookupCoords ? getCellRawValue(lookupCoords.row, lookupCoords.col, data) : lookupVal.trim().replace(/["']/g, '');

      const [start, end] = rangeArg.trim().split(':');
      const startCoords = cellLabelToCoords(start);
      const endCoords = cellLabelToCoords(end);

      if (!startCoords || !endCoords) return '#REF!';

      // Vertical range
      if (startCoords.col === endCoords.col) {
        for (let r = startCoords.row; r <= endCoords.row; r++) {
          if (getCellRawValue(r, startCoords.col, data).toString() === searchValue.toString()) {
            return (r - startCoords.row + 1).toString();
          }
        }
      }
      // Horizontal range
      else if (startCoords.row === endCoords.row) {
        for (let c = startCoords.col; c <= endCoords.col; c++) {
          if (getCellRawValue(startCoords.row, c, data).toString() === searchValue.toString()) {
            return (c - startCoords.col + 1).toString();
          }
        }
      }

      return '#N/A';
    });

    // ============ INFORMATION FUNCTIONS ============

    // ISBLANK(A1) - Bo'sh katakchami?
    parsed = parsed.replace(/ISBLANK\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      if (!coords) return 'FALSE';
      const val = getCellRawValue(coords.row, coords.col, data);
      return val === '' ? 'TRUE' : 'FALSE';
    });

    // ISNUMBER(A1) - Raqammi?
    parsed = parsed.replace(/ISNUMBER\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      if (!coords) return 'FALSE';
      const val = getCellRawValue(coords.row, coords.col, data);
      return isNumeric(val) ? 'TRUE' : 'FALSE';
    });

    // ISTEXT(A1) - Matnmi?
    parsed = parsed.replace(/ISTEXT\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      if (!coords) return 'FALSE';
      const val = getCellRawValue(coords.row, coords.col, data);
      return !isNumeric(val) && val !== '' ? 'TRUE' : 'FALSE';
    });

    // ISERROR(A1) - Xatomi?
    parsed = parsed.replace(/ISERROR\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      if (!coords) return 'FALSE';
      const val = getCellRawValue(coords.row, coords.col, data);
      return val.startsWith('#') ? 'TRUE' : 'FALSE';
    });

    // ISNA(A1)
    parsed = parsed.replace(/ISNA\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      if (!coords) return 'FALSE';
      const val = getCellRawValue(coords.row, coords.col, data);
      return val === '#N/A' ? 'TRUE' : 'FALSE';
    });

    // ISEVEN(A1)
    parsed = parsed.replace(/ISEVEN\(([^)]+)\)/g, (_, arg) => {
      const val = getNumericValueFromArg(arg);
      return val % 2 === 0 ? 'TRUE' : 'FALSE';
    });

    // ISODD(A1)
    parsed = parsed.replace(/ISODD\(([^)]+)\)/g, (_, arg) => {
      const val = getNumericValueFromArg(arg);
      return Math.abs(val % 2) === 1 ? 'TRUE' : 'FALSE';
    });

    // ============ TRIGONOMETRIC FUNCTIONS ============

    // SIN(A1) - Sinus
    parsed = parsed.replace(/SIN\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(arg);
      return Math.sin(val).toString();
    });

    // COS(A1) - Cosinus
    parsed = parsed.replace(/COS\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(arg);
      return Math.cos(val).toString();
    });

    // TAN(A1) - Tangens
    parsed = parsed.replace(/TAN\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(arg);
      return Math.tan(val).toString();
    });

    // ASIN(A1) - Arcsin
    parsed = parsed.replace(/ASIN\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(arg);
      return Math.asin(val).toString();
    });

    // ACOS(A1) - Arccos
    parsed = parsed.replace(/ACOS\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(arg);
      return Math.acos(val).toString();
    });

    // ATAN(A1) - Arctan
    parsed = parsed.replace(/ATAN\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(arg);
      return Math.atan(val).toString();
    });

    // PI() - Pi soni
    parsed = parsed.replace(/PI\(\)/g, () => Math.PI.toString());

    // RADIANS(degrees) - Gradusni radianga aylantirish
    parsed = parsed.replace(/RADIANS\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(arg);
      return (val * Math.PI / 180).toString();
    });

    // DEGREES(radians) - Radianini gradusga aylantirish
    parsed = parsed.replace(/DEGREES\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(arg);
      return (val * 180 / Math.PI).toString();
    });

    // ============ OTHER USEFUL FUNCTIONS ============

    // RAND() - Tasodifiy son (0 va 1 orasida)
    parsed = parsed.replace(/RAND\(\)/g, () => Math.random().toString());

    // RANDBETWEEN(bottom, top) - Ikki son orasida tasodifiy butun son
    parsed = parsed.replace(/RANDBETWEEN\(([^,]+),([^)]+)\)/g, (_, bottom, top) => {
      const coordsBot = cellLabelToCoords(bottom.trim());
      const coordsTop = cellLabelToCoords(top.trim());
      const botVal = coordsBot ? getCellValue(coordsBot.row, coordsBot.col, data) : parseFloat(bottom);
      const topVal = coordsTop ? getCellValue(coordsTop.row, coordsTop.col, data) : parseFloat(top);
      return Math.floor(Math.random() * (topVal - botVal + 1) + botVal).toString();
    });

    // SUMSQ(A1:A10) - Kvadratlar yig'indisi
    parsed = parsed.replace(/SUMSQ\(([^)]+)\)/g, (_, args) => {
      const vals: number[] = [];
      args.split(',').forEach((arg: string) => {
        if (arg.includes(':')) {
          const [start, end] = arg.split(':');
          vals.push(...expandRange(start.trim(), end.trim()));
        }
      });
      return vals.reduce((sum, val) => sum + val * val, 0).toString();
    });

    // EXP(A1) - e ning darajasi
    parsed = parsed.replace(/EXP\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(arg);
      return Math.exp(val).toString();
    });

    // LN(A1) - Natural logarifm
    parsed = parsed.replace(/LN\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(arg);
      return Math.log(val).toString();
    });

    // LOG(A1, base) - Logarifm (asosli)
    parsed = parsed.replace(/LOG\(([^,]+),([^)]+)\)/g, (_, numArg, base) => {
      const coordsNum = cellLabelToCoords(numArg.trim());
      const coordsBase = cellLabelToCoords(base.trim());
      const numVal = coordsNum ? getCellValue(coordsNum.row, coordsNum.col, data) : parseFloat(numArg);
      const baseVal = coordsBase ? getCellValue(coordsBase.row, coordsBase.col, data) : parseFloat(base);
      return (Math.log(numVal) / Math.log(baseVal)).toString();
    });

    // LOG10(A1) - 10-lik logarifm
    parsed = parsed.replace(/LOG10\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(arg);
      return Math.log10(val).toString();
    });

    // FACT(A1) - Faktorial
    parsed = parsed.replace(/FACT\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(arg);
      const n = Math.floor(val);
      if (n < 0) return '#NUM!';
      let result = 1;
      for (let i = 2; i <= n; i++) {
        result *= i;
      }
      return result.toString();
    });

    // GCD(A1,B1) - Eng katta umumiy bo'luvchi
    parsed = parsed.replace(/GCD\(([^,]+),([^)]+)\)/g, (_, arg1, arg2) => {
      const coords1 = cellLabelToCoords(arg1.trim());
      const coords2 = cellLabelToCoords(arg2.trim());
      let a = Math.floor(coords1 ? getCellValue(coords1.row, coords1.col, data) : parseFloat(arg1));
      let b = Math.floor(coords2 ? getCellValue(coords2.row, coords2.col, data) : parseFloat(arg2));

      while (b !== 0) {
        const temp = b;
        b = a % b;
        a = temp;
      }
      return Math.abs(a).toString();
    });

    // LCM(A1,B1) - Eng kichik umumiy karrali
    parsed = parsed.replace(/LCM\(([^,]+),([^)]+)\)/g, (_, arg1, arg2) => {
      const coords1 = cellLabelToCoords(arg1.trim());
      const coords2 = cellLabelToCoords(arg2.trim());
      const a = Math.floor(coords1 ? getCellValue(coords1.row, coords1.col, data) : parseFloat(arg1));
      const b = Math.floor(coords2 ? getCellValue(coords2.row, coords2.col, data) : parseFloat(arg2));

      // Calculate GCD
      let gcdA = a, gcdB = b;
      while (gcdB !== 0) {
        const temp = gcdB;
        gcdB = gcdA % gcdB;
        gcdA = temp;
      }

      return Math.abs((a * b) / gcdA).toString();
    });

    // EVEN(A1) - Eng yaqin juft songa yaxlitlash
    parsed = parsed.replace(/EVEN\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(arg);
      return (Math.ceil(val / 2) * 2).toString();
    });

    // ODD(A1) - Eng yaqin toq songa yaxlitlash
    parsed = parsed.replace(/ODD\(([^)]+)\)/g, (_, arg) => {
      const coords = cellLabelToCoords(arg.trim());
      const val = coords ? getCellValue(coords.row, coords.col, data) : parseFloat(arg);
      const rounded = Math.ceil(Math.abs(val));
      const result = rounded % 2 === 0 ? rounded + 1 : rounded;
      return (val < 0 ? -result : result).toString();
    });

    // 2. Handle Individual Cell References (A1, B5)
    parsed = parsed.replace(/[A-Z]+[0-9]+/g, (match) => {
      const coords = cellLabelToCoords(match);
      if (!coords) return '0';
      return getCellValue(coords.row, coords.col, data).toString();
    });

    // 3. Handle string literals in final result
    if (parsed.startsWith('"') && parsed.endsWith('"')) {
      return parsed.substring(1, parsed.length - 1);
    }

    // 4. Safe Evaluation without eval/Function
    // Only allow digits, operators, parenthesis, and decimal points
    if (!/^[0-9+\-*/().\s]+$/.test(parsed)) {
      return "#ERROR";
    }

    // Safe mathematical expression evaluator
    return safeEvaluateExpression(parsed);
  } catch (e) {
    return "#ERROR";
  }
};

// Safe expression evaluator without eval or Function constructor
const safeEvaluateExpression = (expr: string): number | string => {
  try {
    // Tokenize the expression
    const tokens: string[] = [];
    let current = '';

    for (let i = 0; i < expr.length; i++) {
      const char = expr[i];
      if (/[0-9.]/.test(char)) {
        current += char;
      } else if (/[+\-*/()]/.test(char)) {
        if (current) {
          tokens.push(current);
          current = '';
        }
        if (char.trim()) tokens.push(char);
      } else if (char === ' ') {
        if (current) {
          tokens.push(current);
          current = '';
        }
      }
    }
    if (current) tokens.push(current);

    // Convert to postfix notation (Reverse Polish Notation)
    const toPostfix = (tokens: string[]): string[] => {
      const output: string[] = [];
      const operators: string[] = [];
      const precedence: Record<string, number> = { '+': 1, '-': 1, '*': 2, '/': 2 };

      tokens.forEach(token => {
        if (!isNaN(parseFloat(token))) {
          output.push(token);
        } else if (token === '(') {
          operators.push(token);
        } else if (token === ')') {
          while (operators.length && operators[operators.length - 1] !== '(') {
            output.push(operators.pop()!);
          }
          operators.pop(); // Remove '('
        } else if (precedence[token]) {
          while (
            operators.length &&
            operators[operators.length - 1] !== '(' &&
            precedence[operators[operators.length - 1]] >= precedence[token]
          ) {
            output.push(operators.pop()!);
          }
          operators.push(token);
        }
      });

      while (operators.length) {
        output.push(operators.pop()!);
      }

      return output;
    };

    // Evaluate postfix expression
    const evalPostfix = (postfix: string[]): number => {
      const stack: number[] = [];

      postfix.forEach(token => {
        if (!isNaN(parseFloat(token))) {
          stack.push(parseFloat(token));
        } else {
          const b = stack.pop()!;
          const a = stack.pop()!;
          switch (token) {
            case '+': stack.push(a + b); break;
            case '-': stack.push(a - b); break;
            case '*': stack.push(a * b); break;
            case '/': stack.push(a / b); break;
          }
        }
      });

      return stack[0];
    };

    const postfix = toPostfix(tokens);
    const result = evalPostfix(postfix);
    return isFinite(result) ? result : '#ERROR';
  } catch (e) {
    return '#ERROR';
  }
};

// Build dependency graph for cells
const buildDependencyGraph = (data: GridData): Map<string, Set<string>> => {
  const deps = new Map<string, Set<string>>();

  Object.keys(data).forEach(cellId => {
    const cell = data[cellId];
    if (!cell.value.startsWith('=')) return;

    const formula = cell.value.substring(1).toUpperCase();
    const references = new Set<string>();

    // Extract cell references (A1, B5, etc.)
    const cellRefRegex = /[A-Z]+[0-9]+/g;
    const matches = formula.match(cellRefRegex);
    if (matches) {
      matches.forEach(ref => references.add(ref));
    }

    // Extract range references (A1:B5)
    const rangeRegex = /([A-Z]+[0-9]+):([A-Z]+[0-9]+)/g;
    let rangeMatch;
    while ((rangeMatch = rangeRegex.exec(formula)) !== null) {
      const startCoords = cellLabelToCoords(rangeMatch[1]);
      const endCoords = cellLabelToCoords(rangeMatch[2]);
      if (startCoords && endCoords) {
        const minRow = Math.min(startCoords.row, endCoords.row);
        const maxRow = Math.max(startCoords.row, endCoords.row);
        const minCol = Math.min(startCoords.col, endCoords.col);
        const maxCol = Math.max(startCoords.col, endCoords.col);

        for (let r = minRow; r <= maxRow; r++) {
          for (let c = minCol; c <= maxCol; c++) {
            references.add(getColumnLabel(c) + (r + 1));
          }
        }
      }
    }

    deps.set(cellId, references);
  });

  return deps;
};

// Detect circular dependencies using DFS
const detectCircularDependency = (
  cellId: string,
  deps: Map<string, Set<string>>,
  visited: Set<string>,
  recStack: Set<string>
): boolean => {
  visited.add(cellId);
  recStack.add(cellId);

  const cellDeps = deps.get(cellId);
  if (cellDeps) {
    for (const depId of cellDeps) {
      const depCoords = cellLabelToCoords(depId);
      if (!depCoords) continue;

      const depCellId = getCellId(depCoords.row, depCoords.col);

      if (!visited.has(depCellId)) {
        if (detectCircularDependency(depCellId, deps, visited, recStack)) {
          return true;
        }
      } else if (recStack.has(depCellId)) {
        return true;
      }
    }
  }

  recStack.delete(cellId);
  return false;
};

// Topological sort using DFS
const topologicalSort = (deps: Map<string, Set<string>>): string[] => {
  const visited = new Set<string>();
  const result: string[] = [];

  const dfs = (cellId: string) => {
    if (visited.has(cellId)) return;
    visited.add(cellId);

    const cellDeps = deps.get(cellId);
    if (cellDeps) {
      for (const depId of cellDeps) {
        const depCoords = cellLabelToCoords(depId);
        if (!depCoords) continue;

        const depCellId = getCellId(depCoords.row, depCoords.col);
        dfs(depCellId);
      }
    }

    result.push(cellId);
  };

  deps.forEach((_, cellId) => {
    dfs(cellId);
  });

  return result;
};

export const recomputeSheet = (data: GridData): GridData => {
  const newData = { ...data };

  // First pass: compute non-formula cells
  Object.keys(newData).forEach(key => {
    const cell = newData[key];
    if (!cell.value.startsWith('=')) {
      const num = parseFloat(cell.value);
      cell.computed = isNaN(num) ? cell.value : num;
    }
  });

  // Build dependency graph
  const deps = buildDependencyGraph(newData);

  // Detect circular dependencies
  const visited = new Set<string>();
  const recStack = new Set<string>();
  const circularCells = new Set<string>();

  deps.forEach((_, cellId) => {
    if (!visited.has(cellId)) {
      const visitedInThisRun = new Set<string>();
      const stackInThisRun = new Set<string>();
      if (detectCircularDependency(cellId, deps, visitedInThisRun, stackInThisRun)) {
        visitedInThisRun.forEach(id => circularCells.add(id));
      }
      visitedInThisRun.forEach(id => visited.add(id));
    }
  });

  // Mark circular cells as error
  circularCells.forEach(cellId => {
    if (newData[cellId]) {
      newData[cellId].computed = '#CIRCULAR!';
    }
  });

  // Topological sort to evaluate in correct order
  const sortedCells = topologicalSort(deps);

  // Evaluate formulas in topological order
  sortedCells.forEach(cellId => {
    const cell = newData[cellId];
    if (cell && cell.value.startsWith('=') && !circularCells.has(cellId)) {
      cell.computed = evaluateFormula(cell.value, newData);
    }
  });

  return newData;
};
