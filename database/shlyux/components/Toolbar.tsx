import React, { useState, useRef, useEffect } from 'react';
import ReactDOM from 'react-dom';
import {
  Bold, Italic, Underline, AlignLeft, AlignCenter, AlignRight,
  Type, PaintBucket, Sparkles, Search, X, Undo2, Redo2, Eraser,
  AlignVerticalJustifyStart, AlignVerticalJustifyCenter, AlignVerticalJustifyEnd,
  TextWrap, Printer, Paintbrush, Merge, Square, Plus, Minus, Save, ChevronDown
} from 'lucide-react';
import { CellStyle } from '../types';
import { usePresence } from '../utils/usePresence';
import ColorPicker from './ColorPicker';

interface ToolbarProps {
  onStyleChange: (style: Partial<CellStyle>) => void;
  activeStyle: CellStyle;
  isAiOpen?: boolean;
  onToggleAi: () => void;
  onOpenFindReplace?: () => void;
  onUndo?: () => void;
  onRedo?: () => void;
  canUndo?: boolean;
  canRedo?: boolean;
  onPrint?: () => void;
  onFormatPainter?: () => void;
  formatPainterActive?: boolean;
  onMergeCells?: () => void;
  onUnmergeCells?: () => void;
  isMerged?: boolean;
  autoSaveEnabled?: boolean;
  onToggleAutoSave?: () => void;
  density?: 'comfortable' | 'compact';
}

