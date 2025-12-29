import React, { useEffect, useRef } from 'react';
import { Copy, Scissors, Clipboard, Trash2, Plus, Minus, ArrowDownAZ, ArrowUpAZ } from 'lucide-react';
import { usePresence } from '../utils/usePresence';

interface ContextMenuProps {
    show: boolean;
    x: number;
    y: number;
    onCut: () => void;
    onCopy: () => void;
    onPaste: () => void;
    onDelete: () => void;
    onInsertRowAbove: () => void;
    onInsertRowBelow: () => void;
    onInsertColLeft: () => void;
    onInsertColRight: () => void;
    onDeleteRow: () => void;
    onDeleteCol: () => void;
    onSortAsc: () => void;
    onSortDesc: () => void;
    onClose: () => void;
}

const ContextMenu: React.FC<ContextMenuProps> = ({
    show,
    x,
    y,
    onCut,
    onCopy,
    onPaste,
    onDelete,
    onInsertRowAbove,
    onInsertRowBelow,
    onInsertColLeft,
    onInsertColRight,
    onDeleteRow,
    onDeleteCol,
    onSortAsc,
    onSortDesc,
    onClose
}) => {
    const menuRef = useRef<HTMLDivElement>(null);
    const menuPresence = usePresence(show, { exitDurationMs: 180 });
    const lastPosRef = useRef({ x, y });

    useEffect(() => {
        if (!show) return;
        lastPosRef.current = { x, y };
    }, [show, x, y]);

    useEffect(() => {
        const handleClickOutside = (e: MouseEvent) => {
            if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
                onClose();
            }
        };

        const handleEscape = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                onClose();
            }
        };

        if (show) {
            document.addEventListener('mousedown', handleClickOutside);
            document.addEventListener('keydown', handleEscape);
        }

        return () => {
            document.removeEventListener('mousedown', handleClickOutside);
            document.removeEventListener('keydown', handleEscape);
        };
    }, [show, onClose]);

    if (!menuPresence.isMounted) return null;

    const pos = show ? { x, y } : lastPosRef.current;

    const menuItems = [
        { icon: Scissors, label: 'Kesish', shortcut: 'Ctrl+X', action: onCut },
        { icon: Copy, label: 'Nusxalash', shortcut: 'Ctrl+C', action: onCopy },
        { icon: Clipboard, label: "Qo'yish", shortcut: 'Ctrl+V', action: onPaste },
        { divider: true },
        { icon: Trash2, label: "Tarkibni o'chirish", action: onDelete },
        { divider: true },
        { icon: ArrowDownAZ, label: "Sort A → Z", action: onSortAsc },
        { icon: ArrowUpAZ, label: "Sort Z → A", action: onSortDesc },
        { divider: true },
        { icon: Plus, label: "Qatorni yuqoriga qo'shish", action: onInsertRowAbove },
        { icon: Plus, label: "Qatorni pastga qo'shish", action: onInsertRowBelow },
        { icon: Plus, label: "Ustunni chapga qo'shish", action: onInsertColLeft },
        { icon: Plus, label: "Ustunni o'ngga qo'shish", action: onInsertColRight },
        { divider: true },
        { icon: Minus, label: "Qatorni o'chirish", action: onDeleteRow, danger: true },
        { icon: Minus, label: "Ustunni o'chirish", action: onDeleteCol, danger: true },
    ];

    return (
        <div
            ref={menuRef}
            className="fixed z-50 rounded shadow-lg py-1 min-w-[220px] ui-popover"
            data-state={menuPresence.state}
            style={{
                left: pos.x,
                top: pos.y,
                background: 'var(--card-bg)',
                border: '1px solid var(--border-color)',
                transformOrigin: 'top left'
            }}
        >
            {menuItems.map((item, idx) => {
                if (item.divider) {
                    return <div key={idx} className="h-px my-1" style={{ background: 'var(--border-color)' }} />;
                }

                const Icon = item.icon!;
                return (
                    <button
                        key={idx}
                        onClick={() => {
                            item.action!();
                            onClose();
                        }}
                        className={`w-full flex items-center gap-3 px-4 py-2 text-sm transition-colors ${item.danger ? 'text-red-600 hover:bg-red-50' : ''
                            }`}
                        style={!item.danger ? {
                            color: 'var(--text-primary)',
                            background: 'transparent'
                        } : undefined}
                        onMouseEnter={(e) => {
                            if (!item.danger) {
                                (e.currentTarget as HTMLElement).style.background = 'var(--bg-light)';
                            }
                        }}
                        onMouseLeave={(e) => {
                            if (!item.danger) {
                                (e.currentTarget as HTMLElement).style.background = 'transparent';
                            }
                        }}
                    >
                        <Icon size={14} />
                        <span className="flex-1 text-left">{item.label}</span>
                        {item.shortcut && (
                            <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>{item.shortcut}</span>
                        )}
                    </button>
                );
            })}
        </div>
    );
};

export default ContextMenu;
