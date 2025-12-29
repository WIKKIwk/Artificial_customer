import React, { useState } from 'react';
import { FileText, Save, FolderOpen, FilePlus, User, LogOut, Download, Terminal, Code2, Key, Copy, Check, Trash2, Edit3, Settings, LayoutTemplate, Share2 } from 'lucide-react';
import { generateApiKey } from '../utils/api';
import { usePresence } from '../utils/usePresence';

interface HeaderProps {
    fileName: string;
    currentFileId: number | null;
    currentAccessRole?: 'owner' | 'editor' | 'viewer' | null;
    user: any;
    files: any[];
    apiBase: string;
    authToken: string | null;
    onFileNameChange: (name: string) => void;
    onNewFile: () => void;
    onOpenTemplates?: () => void;
    onSaveFile: () => void;
    onShareFile?: () => void;
    onOpenFile: (id: number) => void;
    onDeleteFile?: (id: number) => void;
    onLogout: () => void;
    onExport?: () => void;
    onImportFile?: (file: File) => void;
    onShowProfile?: () => void;
}

const Header: React.FC<HeaderProps> = ({
    fileName,
    currentFileId,
    currentAccessRole,
    user,
    files,
    apiBase,
    authToken,
    onFileNameChange,
    onNewFile,
    onOpenTemplates,
    onSaveFile,
    onShareFile,
    onOpenFile,
    onDeleteFile,
    onLogout,
    onExport,
    onImportFile,
    onShowProfile
}) => {
    const [isEditing, setIsEditing] = useState(false);
    const [editValue, setEditValue] = useState(fileName);
    const [filesOpen, setFilesOpen] = useState(false);
    const [userMenuOpen, setUserMenuOpen] = useState(false);
    const [devOpen, setDevOpen] = useState(false);
    const [generatedSnippet, setGeneratedSnippet] = useState<string | null>(null);
    const [copyStatus, setCopyStatus] = useState<string | null>(null);
    const [apiKey, setApiKey] = useState<string | null>(null);
    const [apiKeyStatus, setApiKeyStatus] = useState<string | null>(null);

    const isOwner = (currentAccessRole || 'owner') === 'owner';
    const isViewer = currentAccessRole === 'viewer';

    const filesPresence = usePresence(filesOpen, { exitDurationMs: 180 });
    const devPresence = usePresence(devOpen, { exitDurationMs: 180 });
    const userMenuPresence = usePresence(userMenuOpen, { exitDurationMs: 180 });

    const handleSaveName = () => {
        onFileNameChange(editValue);
        setIsEditing(false);
    };

    const apiBaseV1 = `${apiBase}/api/v1`;
    const fileIdForSnippet = currentFileId ? `${currentFileId}` : ':id';
    const apiKeyHeader = apiKey ? `-H "X-API-Key: ${apiKey}"` : `-H "X-API-Key: YOUR_API_KEY"`;
    const placeholderSnippet = `curl ${apiKeyHeader} ${apiBaseV1}/files`;

    const setSnippet = (snippet: string) => {
        setGeneratedSnippet(snippet);
        setCopyStatus(null);
    };

    const handleGenerateApiSnippet = () => {
        setSnippet(placeholderSnippet);
    };

    const copyText = (text: string) => {
        navigator.clipboard?.writeText(text)
            .then(() => setCopyStatus("Nusxa olindi"))
            .catch(() => setCopyStatus("Nusxa olishda xato"));
    };

    const handleCopyPlaceholder = () => {
        if (!generatedSnippet) return;
        copyText(generatedSnippet);
    };

    const handleCopyWithApiKey = () => {
        if (!apiKey) {
            setCopyStatus("API key yo'q, avval yaratib oling");
            return;
        }
        copyText(`curl -H "X-API-Key: ${apiKey}" ${apiBaseV1}/files`);
    };

    const handleCopyApiKeyOnly = () => {
        if (!apiKey) {
            setCopyStatus("API key yo'q, avval yaratib oling");
            return;
        }
        copyText(apiKey);
    };

    const handleGenerateApiKeyClick = async () => {
        if (!authToken) {
            setApiKeyStatus("Token yo'q, login qiling");
            return;
        }
        setApiKeyStatus("Yaratilmoqda...");
        try {
            const res = await generateApiKey(authToken);
            setApiKey(res.api_key);
            setApiKeyStatus("Yaratildi");
            // refresh snippet to use new key
            setSnippet(`curl -H "X-API-Key: ${res.api_key}" ${apiBaseV1}/files`);
        } catch (err) {
            const msg = err instanceof Error ? err.message : 'API key yaratilmadi';
            setApiKeyStatus(msg);
        }
    };

    const handleSnippetListFiles = () => setSnippet(`curl ${apiKeyHeader} ${apiBaseV1}/files`);
    const handleSnippetGetFile = () => setSnippet(`curl ${apiKeyHeader} ${apiBaseV1}/files/${fileIdForSnippet}`);
    const handleSnippetSchema = () => setSnippet(`curl ${apiKeyHeader} ${apiBaseV1}/files/${fileIdForSnippet}/schema`);
    const handleSnippetRange = () => setSnippet(`curl ${apiKeyHeader} "${apiBaseV1}/files/${fileIdForSnippet}/cells?range=A1:D20&format=grid"`);
    const handleSnippetPatch = () => setSnippet(
        `curl -X PATCH ${apiKeyHeader} -H "Content-Type: application/json" -d '{"edits":[{"cell":"A2","value":"Hello"}]}' ${apiBaseV1}/files/${fileIdForSnippet}/cells`
    );
    const handleSnippetRealtimeToken = () => setSnippet(`curl -X POST ${apiKeyHeader} ${apiBaseV1}/realtime/token`);

    return (
        <div className="modern-header">
            <div className="header-content">
                {/* Logo & File Name */}
                <div className="header-left">
                    <div className="logo-container">
                        <FileText size={20} className="logo-icon" />
                        <span className="logo-text">W12C Sheets</span>
                    </div>

                    {user && (
                        <div className="file-name-container">
                            {isEditing ? (
                                <input
                                    value={editValue}
                                    onChange={(e) => setEditValue(e.target.value)}
                                    onBlur={handleSaveName}
                                    onKeyDown={(e) => {
                                        if (e.key === 'Enter') handleSaveName();
                                        if (e.key === 'Escape') {
                                            setEditValue(fileName);
                                            setIsEditing(false);
                                        }
                                    }}
                                    autoFocus
                                    className="file-name-input"
                                />
                            ) : (
                                <div
                                    className="file-name-display"
                                    onClick={() => {
                                        if (isOwner) setIsEditing(true);
                                    }}
                                    title={isOwner ? "Fayl nomini tahrirlash uchun bosing" : "Shared fayl (faqat owner rename qila oladi)"}
                                    style={{
                                        display: 'flex',
                                        alignItems: 'center',
                                        gap: '0.5rem',
                                        padding: '0.375rem 0.75rem',
                                        border: '1px solid var(--border-color)',
                                        borderRadius: '0.375rem',
                                        cursor: isOwner ? 'pointer' : 'default',
                                        transition: 'all 0.2s',
                                        background: 'rgba(0,0,0,0.02)'
                                    }}
                                    onMouseEnter={(e) => {
                                        if (!isOwner) return;
                                        e.currentTarget.style.borderColor = 'var(--primary)';
                                        e.currentTarget.style.background = 'rgba(0,0,0,0.04)';
                                    }}
                                    onMouseLeave={(e) => {
                                        e.currentTarget.style.borderColor = 'var(--border-color)';
                                        e.currentTarget.style.background = 'rgba(0,0,0,0.02)';
                                    }}
                                >
                                    <Edit3 size={14} style={{ color: 'var(--text-secondary)', flexShrink: 0 }} />
                                    <span>{fileName}</span>
                                    {!isOwner && currentAccessRole && (
                                        <span
                                            className="text-xs px-2 py-0.5 rounded"
                                            style={{ background: '#e0f2fe', color: '#0369a1' }}
                                        >
                                            {currentAccessRole}
                                        </span>
                                    )}
                                </div>
                            )}
                        </div>
                    )}
                </div>

                {/* Actions */}
                {user && (
                    <div className="header-actions">
                        <button onClick={onNewFile} className="action-btn" title="New File">
                            <FilePlus size={14} />
                            <span>New</span>
                        </button>

                        {onOpenTemplates && (
                            <button onClick={onOpenTemplates} className="action-btn" title="Templates (Ctrl+Shift+N)">
                                <LayoutTemplate size={14} />
                                <span>Templates</span>
                            </button>
                        )}

                        {onShareFile && currentFileId && isOwner && (
                            <button onClick={onShareFile} className="action-btn" title="Share">
                                <Share2 size={14} />
                                <span>Share</span>
                            </button>
                        )}

                        <button
                            onClick={onSaveFile}
                            disabled={isViewer}
                            className={`action-btn primary btn-lift ${isViewer ? 'opacity-50 cursor-not-allowed' : ''}`}
                            title={isViewer ? "Read-only: Save mumkin emas" : "Save"}
                        >
                            <Save size={14} />
                            <span>Save</span>
                        </button>

                        {onExport && (
                            <button onClick={onExport} className="action-btn" title="Export to CSV">
                                <Download size={14} />
                                <span>Export</span>
                            </button>
                        )}

                        {onImportFile && (
                            <label className="action-btn cursor-pointer" title="Import CSV/Excel">
                                <input
                                    id="file-input"
                                    type="file"
                                    accept=".csv,.xlsx,.xls"
                                    className="hidden"
                                    onChange={(e) => {
                                        const f = e.target.files?.[0];
                                        if (f) {
                                            onImportFile(f);
                                            e.target.value = '';
                                        }
                                    }}
                                />
                                <Download size={14} />
                                <span>Import</span>
                            </label>
                        )}

                        <div className="relative">
                            <button
                                onClick={() => setFilesOpen(!filesOpen)}
                                className="action-btn"
                                title="Open File"
                            >
                                <FolderOpen size={14} />
                                <span>Open</span>
                            </button>

                            {filesPresence.isMounted && (
                                <>
                                    <div
                                        className="dropdown-overlay ui-overlay"
                                        data-state={filesPresence.state}
                                        onClick={() => setFilesOpen(false)}
                                    />
                                    <div className="dropdown files-dropdown ui-popover" data-state={filesPresence.state}>
                                        <div className="dropdown-header">Recent Files</div>
                                        <div className="files-list">
                                            {files.length === 0 ? (
                                                <div className="empty-files">No files yet</div>
                                            ) : (
                                                files.map((file, index) => {
                                                    const role = (file as any).access_role || 'owner';
                                                    const canDelete = role === 'owner';
                                                    return (
                                                        <div
                                                            key={file.id}
                                                            className={`file-item-wrapper ${currentFileId === file.id ? 'active' : ''}`}
                                                            style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}
                                                        >
                                                            <button
                                                                onClick={() => {
                                                                    onOpenFile(file.id);
                                                                    setFilesOpen(false);
                                                                }}
                                                                className={`file-item ${currentFileId === file.id ? 'active' : ''}`}
                                                                style={{ flex: 1 }}
                                                            >
                                                                <span style={{
                                                                    minWidth: '20px',
                                                                    fontWeight: '500',
                                                                    color: '#6b7280'
                                                                }}>
                                                                    {index + 1}.
                                                                </span>
                                                                <FileText size={14} />
                                                                <span>{file.name}</span>
                                                                {role !== 'owner' && (
                                                                    <span
                                                                        className="text-xs px-2 py-0.5 rounded ml-2"
                                                                        style={{ background: '#e0f2fe', color: '#0369a1' }}
                                                                    >
                                                                        {role}
                                                                    </span>
                                                                )}
                                                            </button>
                                                            {onDeleteFile && canDelete && (
                                                                <button
                                                                    onClick={(e) => {
                                                                        e.stopPropagation();
                                                                        onDeleteFile(file.id);
                                                                    }}
                                                                    className="file-delete-btn"
                                                                    title="Delete file"
                                                                    style={{
                                                                        padding: '0.5rem',
                                                                        background: 'transparent',
                                                                        border: 'none',
                                                                        cursor: 'pointer',
                                                                        borderRadius: '0.25rem',
                                                                        display: 'flex',
                                                                        alignItems: 'center',
                                                                        color: '#ef4444',
                                                                        transition: 'all 0.2s'
                                                                    }}
                                                                    onMouseEnter={(e) => {
                                                                        e.currentTarget.style.background = '#fef2f2';
                                                                    }}
                                                                    onMouseLeave={(e) => {
                                                                        e.currentTarget.style.background = 'transparent';
                                                                    }}
                                                                >
                                                                    <Trash2 size={14} />
                                                                </button>
                                                            )}
                                                        </div>
                                                    );
                                                })
                                            )}
                                        </div>
                                    </div>
                                </>
                            )}
                        </div>

                        <div className="relative">
                            <button
                                onClick={() => setDevOpen(!devOpen)}
                                className="action-btn"
                                title="Developer API"
                            >
                                <Terminal size={14} />
                                <span>Dev</span>
                            </button>

                            {devPresence.isMounted && (
                                <>
                                    <div
                                        className="dropdown-overlay ui-overlay"
                                        data-state={devPresence.state}
                                        onClick={() => setDevOpen(false)}
                                    />
                                    <div
                                        className="dropdown dev-dropdown ui-popover"
                                        data-state={devPresence.state}
                                        style={{ minWidth: 420, maxWidth: 520 }}
                                    >
                                        <div className="dev-dropdown-header">
                                            <Code2 size={20} />
                                            <span>API olish</span>
                                        </div>

                                        <div className="dev-section">
                                            <div className="dev-info-row">
                                                <span className="dev-label">Base URL:</span>
                                                <code className="dev-value">{apiBaseV1}</code>
                                            </div>
                                            <div className="dev-info-row">
                                                <span className="dev-label">File ID:</span>
                                                <code className="dev-value">{currentFileId ?? '—'}</code>
                                            </div>
                                            <div className="dev-info-row">
                                                <span className="dev-label">Auth:</span>
                                                <span className="dev-value">Bearer JWT (login/register orqali)</span>
                                            </div>
                                        </div>

                                        <div className="dev-divider" />

                                        <div className="dev-section">
                                            <div className="dev-subsection-header">
                                                <Key size={16} />
                                                <span>API Key</span>
                                            </div>

                                            <div className="api-key-display">
                                                {apiKey ? (
                                                    <>
                                                        <code className="api-key-text">
                                                            {apiKey.slice(0, 8)}...{apiKey.slice(-8)}
                                                        </code>
                                                        <button
                                                            className="icon-copy-btn"
                                                            onClick={handleCopyApiKeyOnly}
                                                            title="Copy API Key"
                                                        >
                                                            {copyStatus === "Nusxa olindi" ? <Check size={14} /> : <Copy size={14} />}
                                                        </button>
                                                    </>
                                                ) : (
                                                    <span className="dev-placeholder">Yaratilmagan</span>
                                                )}
                                            </div>

                                            <button
                                                className="dev-action-btn"
                                                onClick={handleGenerateApiKeyClick}
                                            >
                                                <Key size={16} />
                                                API key yaratish
                                            </button>

                                            {apiKeyStatus && (
                                                <div className="dev-status">{apiKeyStatus}</div>
                                            )}
                                        </div>

                                        <div className="dev-divider" />

                                        <div className="dev-section">
                                            <div className="snippet-actions" style={{ flexWrap: 'wrap' }}>
                                                <button className="snippet-action-btn" onClick={handleSnippetListFiles}>
                                                    <Terminal size={14} />
                                                    files
                                                </button>
                                                <button className="snippet-action-btn" onClick={handleSnippetGetFile} disabled={!currentFileId}>
                                                    <Terminal size={14} />
                                                    file
                                                </button>
                                                <button className="snippet-action-btn" onClick={handleSnippetSchema} disabled={!currentFileId}>
                                                    <Terminal size={14} />
                                                    schema
                                                </button>
                                                <button className="snippet-action-btn" onClick={handleSnippetRange} disabled={!currentFileId}>
                                                    <Terminal size={14} />
                                                    range
                                                </button>
                                                <button className="snippet-action-btn" onClick={handleSnippetPatch} disabled={!currentFileId || isViewer}>
                                                    <Terminal size={14} />
                                                    patch
                                                </button>
                                                <button className="snippet-action-btn" onClick={handleSnippetRealtimeToken}>
                                                    <Terminal size={14} />
                                                    realtime
                                                </button>
                                            </div>

                                            <button
                                                className="dev-action-btn primary"
                                                onClick={handleGenerateApiSnippet}
                                            >
                                                <Code2 size={16} />
                                                cURL yaratish
                                            </button>

                                            {generatedSnippet && (
                                                <div className="code-snippet-container">
                                                    <div className="code-snippet-header">
                                                        <Terminal size={14} />
                                                        <span>cURL Command</span>
                                                    </div>
                                                    <div className="code-snippet">
                                                        <code>{generatedSnippet}</code>
                                                        <button
                                                            className="snippet-copy-btn"
                                                            onClick={handleCopyPlaceholder}
                                                            title="Copy to clipboard"
                                                        >
                                                            {copyStatus === "Nusxa olindi" ? <Check size={14} /> : <Copy size={14} />}
                                                        </button>
                                                    </div>

                                                    <div className="snippet-actions">
                                                        <button
                                                            className="snippet-action-btn"
                                                            onClick={handleCopyWithApiKey}
                                                        >
                                                            <Copy size={14} />
                                                            Nusxa (mening API key)
                                                        </button>
                                                    </div>

                                                    {copyStatus && (
                                                        <div className="copy-feedback">
                                                            <Check size={14} />
                                                            {copyStatus}
                                                        </div>
                                                    )}
                                                </div>
                                            )}
                                        </div>
                                    </div>
                                </>
                            )}
                        </div>
                    </div>
                )}

                {/* Right Side */}
                <div className="header-right">
                    {user && (
                        <div className="relative">
                            <button
                                onClick={() => setUserMenuOpen(!userMenuOpen)}
                                className="user-btn"
                                title="User Menu"
                            >
                                <div className="user-avatar">
                                    {user.name?.charAt(0).toUpperCase() || 'U'}
                                </div>
                                <span className="user-name">{user.name}</span>
                            </button>

                            {userMenuPresence.isMounted && (
                                <>
                                    <div
                                        className="dropdown-overlay ui-overlay"
                                        data-state={userMenuPresence.state}
                                        onClick={() => setUserMenuOpen(false)}
                                    />
                                    <div className="dropdown user-dropdown ui-popover" data-state={userMenuPresence.state}>
                                        <div className="user-info">
                                            <div className="user-avatar large">
                                                {user.name?.charAt(0).toUpperCase() || 'U'}
                                            </div>
                                            <div>
                                                <div className="user-info-name">{user.name}</div>
                                                <div className="user-info-email">{user.email}</div>
                                            </div>
                                        </div>
                                        <div className="dropdown-divider" />
                                        {onShowProfile && (
                                            <button
                                                onClick={() => {
                                                    onShowProfile();
                                                    setUserMenuOpen(false);
                                                }}
                                                className="dropdown-item"
                                            >
                                                <Settings size={16} />
                                                <span>Profile</span>
                                            </button>
                                        )}
                                        <button
                                            onClick={() => {
                                                onLogout();
                                                setUserMenuOpen(false);
                                            }}
                                            className="dropdown-item danger"
                                        >
                                            <LogOut size={16} />
                                            <span>Logout</span>
                                        </button>
                                    </div>
                                </>
                            )}
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
};

export default Header;