const Toolbar: React.FC<ToolbarProps> = ({
  onStyleChange,
  activeStyle,
  isAiOpen,
  onToggleAi,
  onOpenFindReplace,
  onUndo,
  onRedo,
  canUndo,
  canRedo,
  onPrint,
  onFormatPainter,
  formatPainterActive,
  onMergeCells,
  onUnmergeCells,
  isMerged,
  autoSaveEnabled,
  onToggleAutoSave,
  density
}) => {
  const [showFontDropdown, setShowFontDropdown] = useState(false);
  const fontDropdownRef = useRef<HTMLDivElement>(null);
  const fontDropdownPresence = usePresence(showFontDropdown, { exitDurationMs: 180 });

  const btnClass = (active: boolean) =>
    `p-2 rounded hover:bg-gray-100 transition-colors ${active ? 'bg-blue-100 text-blue-600 border border-blue-300' : 'text-primary'}`;

  const FONT_FAMILIES = [
    'Inter',
    'Poppins',
    'Roboto',
    'Open Sans',
    'Lato',
    'Montserrat',
    'Raleway',
    'Outfit',
    'Space Grotesk',
    'JetBrains Mono',
    'Fira Code',
    'Source Code Pro',
    'Playfair Display',
    'Merriweather',
    'Crimson Text',
    'Lora',
    'Bebas Neue',
    'Oswald',
    'Ubuntu',
    'Quicksand',
    'Nunito',
    'Comfortaa',
    'Arial',
    'Georgia',
    'Times New Roman',
    'Courier New'
  ];

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (fontDropdownRef.current && !fontDropdownRef.current.contains(event.target as Node)) {
        setShowFontDropdown(false);
      }
    };

    if (showFontDropdown) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [showFontDropdown]);
  const FONT_SIZES = [8, 10, 12, 13, 14, 16, 18, 24, 32, 48];
  const NUMBER_FORMATS: { label: string; value: CellStyle['numberFormat'] }[] = [
    { label: 'General', value: 'general' },
    { label: 'Number', value: 'number' },
    { label: 'Currency', value: 'currency' },
    { label: 'Percent', value: 'percent' }
  ];

  const currentFont = activeStyle.fontFamily || FONT_FAMILIES[0];
  const currentSize = activeStyle.fontSize || 13;
  const currentFormat = activeStyle.numberFormat || 'general';
  const currentDecimals = activeStyle.decimalPlaces ?? 2;
  const currentVertical = activeStyle.verticalAlign || 'middle';
  const currentWrap = activeStyle.wrapMode || 'overflow';
  const toolbarHeight = density === 'compact' ? 44 : 48;

  const handleDecimalChange = (delta: number) => {
    const base = Math.max(0, currentDecimals + delta);
    onStyleChange({
      numberFormat: activeStyle.numberFormat || 'number',
      decimalPlaces: base
    });
  };

  return (
    <div
      className="border-b flex items-center px-4 space-x-2 shadow-sm z-20"
      style={{ height: toolbarHeight, borderColor: 'var(--border-color)', background: 'var(--card-bg)' }}
    >
      {/* Undo/Redo */}
      <div className="flex items-center space-x-1 border-r pr-2" style={{ borderColor: 'var(--border-color)' }}>
        <button
          className="p-2 rounded transition-colors"
          onClick={onUndo}
          disabled={!canUndo}
          title="Undo (Ctrl+Z)"
          style={{ color: 'var(--text-primary)', background: 'transparent', opacity: canUndo ? 1 : 0.3, cursor: canUndo ? 'pointer' : 'not-allowed' }}
          onMouseEnter={(e) => canUndo && (e.currentTarget.style.background = 'var(--bg-light)')}
          onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
        >
          <Undo2 size={18} />
        </button>
        <button
          className="p-2 rounded transition-colors"
          onClick={onRedo}
          disabled={!canRedo}
          title="Redo (Ctrl+Y)"
          style={{ color: 'var(--text-primary)', background: 'transparent', opacity: canRedo ? 1 : 0.3, cursor: canRedo ? 'pointer' : 'not-allowed' }}
          onMouseEnter={(e) => canRedo && (e.currentTarget.style.background = 'var(--bg-light)')}
          onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
        >
          <Redo2 size={18} />
        </button>
        <button
          className="p-2 rounded transition-colors"
          onClick={onPrint}
          title="Print"
          style={{ color: 'var(--text-primary)', background: 'transparent' }}
          onMouseEnter={(e) => (e.currentTarget.style.background = 'var(--bg-light)')}
          onMouseLeave={(e) => (e.currentTarget.style.background = 'transparent')}
        >
          <Printer size={18} />
        </button>
      </div>

      {/* Format Painter */}
      <div className="flex items-center border-r pr-2" style={{ borderColor: 'var(--border-color)' }}>
        <button
          className={btnClass(!!formatPainterActive)}
          onClick={onFormatPainter}
          title="Format Painter"
        >
          <Paintbrush size={18} />
        </button>
      </div>

      <div className="flex items-center space-x-2 border-r pr-2" style={{ borderColor: 'var(--border-color)' }}>
        {/* Custom Animated Font Dropdown */}
        <div className="relative" ref={fontDropdownRef}>
          <button
            onClick={() => setShowFontDropdown(!showFontDropdown)}
            className="h-8 px-3 pr-8 text-sm border rounded flex items-center justify-between hover:bg-gray-50 transition-colors relative min-w-[140px]"
            style={{
              borderColor: 'var(--border-color)',
              background: 'var(--card-bg)',
              color: 'var(--text-primary)',
              fontFamily: currentFont
            }}
          >
            <span className="truncate">{currentFont}</span>
            <ChevronDown
              size={14}
              className="absolute right-2 top-1/2 -translate-y-1/2 transition-transform duration-200"
              style={{ transform: showFontDropdown ? 'translateY(-50%) rotate(180deg)' : 'translateY(-50%)' }}
            />
          </button>

          {fontDropdownPresence.isMounted && (
            <>
              <style>{`
	                .font-dropdown-item {
	                  transition: all 0.15s ease;
	                }
	                .font-dropdown-item:hover {
	                  background: var(--bg-light);
	                  padding-left: 16px;
	                }
	                .font-dropdown-menu-container {
	                  position: absolute !important;
	                  top: 100% !important;
                  left: 0 !important;
                  z-index: 999999 !important;
                  margin-top: 4px !important;
                }
              `}</style>
              <div
                className="font-dropdown-menu-container ui-popover w-64 max-h-96 overflow-y-auto border rounded-lg shadow-xl"
                data-state={fontDropdownPresence.state}
                style={{
                  background: 'var(--card-bg)',
                  borderColor: 'var(--border-color)',
                  boxShadow: '0 10px 40px rgba(0, 0, 0, 0.15), 0 0 0 1px rgba(0, 0, 0, 0.05)',
                  transformOrigin: 'top left'
                }}
              >
                {FONT_FAMILIES.map((font) => {
                  const isSelected = currentFont === font;
                  return (
                  <button
                    key={font}
                    onClick={(e) => {
                      e.preventDefault();
                      e.stopPropagation();
                      onStyleChange({ fontFamily: font });
                      setShowFontDropdown(false);
                    }}
                    className="font-dropdown-item w-full text-left px-3 py-2.5 text-sm hover:bg-blue-50 border-b last:border-b-0"
                    style={{
                      fontFamily: font,
                      borderColor: 'rgba(0,0,0,0.05)',
                      background: isSelected ? 'var(--sheet-selection-fill-weak)' : 'transparent',
                      color: isSelected ? 'var(--brand)' : 'var(--text-primary)',
                      fontWeight: isSelected ? 600 : 400,
                      cursor: 'pointer'
                    }}
                  >
                    <div className="flex items-center justify-between">
                      <span>{font}</span>
                      {isSelected && (
                        <span className="text-xs" style={{ color: 'var(--brand)' }}>✓</span>
                      )}
                    </div>
                  </button>
                  );
                })}
              </div>
            </>
          )}
        </div>
        <div className="flex items-center space-x-1">
          <button
            className="p-1 rounded hover:bg-gray-100 transition-colors"
            onClick={() => onStyleChange({ fontSize: Math.max(8, currentSize - 1) })}
            title="Decrease font size"
            style={{ color: 'var(--text-primary)' }}
          >
            <Minus size={14} />
          </button>
          <select
            value={currentSize}
            onChange={(e) => onStyleChange({ fontSize: parseInt(e.target.value, 10) })}
            className="h-8 px-2 text-sm border rounded w-16"
            style={{ borderColor: 'var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
          >
            {FONT_SIZES.map(size => (
              <option key={size} value={size}>{size}</option>
            ))}
          </select>
          <button
            className="p-1 rounded hover:bg-gray-100 transition-colors"
            onClick={() => onStyleChange({ fontSize: Math.min(72, currentSize + 1) })}
            title="Increase font size"
            style={{ color: 'var(--text-primary)' }}
          >
            <Plus size={14} />
          </button>
        </div>
      </div>

      <div className="flex items-center space-x-1 border-r pr-2" style={{ borderColor: 'var(--border-color)' }}>
        <button className={btnClass(!!activeStyle.bold)} onClick={() => onStyleChange({ bold: !activeStyle.bold })}>
          <Bold size={18} />
        </button>
        <button className={btnClass(!!activeStyle.italic)} onClick={() => onStyleChange({ italic: !activeStyle.italic })}>
          <Italic size={18} />
        </button>
        <button className={btnClass(!!activeStyle.underline)} onClick={() => onStyleChange({ underline: !activeStyle.underline })}>
          <Underline size={18} />
        </button>
      </div>

      <div className="flex items-center space-x-1 border-r pr-2" style={{ borderColor: 'var(--border-color)' }}>
        <button className={btnClass(activeStyle.textAlign === 'left')} onClick={() => onStyleChange({ textAlign: 'left' })}>
          <AlignLeft size={18} />
        </button>
        <button className={btnClass(activeStyle.textAlign === 'center')} onClick={() => onStyleChange({ textAlign: 'center' })}>
          <AlignCenter size={18} />
        </button>
        <button className={btnClass(activeStyle.textAlign === 'right')} onClick={() => onStyleChange({ textAlign: 'right' })}>
          <AlignRight size={18} />
        </button>
      </div>

      {/* Merge Cells */}
      <div className="flex items-center space-x-1 border-r pr-2" style={{ borderColor: 'var(--border-color)' }}>
        {isMerged ? (
          <button
            className="p-2 rounded hover:bg-gray-100 transition-colors"
            onClick={onUnmergeCells}
            title="Unmerge cells"
            style={{ color: 'var(--text-primary)' }}
          >
            <Merge size={18} />
          </button>
        ) : (
          <button
            className="p-2 rounded hover:bg-gray-100 transition-colors"
            onClick={onMergeCells}
            title="Merge cells"
            style={{ color: 'var(--text-primary)' }}
          >
            <Merge size={18} />
          </button>
        )}
      </div>

      {/* Borders */}
      <div className="flex items-center space-x-1 border-r pr-2" style={{ borderColor: 'var(--border-color)' }}>
        <button
          className={btnClass(!!(activeStyle.borders?.top && activeStyle.borders?.right && activeStyle.borders?.bottom && activeStyle.borders?.left))}
          onClick={() => {
            const hasAllBorders = activeStyle.borders?.top && activeStyle.borders?.right && activeStyle.borders?.bottom && activeStyle.borders?.left;
            onStyleChange({
              borders: hasAllBorders ? {} : {
                top: true,
                right: true,
                bottom: true,
                left: true,
                color: 'var(--text-primary)',
                style: 'solid'
              }
            });
          }}
          title="All borders"
        >
          <Square size={18} />
        </button>
      </div>

      <div className="flex items-center space-x-1 border-r pr-2" style={{ borderColor: 'var(--border-color)' }}>
        <button
          className={btnClass(currentVertical === 'top')}
          onClick={() => onStyleChange({ verticalAlign: 'top' })}
          title="Align Top"
        >
          <AlignVerticalJustifyStart size={18} />
        </button>
        <button
          className={btnClass(currentVertical === 'middle')}
          onClick={() => onStyleChange({ verticalAlign: 'middle' })}
          title="Align Middle"
        >
          <AlignVerticalJustifyCenter size={18} />
        </button>
        <button
          className={btnClass(currentVertical === 'bottom')}
          onClick={() => onStyleChange({ verticalAlign: 'bottom' })}
          title="Align Bottom"
        >
          <AlignVerticalJustifyEnd size={18} />
        </button>
      </div>

      <div className="flex items-center space-x-2 border-r pr-2" style={{ borderColor: 'var(--border-color)' }}>
        <div className="flex items-center space-x-1">
          {['overflow', 'wrap', 'clip'].map(mode => (
            <button
              key={mode}
              className="px-2 py-1 text-xs rounded border hover:opacity-80 transition-opacity"
              style={{
                borderColor: currentWrap === mode ? 'var(--sheet-selection-border)' : 'var(--border-color)',
                background: currentWrap === mode ? 'var(--sheet-selection-fill-weak)' : 'var(--card-bg)',
                color: currentWrap === mode ? 'var(--brand)' : 'var(--text-primary)'
              }}
              onClick={() => onStyleChange({ wrapMode: mode as CellStyle['wrapMode'] })}
            >
              {mode === 'wrap' ? <TextWrap size={16} /> : mode.charAt(0).toUpperCase() + mode.slice(1)}
            </button>
          ))}
        </div>
      </div>

      <div className="flex items-center space-x-2 border-r pr-2" style={{ borderColor: 'var(--border-color)' }}>
        <select
          value={currentFormat}
          onChange={(e) => onStyleChange({ numberFormat: e.target.value as CellStyle['numberFormat'] })}
          className="h-8 px-2 text-sm border rounded"
          style={{ borderColor: 'var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
        >
          {NUMBER_FORMATS.map(format => (
            <option key={format.value || 'general'} value={format.value || 'general'}>
              {format.label}
            </option>
          ))}
        </select>
        <button
          className="px-2 py-1 rounded border text-sm hover:bg-gray-100"
          style={{ borderColor: 'var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
          onClick={() => handleDecimalChange(-1)}
        >
          -.0
        </button>
        <button
          className="px-2 py-1 rounded border text-sm hover:bg-gray-100"
          style={{ borderColor: 'var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
          onClick={() => handleDecimalChange(1)}
        >
          +.0
        </button>
        <button
          className="px-2 py-1 rounded border text-sm hover:bg-gray-100"
          style={{ borderColor: 'var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
          onClick={() => onStyleChange({ numberFormat: 'percent', decimalPlaces: currentDecimals })}
        >
          %
        </button>
      </div>

      <div className="flex items-center space-x-2 border-r pr-2" style={{ borderColor: 'var(--border-color)' }}>
        <div className="flex items-center space-x-1">
          <Type size={18} style={{ color: 'var(--text-secondary)' }} />
          <ColorPicker
            title="Matn rangi"
            value={activeStyle.color}
            defaultLabel="Avto"
            defaultSwatch="var(--text-primary)"
            onChange={(next) => onStyleChange({ color: next })}
          />
        </div>
        <div className="flex items-center space-x-1">
          <PaintBucket size={18} style={{ color: 'var(--text-secondary)' }} />
          <ColorPicker
            title="Katak rangi"
            value={activeStyle.backgroundColor}
            defaultLabel="Standart"
            defaultSwatch="var(--sheet-cell-bg)"
            onChange={(next) => onStyleChange({ backgroundColor: next })}
          />
        </div>
      </div>

      <div className="flex items-center space-x-2 border-r pr-2" style={{ borderColor: 'var(--border-color)' }}>
        <button
          className="flex items-center px-3 py-2 rounded border hover:bg-gray-100 text-sm"
          style={{ borderColor: 'var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
          onClick={() => onStyleChange({ __reset: true } as any)}
        >
          <Eraser size={16} className="mr-1" />
          Clear
        </button>
      </div>

      <div className="flex-1"></div>

      {/* Auto Save Toggle */}
      {onToggleAutoSave && (
        <button
          onClick={onToggleAutoSave}
          className={`flex items-center gap-1.5 px-3 h-9 rounded border shadow-sm transition-all ${autoSaveEnabled ? 'border-green-400' : ''}`}
          style={{ borderColor: autoSaveEnabled ? '#4ade80' : 'var(--border-color)', background: autoSaveEnabled ? '#f0fdf4' : 'var(--card-bg)' }}
          title={autoSaveEnabled ? "Auto Save yoqilgan" : "Auto Save o'chirilgan"}
          onMouseEnter={(e) => !autoSaveEnabled && (e.currentTarget.style.background = 'var(--bg-light)')}
          onMouseLeave={(e) => !autoSaveEnabled && (e.currentTarget.style.background = 'var(--card-bg)')}
        >
          <Save size={14} className={autoSaveEnabled ? 'text-green-600' : 'text-gray-600'} />
          <span className={`text-xs font-medium ${autoSaveEnabled ? 'text-green-700' : 'text-gray-700'}`}>
            Auto Save
          </span>
        </button>
      )}

      {onOpenFindReplace && (
        <button
          onClick={onOpenFindReplace}
          className="flex items-center justify-center w-9 h-9 rounded border hover:bg-gray-100 shadow-sm"
          style={{ borderColor: 'var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
          title="Qidirish (Ctrl+F)"
        >
          <Search size={16} />
        </button>
      )}

      <button
        onClick={onToggleAi}
        className="flex items-center justify-center w-9 h-9 rounded border hover:bg-gray-100 shadow-sm"
        style={{
          borderColor: isAiOpen ? 'var(--primary)' : 'var(--border-color)',
          background: isAiOpen ? 'var(--bg-light)' : 'var(--card-bg)',
          color: 'var(--text-primary)'
        }}
        title={isAiOpen ? "AI (yopish)" : 'AI'}
      >
        {isAiOpen ? <X size={16} /> : <Sparkles size={16} />}
      </button>
    </div>
  );
};

export default Toolbar;
