const envApiBase = (import.meta.env.VITE_API_BASE || '').trim();
const runtimeApiBase = typeof window !== 'undefined'
  ? window.location.origin
  : 'http://localhost:8080';
export const API_BASE = envApiBase !== '' ? envApiBase : runtimeApiBase;

export interface AuthUser {
  id: number;
  name: string;
  email: string;
}

export interface AuthResult {
  token: string;
  user: AuthUser;
}

const jsonHeaders = {
  'Content-Type': 'application/json',
};

async function handleJsonResponse<T>(res: Response): Promise<T> {
  const text = await res.text();
  let data: any = null;

  try {
    data = text ? JSON.parse(text) : null;
  } catch (err) {
    throw new Error('Invalid JSON response from server');
  }

  if (!res.ok) {
    const message = data?.error || res.statusText;
    throw new Error(message || 'Request failed');
  }

  return data as T;
}

export async function register(payload: { name: string; email: string; password: string }): Promise<AuthResult> {
  const res = await fetch(`${API_BASE}/api/register`, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify(payload),
  });

  return handleJsonResponse<AuthResult>(res);
}

export async function login(payload: { email: string; password: string }): Promise<AuthResult> {
  const res = await fetch(`${API_BASE}/api/login`, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify(payload),
  });

  return handleJsonResponse<AuthResult>(res);
}

export async function fetchMe(token: string): Promise<AuthUser> {
  const res = await fetch(`${API_BASE}/api/me`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });

  return handleJsonResponse<AuthUser>(res);
}

export async function convertExcel(file: File, token?: string): Promise<{ blob: Blob; savedToDb: boolean; filename: string }> {
  const formData = new FormData();
  formData.append('file', file);

  const res = await fetch(`${API_BASE}/convert`, {
    method: 'POST',
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    body: formData,
  });

  if (!res.ok) {
    const errText = await res.text();
    throw new Error(errText || 'Conversion failed');
  }

  const blob = await res.blob();
  const savedToDb = res.headers.get('X-Db-Saved') === 'true';
  const disposition = res.headers.get('Content-Disposition');
  const filenameMatch = disposition?.match(/filename="?([^";]+)"?/);
  const filename = filenameMatch ? filenameMatch[1] : `${file.name}.csv`;

  return { blob, savedToDb, filename };
}

// File storage APIs
export interface SheetFileMeta {
  id: number;
  name: string;
  updated_at?: string;
  owner_id?: number;
  access_role?: 'owner' | 'editor' | 'viewer';
}

export async function listFiles(token: string): Promise<SheetFileMeta[]> {
  const res = await fetch(`${API_BASE}/api/files`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  const data = await handleJsonResponse<any>(res);

  // Handle both old format (array) and new format (object with files array)
  if (Array.isArray(data)) {
    return data as SheetFileMeta[];
  }

  // New paginated format
  if (data && Array.isArray(data.files)) {
    return data.files as SheetFileMeta[];
  }

  return [];
}

export async function saveFile(token: string, payload: { id?: number; name: string; state: any }): Promise<{ id: number; name: string }> {
  const res = await fetch(`${API_BASE}/api/files`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });
  return handleJsonResponse<{ id: number; name: string }>(res);
}

export interface SheetFileResponse {
  id: number;
  name: string;
  state: any;
  user_id?: number;
  created_at?: string;
  updated_at?: string;
  access_role?: 'owner' | 'editor' | 'viewer';
}

export async function getFile(token: string, id: number): Promise<SheetFileResponse> {
  const res = await fetch(`${API_BASE}/api/files/${id}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  return handleJsonResponse<SheetFileResponse>(res);
}

export async function deleteFile(token: string, id: number): Promise<void> {
  const res = await fetch(`${API_BASE}/api/files/${id}`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` },
  });

  if (!res.ok) {
    const text = await res.text();
    let data: any = null;
    try {
      data = text ? JSON.parse(text) : null;
    } catch (err) {
      throw new Error('Failed to delete file');
    }
    const message = data?.error || res.statusText;
    throw new Error(message || 'Failed to delete file');
  }
}

export interface FileShareRow {
  user_id: number;
  name: string;
  email: string;
  role: 'viewer' | 'editor';
  created_at?: string;
  updated_at?: string;
}

export async function listFileShares(token: string, fileId: number): Promise<{ file_id: number; shares: FileShareRow[] }> {
  let res = await fetch(`${API_BASE}/api/v1/files/${fileId}/shares`, {
    headers: { Authorization: `Bearer ${token}` },
  });

  if (!res.ok && res.status === 404) {
    res = await fetch(`${API_BASE}/api/files/${fileId}/shares`, {
      headers: { Authorization: `Bearer ${token}` },
    });
  }

  return handleJsonResponse<{ file_id: number; shares: FileShareRow[] }>(res);
}

