import React, { useMemo, useState } from 'react';
import { FileText, LayoutTemplate, X } from 'lucide-react';
import { usePresence } from '../utils/usePresence';

export type SheetTemplate = {
  id: string;
  title: string;
  description: string;
  fileName: string;
  rows: string[][];
};

interface TemplatePickerModalProps {
  isOpen: boolean;
  hasExistingData: boolean;
  onApply: (rows: string[][], fileName: string) => void;
  onClose: () => void;
}

const TemplatePickerModal: React.FC<TemplatePickerModalProps> = ({ isOpen, hasExistingData, onApply, onClose }) => {
  const templates = useMemo<SheetTemplate[]>(() => ([
    {
      id: 'blank',
      title: 'Bo‘sh jadval',
      description: 'Toza (blank) fayl',
      fileName: 'Yangi fayl',
      rows: []
    },
    {
      id: 'budget',
      title: 'Oylik Budjet',
      description: 'Daromad/xarajat reja va amalga oshgan',
      fileName: 'Oylik budjet',
      rows: [
        ['Kategoriya', 'Reja', 'Amal', 'Farq'],
        ['Ish haqi', '0', '0', '=B2-C2'],
        ['Ijara', '0', '0', '=B3-C3'],
        ['Kommunal', '0', '0', '=B4-C4'],
        ['Oziq-ovqat', '0', '0', '=B5-C5'],
        ['Transport', '0', '0', '=B6-C6'],
        ['Boshqa', '0', '0', '=B7-C7'],
        ['Jami', '=SUM(B2:B7)', '=SUM(C2:C7)', '=B8-C8'],
      ]
    },
    {
      id: 'invoice',
      title: 'Invoice',
      description: 'Hisob-faktura shabloni (qty × price)',
      fileName: 'Invoice',
      rows: [
        ['Invoice', '#INV-001', '', ''],
        ['Sana', '2025-01-01', '', ''],
        ['Mijoz', '', '', ''],
        ['', '', '', ''],
        ['Item', 'Qty', 'Unit Price', 'Amount'],
        ['Xizmat 1', '1', '0', '=B6*C6'],
        ['Xizmat 2', '1', '0', '=B7*C7'],
        ['Xizmat 3', '1', '0', '=B8*C8'],
        ['', '', 'Jami', '=SUM(D6:D8)'],
      ]
    },
    {
      id: 'crm',
      title: 'CRM Ro‘yxat',
      description: 'Kontaktlar va statuslar',
      fileName: 'CRM ro‘yxat',
      rows: [
        ['Ism', 'Telefon', 'Email', 'Status', 'Izoh'],
        ['Aziz', '+998 90 000 00 00', 'aziz@example.com', 'Yangi', ''],
        ['Madina', '+998 91 111 11 11', 'madina@example.com', 'Kontakt qilindi', ''],
        ['Javohir', '+998 93 222 22 22', 'javohir@example.com', 'Kelishuv', ''],
      ]
    }
  ]), []);

  const [pending, setPending] = useState<SheetTemplate | null>(null);
  const modalPresence = usePresence(isOpen, { exitDurationMs: 180 });

  if (!modalPresence.isMounted) return null;

  const applyTemplate = (tpl: SheetTemplate) => {
    if (hasExistingData && tpl.id !== 'blank') {
      setPending(tpl);
      return;
    }
    onApply(tpl.rows, tpl.fileName);
    onClose();
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center ui-overlay"
      data-state={modalPresence.state}
      style={{ background: 'rgba(0,0,0,0.45)' }}
    >
      <div
        className="w-full max-w-3xl rounded-lg shadow-xl overflow-hidden ui-modal"
        data-state={modalPresence.state}
        style={{ background: 'var(--card-bg)', border: '1px solid var(--border-color)' }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-5 py-4 flex items-center justify-between border-b" style={{ borderColor: 'var(--border-color)' }}>
          <div className="flex items-center gap-2">
            <LayoutTemplate size={18} style={{ color: 'var(--text-secondary)' }} />
            <h3 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>
              Template tanlang
            </h3>
          </div>
          <button
            type="button"
            onClick={() => {
              setPending(null);
              onClose();
            }}
            className="p-2 rounded hover:opacity-80 transition-opacity"
            style={{ color: 'var(--text-secondary)' }}
            title="Yopish"
          >
            <X size={18} />
          </button>
        </div>

        <div className="p-5">
          {pending ? (
            <div className="rounded-lg p-4" style={{ background: 'var(--bg-light)', border: '1px solid var(--border-color)' }}>
              <p className="text-sm" style={{ color: 'var(--text-primary)' }}>
                Joriy jadvaldagi ma’lumotlar yo‘qolishi mumkin.
              </p>
              <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>
                <span className="font-medium" style={{ color: 'var(--text-primary)' }}>{pending.title}</span> template’ni qo‘llaysizmi?
              </p>
              <div className="mt-4 flex justify-end gap-2">
                <button
                  type="button"
                  className="px-4 py-2 rounded text-sm"
                  style={{ background: 'var(--card-bg)', border: '1px solid var(--border-color)', color: 'var(--text-primary)' }}
                  onClick={() => setPending(null)}
                >
                  Bekor qilish
                </button>
                <button
                  type="button"
                  className="px-4 py-2 rounded text-sm text-white"
                  style={{ background: 'var(--primary)' }}
                  onClick={() => {
                    onApply(pending.rows, pending.fileName);
                    setPending(null);
                    onClose();
                  }}
                >
                  Ha, qo‘llash
                </button>
              </div>
            </div>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              {templates.map((tpl) => (
                <button
                  key={tpl.id}
                  type="button"
                  onClick={() => applyTemplate(tpl)}
                  className="text-left rounded-lg p-4 transition-all hover:shadow-sm"
                  style={{ border: '1px solid var(--border-color)', background: 'var(--card-bg)' }}
                >
                  <div className="flex items-start gap-3">
                    <div className="w-10 h-10 rounded flex items-center justify-center" style={{ background: 'var(--bg-light)', border: '1px solid var(--border-color)' }}>
                      <FileText size={18} style={{ color: 'var(--text-secondary)' }} />
                    </div>
                    <div className="min-w-0">
                      <div className="font-semibold" style={{ color: 'var(--text-primary)' }}>{tpl.title}</div>
                      <div className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>{tpl.description}</div>
                      <div className="text-xs mt-2" style={{ color: 'var(--text-secondary)' }}>Fayl nomi: {tpl.fileName}</div>
                    </div>
                  </div>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default TemplatePickerModal;
