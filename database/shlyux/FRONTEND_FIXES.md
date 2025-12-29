# Frontend Tuzatishlari

Bu fayl frontend (shlyux) dagi tuzatilgan muammolarni tushuntiradi.

## Tuzatilgan Muammolar

### 1. ✅ Open Tugmasi (File Dropdown) Muammosi

**Muammo:**
- Open tugmasini bosganda fayllar ro'yxati ko'rinardi, lekin file nomlari bo'sh edi
- Faqat file iconlari ko'rinib, nomlar ko'rinmasdi

**Sabab:**
- Backend API `SheetFile` modelida JSON tag'lar yo'q edi
- Go default ravishda field nomlarini katta harf bilan yuboradi: `Name`, `ID`, `UserID`
- Frontend esa kichik harf kutardi: `name`, `id`, `user_id`
- Natijada frontend `file.name` ni o'qiyotganda `undefined` qiymat kelardi

**Yechim:**
`backend-go/internal/models/file.go` faylida JSON tag'lar qo'shildi:

```go
type SheetFile struct {
	ID        uint            `gorm:"primaryKey" json:"id"`
	UserID    uint            `gorm:"index;not null" json:"user_id"`
	Name      string          `gorm:"not null" json:"name"`
	State     json.RawMessage `gorm:"type:jsonb;not null" json:"state"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}