export async function createFileShare(
  token: string,
  fileId: number,
  payload: { email: string; role?: 'viewer' | 'editor' }
): Promise<any> {
  let res = await fetch(`${API_BASE}/api/v1/files/${fileId}/shares`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  if (!res.ok && res.status === 404) {
    res = await fetch(`${API_BASE}/api/files/${fileId}/shares`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
    });
  }

  return handleJsonResponse<any>(res);
}

export async function deleteFileShare(token: string, fileId: number, userId: number): Promise<void> {
  let res = await fetch(`${API_BASE}/api/v1/files/${fileId}/shares/${userId}`, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` },
  });

  if (!res.ok && res.status === 404) {
    res = await fetch(`${API_BASE}/api/files/${fileId}/shares/${userId}`, {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${token}` },
    });
  }

  if (!res.ok) {
    const text = await res.text();
    let data: any = null;
    try {
      data = text ? JSON.parse(text) : null;
    } catch {
      throw new Error('Failed to delete share');
    }
    const message = data?.error || res.statusText;
    throw new Error(message || 'Failed to delete share');
  }
}

export async function generateApiKey(token: string): Promise<{ api_key: string }> {
  // Try v1 endpoint first, fallback to legacy
  let res = await fetch(`${API_BASE}/api/v1/api-key/generate`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
  });

  // Fallback to legacy endpoint if v1 doesn't exist
  if (!res.ok && res.status === 404) {
    res = await fetch(`${API_BASE}/api/api-key/generate`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}` },
    });
  }

  return handleJsonResponse<{ api_key: string }>(res);
}

export interface RealtimeTokenResponse {
  token: string;
  expires_in: number;
  sheet_id: number;
  role: string;
}

export async function getFileRealtimeToken(token: string, fileId: number): Promise<RealtimeTokenResponse> {
  // Try v1 endpoint first, fallback to legacy
  let res = await fetch(`${API_BASE}/api/v1/files/${fileId}/realtime/token`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` },
  });

  if (!res.ok && res.status === 404) {
    res = await fetch(`${API_BASE}/api/files/${fileId}/realtime/token`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${token}` },
    });
  }

  return handleJsonResponse<RealtimeTokenResponse>(res);
}

// AI endpoints
export interface AIRequest {
  prompt: string;
  sheet_data?: string;
  max_tokens?: number;
  temperature?: number;
}

export interface AIResponse {
  text: string;
  cached: boolean;
  timestamp: number;
}

export async function getGeminiApiKey(token: string): Promise<{ gemini_api_key: string | null }> {
  const res = await fetch(`${API_BASE}/api/v1/ai/gemini-key`, {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  });
  return handleJsonResponse<{ gemini_api_key: string | null }>(res);
}

export async function saveGeminiApiKey(token: string, geminiApiKey: string | null): Promise<{ gemini_api_key: string | null }> {
  const res = await fetch(`${API_BASE}/api/v1/ai/gemini-key`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ gemini_api_key: geminiApiKey }),
  });
  return handleJsonResponse<{ gemini_api_key: string | null }>(res);
}

export async function generateAI(token: string, request: AIRequest): Promise<AIResponse> {
  const res = await fetch(`${API_BASE}/api/v1/ai/generate`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(request),
  });

  return handleJsonResponse<AIResponse>(res);
}

export async function streamAI(
  token: string,
  request: AIRequest,
  onChunk: (chunk: string) => void,
  onComplete: () => void,
  onError: (error: string) => void
): Promise<void> {
  try {
    const res = await fetch(`${API_BASE}/api/v1/ai/stream`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${token}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(request),
    });

    if (!res.ok) {
      const error = await res.text();
      onError(error || 'Stream request failed');
      return;
    }

    const reader = res.body?.getReader();
    if (!reader) {
      onError('Response body is not readable');
      return;
    }

    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      for (const line of lines) {
        if (line.startsWith('data: ')) {
          const data = line.slice(6);
          try {
            const parsed = JSON.parse(data);
            if (parsed.done) {
              onComplete();
            } else if (parsed.candidates?.[0]?.content?.parts?.[0]?.text) {
              onChunk(parsed.candidates[0].content.parts[0].text);
            }
          } catch (e) {
            // Ignore parse errors
          }
        }
      }
    }
  } catch (error) {
    onError(error instanceof Error ? error.message : 'Stream error');
  }
}
