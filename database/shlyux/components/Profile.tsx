import React, { useState, useEffect } from 'react';
import { X } from 'lucide-react';

interface ProfileProps {
    isOpen: boolean;
    onClose: () => void;
    onSaved?: (prefs: { theme: string; font: string; density: 'comfortable' | 'compact' }) => void;
}

const Profile: React.FC<ProfileProps> = ({ isOpen, onClose, onSaved }) => {
    const [selectedTheme, setSelectedTheme] = useState(() => {
        return localStorage.getItem('app-theme') || 'light';
    });

    const [selectedFont, setSelectedFont] = useState(() => {
        return localStorage.getItem('app-font') || 'Inter, sans-serif';
    });

    const [selectedDensity, setSelectedDensity] = useState<'comfortable' | 'compact'>(() => {
        return localStorage.getItem('app-density') === 'compact' ? 'compact' : 'comfortable';
    });

    const handleSave = () => {
        // Save to localStorage
        localStorage.setItem('app-theme', selectedTheme);
        localStorage.setItem('app-font', selectedFont);
        localStorage.setItem('app-density', selectedDensity);

        // Apply theme
        document.documentElement.setAttribute('data-theme', selectedTheme);

        // Apply density
        document.documentElement.setAttribute('data-density', selectedDensity);

        // Apply font
        document.documentElement.style.setProperty('--font-family', selectedFont);

        onSaved?.({ theme: selectedTheme, font: selectedFont, density: selectedDensity });

        // Close modal
        onClose();
    };

    if (!isOpen) return null;

    return (
        <div className="profile-overlay" onClick={onClose}>
            <div className="profile-modal" onClick={(e) => e.stopPropagation()}>
                <div className="profile-header">
                    <h1>Profile Settings</h1>
                    <button onClick={onClose} className="profile-close">
                        <X size={18} />
                    </button>
                </div>

                <div className="profile-content">
                    <h2 className="profile-title">Settings</h2>
                    <p className="profile-subtitle">Customize your app appearance</p>

                    {/* Theme Selector */}
                    <div className="profile-section">
                        <label className="profile-label">Theme</label>
                        <div className="theme-options">
                            <button
                                className={`theme-option ${selectedTheme === 'light' ? 'active' : ''}`}
                                onClick={() => setSelectedTheme('light')}
                            >
                                <div className="theme-preview light">
                                    <div className="theme-preview-header"></div>
                                    <div className="theme-preview-body"></div>
                                </div>
                                <span>Light</span>
                            </button>
                            <button
                                className={`theme-option ${selectedTheme === 'dark' ? 'active' : ''}`}
                                onClick={() => setSelectedTheme('dark')}
                            >
                                <div className="theme-preview dark">
                                    <div className="theme-preview-header"></div>
                                    <div className="theme-preview-body"></div>
                                </div>
                                <span>Dark</span>
                            </button>
                        </div>
                    </div>

                    {/* Font Selector */}
                    <div className="profile-section">
                        <label className="profile-label" htmlFor="font-select">Font Family</label>
                        <select
                            id="font-select"
                            value={selectedFont}
                            onChange={(e) => setSelectedFont(e.target.value)}
                            className="font-select"
                        >
                            <option value="Inter, sans-serif">Inter</option>
                            <option value="Roboto, sans-serif">Roboto</option>
                            <option value="'Open Sans', sans-serif">Open Sans</option>
                            <option value="Lato, sans-serif">Lato</option>
                            <option value="Arial, sans-serif">Arial</option>
                            <option value="'Segoe UI', sans-serif">Segoe UI</option>
                        </select>
                        <p className="font-preview" style={{ fontFamily: selectedFont }}>
                            The quick brown fox jumps over the lazy dog
                        </p>
                    </div>

                    {/* Density Selector */}
                    <div className="profile-section">
                        <label className="profile-label" htmlFor="density-select">Density</label>
                        <select
                            id="density-select"
                            value={selectedDensity}
                            onChange={(e) => setSelectedDensity(e.target.value as 'comfortable' | 'compact')}
                            className="font-select"
                        >
                            <option value="comfortable">Comfortable</option>
                            <option value="compact">Compact</option>
                        </select>
                        <p className="profile-subtitle" style={{ marginTop: '0.5rem' }}>
                            Jadval va toolbar zichligini sozlash
                        </p>
                    </div>

                    {/* Action Buttons */}
                    <div className="profile-actions">
                        <button onClick={onClose} className="profile-btn cancel">
                            Cancel
                        </button>
                        <button onClick={handleSave} className="profile-btn save">
                            Save Changes
                        </button>
                    </div>
                </div>

                <style>{`
                    .profile-overlay {
                        position: fixed;
                        inset: 0;
                        background: rgba(0, 0, 0, 0.5);
                        display: flex;
                        align-items: center;
                        justify-content: center;
                        z-index: 1000;
                        animation: fadeIn 0.2s ease-out;
                    }

                    @keyframes fadeIn {
                        from { opacity: 0; }
                        to { opacity: 1; }
                    }

                    @keyframes ripple {
                        0% {
                            transform: scale(0);
                            opacity: 0.6;
                        }
                        100% {
                            transform: scale(2.5);
                            opacity: 0;
                        }
                    }

                    @keyframes bouncePress {
                        0% { transform: perspective(500px) rotateX(0deg) translateZ(0); }
                        50% { transform: perspective(500px) rotateX(-5deg) translateZ(-10px); }
                        100% { transform: perspective(500px) rotateX(0deg) translateZ(0); }
                    }

                    @keyframes glowPulse {
                        0%, 100% { box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.2), 0 4px 12px rgba(102, 126, 234, 0.25); }
                        50% { box-shadow: 0 0 0 5px rgba(102, 126, 234, 0.3), 0 6px 20px rgba(102, 126, 234, 0.4); }
                    }

                    .profile-modal {
                        background: var(--card-bg, #ffffff);
                        border-radius: 12px;
                        width: 90%;
                        max-width: 500px;
                        box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
                        animation: slideUp 0.3s ease-out;
                        overflow: hidden;
                    }

                    @keyframes slideUp {
                        from { transform: translateY(20px); opacity: 0; }
                        to { transform: translateY(0); opacity: 1; }
                    }

                    .profile-header {
                        display: flex;
                        align-items: center;
                        justify-content: space-between;
                        padding: 1.5rem;
                        border-bottom: 1px solid var(--border-color, #e5e7eb);
                    }

                    .profile-header h1 {
                        font-size: 1.25rem;
                        font-weight: 600;
                        color: var(--text-primary, #111827);
                        margin: 0;
                    }

                    .profile-close {
                        background: transparent;
                        border: none;
                        cursor: pointer;
                        padding: 0.5rem;
                        border-radius: 6px;
                        color: var(--text-secondary, #6b7280);
                        transition: all 0.2s;
                        display: flex;
                        align-items: center;
                        justify-content: center;
                    }

                    .profile-close:hover {
                        background: var(--bg-light, #f3f4f6);
                        color: var(--text-primary, #111827);
                    }

                    .profile-content {
                        padding: 1.5rem;
                    }

                    .profile-title {
                        font-size: 1.125rem;
                        font-weight: 600;
                        color: var(--text-primary, #111827);
                        margin: 0 0 0.5rem 0;
                    }

                    .profile-subtitle {
                        font-size: 0.875rem;
                        color: var(--text-secondary, #6b7280);
                        margin: 0 0 1.5rem 0;
                    }

                    .profile-section {
                        margin-bottom: 1.5rem;
                    }

                    .profile-label {
                        display: block;
                        font-size: 0.875rem;
                        font-weight: 500;
                        color: var(--text-primary, #111827);
                        margin-bottom: 0.75rem;
                    }

                    .theme-options {
                        display: grid;
                        grid-template-columns: 1fr 1fr;
                        gap: 1rem;
                    }

                    .theme-option {
                        position: relative;
                        background: transparent;
                        border: 2px solid var(--border-color, #e5e7eb);
                        border-radius: 8px;
                        padding: 1rem;
                        cursor: pointer;
                        transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
                        display: flex;
                        flex-direction: column;
                        align-items: center;
                        gap: 0.75rem;
                        overflow: hidden;
                        transform-style: preserve-3d;
                    }

                    .theme-option::before {
                        content: '';
                        position: absolute;
                        top: 50%;
                        left: 50%;
                        width: 0;
                        height: 0;
                        border-radius: 50%;
                        background: radial-gradient(circle, rgba(102, 126, 234, 0.3), transparent);
                        transform: translate(-50%, -50%);
                        transition: width 0.6s, height 0.6s;
                    }

                    .theme-option:hover {
                        border-color: var(--primary, #667eea);
                        transform: perspective(500px) translateY(-4px) rotateX(2deg);
                        box-shadow: 0 8px 20px rgba(102, 126, 234, 0.2);
                    }

                    .theme-option.active {
                        border-color: var(--primary, #667eea);
                        background: rgba(102, 126, 234, 0.08);
                        border-width: 3px;
                        animation: glowPulse 2s ease-in-out infinite;
                    }

                    .theme-option:active::before {
                        width: 300%;
                        height: 300%;
                        transition: width 0s, height 0s;
                    }

                    .theme-option:active {
                        animation: bouncePress 0.4s ease-out;
                    }

                    .theme-option span {
                        font-size: 0.875rem;
                        font-weight: 500;
                        color: var(--text-primary, #111827);
                        position: relative;
                        z-index: 1;
                    }

                    .theme-preview {
                        width: 100%;
                        height: 80px;
                        border-radius: 6px;
                        overflow: hidden;
                        border: 1px solid #ddd;
                        position: relative;
                        z-index: 1;
                        transition: transform 0.3s ease;
                    }

                    .theme-option:hover .theme-preview {
                        transform: scale(1.02);
                    }

                    .theme-preview.light {
                        background: #ffffff;
                    }

                    .theme-preview.dark {
                        background: #1a1a1a;
                    }

                    .theme-preview-header {
                        height: 20px;
                    }

                    .theme-preview.light .theme-preview-header {
                        background: #f3f4f6;
                    }

                    .theme-preview.dark .theme-preview-header {
                        background: #2d2d2d;
                    }

                    .theme-preview-body {
                        height: 60px;
                        padding: 0.5rem;
                        display: flex;
                        flex-direction: column;
                        gap: 0.25rem;
                    }

                    .theme-preview.light .theme-preview-body::before,
                    .theme-preview.light .theme-preview-body::after {
                        content: '';
                        height: 6px;
                        background: #d1d5db;
                        border-radius: 2px;
                    }

                    .theme-preview.dark .theme-preview-body::before,
                    .theme-preview.dark .theme-preview-body::after {
                        content: '';
                        height: 6px;
                        background: #4b5563;
                        border-radius: 2px;
                    }

                    .font-select {
                        width: 100%;
                        padding: 0.75rem;
                        border: 1px solid var(--border-color, #e5e7eb);
                        border-radius: 6px;
                        background: var(--card-bg, #ffffff);
                        color: var(--text-primary, #111827);
                        font-size: 0.875rem;
                        cursor: pointer;
                        transition: all 0.2s;
                    }

                    .font-select:focus {
                        outline: none;
                        border-color: var(--primary, #667eea);
                        box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
                    }

                    .font-preview {
                        margin-top: 0.75rem;
                        padding: 0.75rem;
                        background: var(--bg-light, #f9fafb);
                        border-radius: 6px;
                        font-size: 0.875rem;
                        color: var(--text-primary, #111827);
                    }

                    .profile-actions {
                        display: flex;
                        gap: 0.75rem;
                        margin-top: 2rem;
                    }

                    .profile-btn {
                        position: relative;
                        flex: 1;
                        padding: 0.75rem 1.5rem;
                        border: none;
                        border-radius: 6px;
                        font-size: 0.875rem;
                        font-weight: 500;
                        cursor: pointer;
                        transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
                        overflow: hidden;
                        transform-style: preserve-3d;
                    }

                    .profile-btn::after {
                        content: '';
                        position: absolute;
                        top: 50%;
                        left: 50%;
                        width: 0;
                        height: 0;
                        border-radius: 50%;
                        background: rgba(255, 255, 255, 0.5);
                        transform: translate(-50%, -50%);
                        transition: width 0.6s, height 0.6s;
                    }

                    .profile-btn:active::after {
                        width: 300%;
                        height: 300%;
                        transition: width 0s, height 0s;
                    }

                    .profile-btn.cancel {
                        background: var(--bg-light, #f3f4f6);
                        color: var(--text-primary, #111827);
                        border: 1px solid var(--border-color, #e5e7eb);
                    }

                    .profile-btn.cancel:hover {
                        background: #e5e7eb;
                        border-color: #d1d5db;
                        transform: perspective(500px) translateY(-2px);
                        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
                    }

                    .profile-btn.cancel:active {
                        animation: bouncePress 0.3s ease-out;
                    }

                    .profile-btn.save {
                        background: linear-gradient(135deg, var(--primary, #667eea) 0%, #764ba2 100%);
                        color: white;
                        box-shadow: 0 4px 15px rgba(102, 126, 234, 0.3);
                        background-size: 200% 200%;
                        background-position: left center;
                        transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
                    }

                    .profile-btn.save:hover {
                        background-position: right center;
                        box-shadow: 0 6px 20px rgba(102, 126, 234, 0.5);
                        transform: perspective(500px) translateY(-3px) rotateX(2deg);
                    }

                    .profile-btn.save:active {
                        animation: bouncePress 0.3s ease-out;
                        box-shadow: 0 2px 8px rgba(102, 126, 234, 0.4);
                    }
                `}</style>
            </div>
        </div>
    );
};

export default Profile;
