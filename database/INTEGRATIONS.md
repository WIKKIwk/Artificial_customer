# Integrations (Sheets API)

Bu loyiha har qanday sheet’ni boshqa dasturlar bilan ulash uchun **REST API** va **Realtime (WebSocket)** beradi.

## 1) Auth (API key)

1. Web UI’da `Dev` tugmasi → **API key yaratish**
2. Barcha so‘rovlarda header qo‘shing:

`X-API-Key: sk_...`

API key — parolga o‘xshaydi. Uni hech kimga oshkor qilmang, kerak bo‘lsa qayta generate qiling.

## 2) REST API (o‘qish/yozish)

Base URL: `http://localhost:8080`

### Fayllar ro‘yxati

`GET /api/v1/files`

### Faylni to‘liq olish (state JSON)

`GET /api/v1/files/:id`

### Range bo‘yicha kataklarni o‘qish

`GET /api/v1/files/:id/cells?range=A1:D20&format=grid`

Query:
- `range` (majburiy) — A1 format, masalan `A1:D20`
- `format` — `grid` (default) yoki `sparse`
- `value` — `raw` (default) yoki `computed` (agar state’da bo‘lsa)

### Kataklarni yangilash (batch update)

`PATCH /api/v1/files/:id/cells`

Body:
```json
{
  "edits": [
    { "cell": "A2", "value": "Hello" },
    { "row": 0, "col": 1, "value": "42" }
  ]
}
```

### Schema (ustun headerlari + used range)

`GET /api/v1/files/:id/schema`

Bu endpoint integratsiya qilayotgan dasturga:
- used range (min/max row/col)
- header row (taxmin)
- har bir ustun uchun header qiymatini beradi.

## 3) Realtime (WebSocket)

Realtime server: `ws://localhost:4000/socket`

1) Avval short-lived token oling:

`POST /api/v1/realtime/token`

2) Phoenix channel’ga ulang:
- topic: `spreadsheet:<file_id>`
- events: `cell_update`, `batch_update`, `full_sync`
- write: `cell_edit`, `batch_edit`

Tavsiya: realtime orqali “live sync”, REST orqali “import/export / bulk write”.