```

Backend qayta build qilindi va ishga tushirildi:
```bash
docker-compose build backend-go
docker-compose up -d backend-go
```

---

### 2. ✅ API Format Compatibility (Pagination)

**Muammo:**
- Backend v2.3.0 da pagination qo'shildi va API response formati o'zgardi
- Eski format: `[{id: 1, name: "file1"}, ...]` (array)
- Yangi format: `{files: [...], pagination: {...}}` (object)

**Yechim:**
`shlyux/utils/api.ts` faylida `listFiles` funksiyasi yangilandi:

```typescript
export async function listFiles(token: string): Promise<SheetFileMeta[]> {
  const res = await fetch(`${API_BASE}/api/files`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  const data = await handleJsonResponse<any>(res);

  console.log('DEBUG listFiles: Raw API response:', data);

  // Handle both old format (array) and new format (object with files array)
  if (Array.isArray(data)) {
    console.log('DEBUG listFiles: Using old array format, files count:', data.length);
    return data as SheetFileMeta[];
  }

  // New paginated format
  if (data && Array.isArray(data.files)) {
    console.log('DEBUG listFiles: Using new paginated format, files count:', data.files.length);
    console.log('DEBUG listFiles: Files data:', data.files);
    return data.files as SheetFileMeta[];
  }

  console.log('DEBUG listFiles: No files found, returning empty array');
  return [];
}
```

Bu ham eski, ham yangi formatlarni qo'llab-quvvatlaydi (backward compatibility).

---

### 3. ✅ Clear Tugmasi Ishlamayapti

**Muammo:**
- Toolbar dagi "Clear" tugmasini bosganda faqat style tozalanardi
- Value va computed field'lar o'zgarmay qolardi

**Sabab:**
- `App.tsx` dagi `handleStyleChange` funksiyasi `__reset: true` ni olganida faqat style'ni tozalayardi
- Value va computed field'lar o'zgarmay qolardi

**Yechim:**
`shlyux/App.tsx` faylida `handleStyleChange` funksiyasi yangilandi:

```typescript
const handleStyleChange = (styleChange: any) => {
  if (!sheet.selection) return;
  const { __reset, ...rest } = styleChange || {};

  // ...

  for (let r = minR; r <= maxR; r++) {
    for (let c = minC; c <= maxC; c++) {
      const id = getCellId(r, c);
      const currentCell = newData[id] || { value: '' };
      if (__reset) {
        // Clear both style AND value/computed ⬅️ YANGILANDI
        newData[id] = { value: '', computed: '', style: {} };
        editsToSend.push({ row: r, col: c, value: '' });
      } else {
        // ...
      }
    }
  }

  // Recompute if reset
  const newSheet = { ...sheet, data: __reset ? recomputeSheet(newData) : newData };
  saveState(newSheet);

  // Broadcast to realtime
  if (__reset && realtimeClient && editsToSend.length > 0) {
    // ...
  }
};
```

Endi Clear tugmasi:
1. ✅ Cell value'ni tozalaydi
2. ✅ Computed value'ni tozalaydi
3. ✅ Style'ni tozalaydi
4. ✅ Formula'larni qayta hisoblaydi
5. ✅ Real-time collaboration'ga broadcast qiladi

---

### 4. ✅ API Key Generatsiya Xatosi

**Muammo:**
- Dev tugmasidan API Key yaratishga urinilganda 404 xatosi
- Console'da: `POST http://localhost:8080/api/api-key 404`

**Sabab:**
- Backend v2.3.0 da endpoint `/api/v1/api-key/generate` ga o'zgartirildi
- Frontend eski `/api/api-key` endpoint'ga so'rov yubormoqda edi

**Yechim:**
`shlyux/utils/api.ts` faylida `generateApiKey` funksiyasi yangilandi:

```typescript
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
```

Endi:
1. ✅ Avval yangi v1 endpoint'ga urinadi
2. ✅ Agar 404 bo'lsa, eski endpoint'ga fallback qiladi
3. ✅ Backward compatibility ta'minlangan

---

### 5. ✅ Debug Logging Qo'shildi

Frontend debug qilish uchun console.log'lar qo'shildi:

**shlyux/components/Header.tsx:**
```typescript
files.map((file) => {
    console.log('DEBUG: Rendering file:', file);
    return (
        <button
            onClick={() => {
                console.log('DEBUG: File clicked:', file.id, file.name);
                // ...
            }}
        >
            {/* ... */}
        </button>
    );
})
```

**shlyux/utils/api.ts:**
```typescript
console.log('DEBUG listFiles: Raw API response:', data);
console.log('DEBUG listFiles: Using new paginated format, files count:', data.files.length);
console.log('DEBUG listFiles: Files data:', data.files);
```

---

### 6. ✅ File Delete Tugmasi Qo'shildi

**Muammo:**
- Open file dropdown'ida delete tugmasi yo'q edi
- Fayllarni o'chirib bo'lmasdi

**Sabab:**
- Frontend'da delete funksiyasi hech qachon implement qilinmagan edi
- Backend'da DELETE endpoint mavjud edi, lekin route qo'shilmagan edi

**Yechim:**

**1. Backend route qo'shildi:**
`backend-go/cmd/server/main.go`:
```go
api.DELETE("/files/:id", fileHandler.Delete)
```

**2. Frontend API funksiya qo'shildi:**
`shlyux/utils/api.ts`:
```typescript
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
```

**3. App.tsx'da delete handler qo'shildi:**
```typescript
const handleDeleteFile = useCallback(async (id: number) => {
  if (!token) return;
  if (!confirm('Bu faylni o\'chirishni xohlaysizmi?')) return;

  try {
    await deleteFile(token, id);
    // Refresh file list
    const updatedFiles = await listFiles(token);
    setFiles(updatedFiles);

    // If deleted file was the current file, create a new one
    if (currentFileId === id) {
      handleNewFile();
    }
  } catch (err) {
    console.error('Delete file error:', err);
    alert('Faylni o\'chirishda xato: ' + (err instanceof Error ? err.message : 'Unknown error'));
  }
}, [token, currentFileId]);
```

**4. Header.tsx'da delete tugmasi qo'shildi:**
- Har bir file item'ning yonida qizil Trash icon
- Hover qilganda background rang o'zgaradi
- Click qilganda tasdiqlash dialog chiqadi
- File o'chirilgandan keyin list avtomatik yangilanadi

**5. CORS configuration'ga DELETE method qo'shildi:**
`backend-go/cmd/server/main.go`:
```go
AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
```

**Muammo:** "Failed to fetch" xatosi chiqardi chunki CORS'da DELETE method yo'q edi.

**Yechim:** CORS configuration'ga DELETE, PUT, PATCH method'lar qo'shildi.

**6. File raqamlari va UI yaxshilanishlari:**
- Har bir file oldida raqam qo'shildi (1., 2., 3., ...)
- Active button'lar qora fon o'rniga ko'k rangda (`bg-blue-100`)
- Active tugmalar ko'k border bilan (`border-blue-300`)
- Text rangi ko'k (`text-blue-600`)

**Qanday ishlaydi:**
1. Open tugmasini bosing
2. File ro'yxatida har bir file:
   - **Raqam** bilan ko'rinadi (1., 2., 3., ...)
   - File icon
   - File nomi
   - Qizil **trash icon** 🗑️
3. Trash icon'ga bosing
4. "Bu faylni o'chirishni xohlaysizmi?" tasdiqni qabul qiling
5. File o'chiriladi va list yangilanadi
6. Agar o'chirilgan file hozirgi file bo'lsa, yangi file yaratiladi

---

## Cho'tka Icon (Paintbrush) Funksiyasi

**Savol:** Cho'tka icon vazifasi nima?

**Javob:**
Bu **Format Painter** (Format Ko'chirish) funksiyasi.

### Qanday ishlaydi:

1. **Birinchi qadam**: Formatini ko'chirmoqchi bo'lgan cell'ni tanlang (masalan, qizil rang, bold, 18px)

2. **Ikkinchi qadam**: Cho'tka (Paintbrush) tugmasini bosing
   - Tugma aktivlashadi (qora rangga aylanadi)
   - Tanlangan cell'ning formati nusxalanadi

3. **Uchinchi qadam**: Format qo'llashni xohlagan boshqa cell'ga bosing
   - Format avtomatik ko'chiriladi
   - Tugma avtomatik deaktivlashadi

### Misol:
```
A1: "Sarlavha" (qizil, bold, 18px)
    ↓
[Paintbrush] bosing
    ↓
B5 cell'ga bosing
    ↓
B5 endi qizil, bold, 18px bo'ladi (faqat format, text emas!)
```

**Keyboard shortcut:** Yo'q (faqat tugma orqali)

**Kod joylashuvi:**
- Toolbar: `shlyux/components/Toolbar.tsx:95-101`
- Handler: `shlyux/App.tsx:738-752`
- Icon: `<Paintbrush size={18} />` (lucide-react)

---

## Testlash

Tuzatishlarni test qilish:

```bash
cd shlyux
npm run dev
```

### Test Cases:

1. **Open Tugmasi:**
   - ✅ Login qiling
   - ✅ Bir nechta file saqlang
   - ✅ Open tugmasini bosing
   - ✅ Fayllar ro'yxati ko'rinishi kerak
   - ✅ File nomlari to'g'ri ko'rinishi kerak

2. **Clear Tugmasi:**
   - ✅ Bir nechta cell'ga text va style qo'shing
   - ✅ Cell'larni tanlang
   - ✅ Clear tugmasini bosing
   - ✅ Hamma narsa tozalanishi kerak

3. **Format Painter:**
   - ✅ A1'ga "Test" yozing va qizil, bold qiling
   - ✅ A1'ni tanlang
   - ✅ Cho'tka tugmasini bosing
   - ✅ B5'ga bosing
   - ✅ B5 qizil, bold bo'lishi kerak

4. **API Key:**
   - ✅ Dev tugmasini bosing
   - ✅ "API key yaratish" ni bosing
   - ✅ API key yaratilishi kerak
   - ✅ Console'da xato bo'lmasligi kerak

---

## Qo'shimcha Ma'lumotlar

### Backend O'zgarishlari

**File:** `backend-go/internal/models/file.go`

Eski kod:
```go
type SheetFile struct {
	ID        uint            `gorm:"primaryKey"`
	Name      string          `gorm:"not null"`
	// ...
}
```

Yangi kod:
```go
type SheetFile struct {
	ID        uint            `gorm:"primaryKey" json:"id"`
	Name      string          `gorm:"not null" json:"name"`
	// ...
}
```

### File Structure

```
shlyux/
├── App.tsx                  # Main app component (UPDATED)
├── components/
│   ├── Header.tsx           # Top header with file operations (UPDATED - Debug logs)
│   ├── Toolbar.tsx          # Formatting toolbar
│   └── ...
├── utils/
│   └── api.ts              # API calls (UPDATED - Debug logs, pagination compatibility)
└── FRONTEND_FIXES.md       # This file

backend-go/
├── internal/
│   └── models/
│       └── file.go         # SheetFile model (UPDATED - JSON tags)
└── ...
```

---

## Muammolar?

Agar muammolar yuzaga kelsa:

1. **Browser Console** ni tekshiring (F12)
2. **Network Tab** da API request'larni ko'ring
3. **Backend logs** ni tekshiring: `docker logs database-backend-go-1`
4. **GitHub Issues** da xabar bering

---

Barcha tuzatishlar qilindi! ✅
