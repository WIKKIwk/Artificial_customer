import React, { useEffect } from 'react';
import { X, AlertCircle } from 'lucide-react';

interface ErrorToastProps {
  message: string | null;
  onClose: () => void;
  duration?: number;
}

const ErrorToast: React.FC<ErrorToastProps> = ({ message, onClose, duration = 5000 }) => {
  useEffect(() => {
    if (message) {
      const timer = setTimeout(() => {
        onClose();
      }, duration);

      return () => clearTimeout(timer);
    }
  }, [message, duration, onClose]);

  if (!message) return null;

  return (
    <div
      className="fixed top-4 right-4 z-50 max-w-md shadow-lg rounded-lg p-4 flex items-start space-x-3 animate-slide-in"
      style={{
        background: '#FEE2E2',
        border: '1px solid #EF4444',
        animation: 'slideIn 0.3s ease-out'
      }}
    >
      <AlertCircle size={20} style={{ color: '#DC2626', flexShrink: 0, marginTop: '2px' }} />
      <div className="flex-1">
        <p className="text-sm font-medium" style={{ color: '#7F1D1D' }}>Xato</p>
        <p className="text-sm mt-1" style={{ color: '#991B1B' }}>{message}</p>
      </div>
      <button
        onClick={onClose}
        className="p-1 rounded hover:bg-red-200 transition-colors"
        style={{ color: '#DC2626' }}
      >
        <X size={16} />
      </button>

      <style>{`
        @keyframes slideIn {
          from {
            transform: translateX(100%);
            opacity: 0;
          }
          to {
            transform: translateX(0);
            opacity: 1;
          }
        }
      `}</style>
    </div>
  );
};

export default ErrorToast;
