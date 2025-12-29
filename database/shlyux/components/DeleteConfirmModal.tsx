import React, { useEffect } from 'react';
import { AlertTriangle, X } from 'lucide-react';
import { usePresence } from '../utils/usePresence';

interface DeleteConfirmModalProps {
  isOpen: boolean;
  fileName: string;
  onConfirm: () => void;
  onCancel: () => void;
}

const DeleteConfirmModal: React.FC<DeleteConfirmModalProps> = ({
  isOpen,
  fileName,
  onConfirm,
  onCancel,
}) => {
  const modalPresence = usePresence(isOpen, { exitDurationMs: 180 });

  useEffect(() => {
    if (!isOpen) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onCancel();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [isOpen, onCancel]);

  if (!modalPresence.isMounted) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center ui-overlay"
      data-state={modalPresence.state}
      style={{ background: 'rgba(0,0,0,0.45)' }}
      onClick={onCancel}
    >
      <div
        className="rounded-lg shadow-xl max-w-md w-full mx-4 ui-modal"
        data-state={modalPresence.state}
        onClick={(e) => e.stopPropagation()}
        style={{ background: 'var(--card-bg)', border: '1px solid var(--border-color)' }}
      >
        <div className="flex items-start justify-between p-6 border-b" style={{ borderColor: 'var(--border-color)' }}>
          <div className="flex items-center gap-3">
            <div
              className="w-12 h-12 rounded-full flex items-center justify-center"
              style={{ background: 'var(--sheet-danger-fill)', border: '1px solid var(--border-color)' }}
            >
              <AlertTriangle size={24} style={{ color: '#ef4444' }} />
            </div>
            <div>
              <h3 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
                Faylni o‘chirish
              </h3>
              <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
                Bu amalni bekor qilib bo‘lmaydi
              </p>
            </div>
          </div>
          <button
            onClick={onCancel}
            className="p-2 rounded hover:opacity-80 transition-opacity"
            style={{ color: 'var(--text-secondary)' }}
            title="Yopish"
          >
            <X size={20} />
          </button>
        </div>

        <div className="p-6">
          <p style={{ color: 'var(--text-primary)' }}>
            Haqiqatan ham <span className="font-semibold">"{fileName}"</span> nomli faylni o‘chirmoqchimisiz?
          </p>
          <p className="text-sm mt-2" style={{ color: 'var(--text-secondary)' }}>
            Bu fayl butunlay o‘chiriladi va uni qayta tiklab bo‘lmaydi.
          </p>
        </div>

        <div
          className="flex items-center justify-end gap-3 p-6 border-t rounded-b-lg"
          style={{ borderColor: 'var(--border-color)', background: 'var(--bg-light)' }}
        >
          <button
            onClick={onCancel}
            className="px-4 py-2 text-sm font-medium border rounded-md hover:opacity-80 transition-opacity"
            style={{ borderColor: 'var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
          >
            Bekor qilish
          </button>
          <button
            onClick={onConfirm}
            className="px-4 py-2 text-sm font-medium text-white rounded-md hover:opacity-90 transition-opacity"
            style={{ background: '#ef4444' }}
          >
            O‘chirish
          </button>
        </div>
      </div>
    </div>
  );
};

export default DeleteConfirmModal;
