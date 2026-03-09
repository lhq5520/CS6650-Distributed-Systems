# Hummingbird Midterm — Bug Investigation Report

---

## Ticket #1 — "Server started on the wrong port"

### Bug Location
**File:** `server.js`, **Line 35**

### What's Broken
```js
const port = process.env.APP_PORT;
```
The port is read directly from the environment variable `APP_PORT` with no default fallback. If `APP_PORT` is not set in `.env`, `port` is `undefined`. In Node.js, `app.listen(undefined)` does not crash — it silently binds to a random OS-assigned port. The log outputs `listening on port undefined`, and the server is unreachable on the expected port 9000.

### Why It's a Bug
Express/Node's `http.listen()` accepts `undefined` as a valid argument — it just picks an arbitrary ephemeral port. There is no error, no crash, no warning. The service appears healthy but is not listening where the ALB (or any client) expects it.

### Code Diff

**Before** (`server.js:35`):
```js
const port = process.env.APP_PORT;
```

**After**:
```js
const port = process.env.APP_PORT || 9000;
```

---

## Ticket #2 — "Width is missing from metadata"

### Bug Location
**File:** `clients/dynamodb.js`, **Lines 78–84** (inside `getMedia`)

### What's Broken
`createMedia` (line 34) saves `width` to DynamoDB:
```js
width: { N: String(width) },
```
But `getMedia` (lines 78–84) never reads it back:
```js
return {
  mediaId,
  size: Number(Item.size.N),
  name: Item.name.S,
  mimetype: Item.mimetype.S,
  status: Item.status.S,
};
```
The `width` field is stored in the DynamoDB item but is omitted from the return object.

### Why It's a Bug
The data is in DynamoDB — it's just never extracted. When a client calls `GET /v1/media/:id`, the `getController` returns whatever `getMedia` gives it. Since `width` is missing from the return object, the API response never includes it, even though it was correctly saved during upload.

### Code Diff

**Before** (`clients/dynamodb.js:78-84`):
```js
return {
  mediaId,
  size: Number(Item.size.N),
  name: Item.name.S,
  mimetype: Item.mimetype.S,
  status: Item.status.S,
};
```

**After**:
```js
return {
  mediaId,
  size: Number(Item.size.N),
  name: Item.name.S,
  mimetype: Item.mimetype.S,
  status: Item.status.S,
  width: Number(Item.width.N),
};
```

---

## Ticket #3 — "Your redirect URL is broken"

### Bug Location
**File:** `controllers/media.js`, **Line 111** (inside `downloadController`)

### What's Broken
```js
res.set('Location', `${req.hostname}/v1/media/${mediaId}/status`);
```
`req.hostname` in Express returns just the hostname (e.g., `hummingbird-alb-xxx.elb.amazonaws.com`) — no protocol prefix. The resulting `Location` header is:
```
Location: hummingbird-alb-xxx.elb.amazonaws.com/v1/media/abc/status
```
This is not a valid absolute URL. HTTP clients that follow redirects or `Location` headers expect a full URL starting with `http://` (or `https://`).

### Why It's a Bug
Per RFC 7231, a `Location` header on a `3xx` or `201` response should be a URI-reference. Most HTTP clients expect an absolute URI with a scheme (`http://`). Without the scheme, the client either fails to follow the redirect or interprets the value as a relative path, which resolves incorrectly.

### Code Diff

**Before** (`controllers/media.js:111`):
```js
res.set('Location', `${req.hostname}/v1/media/${mediaId}/status`);
```

**After**:
```js
res.set('Location', `http://${req.get('host')}/v1/media/${mediaId}/status`);
```

> **Note:** `req.get('host')` is used instead of `req.hostname` because `req.get('host')` includes the port (e.g., `hostname:9000`), which is important when the service is not running on port 80.

---

## Ticket #4 — "Download never redirects even when COMPLETE"

### Bug Location
**File:** `controllers/media.js`, **Line 108** (inside `downloadController`)

### What's Broken
```js
if (media.status !== MEDIA_STATUS.PROCESSING) {
```
This condition returns a 202 ("still processing") when the status is anything **other than** `PROCESSING`. That means:
- `PENDING` → not `PROCESSING` → 202 (correct by accident)
- `COMPLETE` → not `PROCESSING` → 202 (WRONG — should redirect)
- `ERROR` → not `PROCESSING` → 202 (also wrong)

The only status that would trigger the redirect (302) is `PROCESSING` — exactly backwards from the intended logic.

### Why It's a Bug
The intent is: "if the media is NOT yet complete, return 202; otherwise redirect." The condition should check `!== COMPLETE`, not `!== PROCESSING`. As written, a `COMPLETE` item never reaches the `res.redirect(302, url)` line — the function always returns 202 first.

### Code Diff

**Before** (`controllers/media.js:108`):
```js
if (media.status !== MEDIA_STATUS.PROCESSING) {
```

**After**:
```js
if (media.status !== MEDIA_STATUS.COMPLETE) {
```

---

## Bonus — "Status never changes. No errors. Nothing."

### Bug Location
**File:** `clients/dynamodb.js`, **Line 154** (inside `setMediaStatus`)

### Investigation Notes

Three DynamoDB functions operate on media records:

| Function | PK | SK |
|---|---|---|
| `createMedia` (line 29) | `MEDIA#${mediaId}` | `METADATA` |
| `getMedia` (line 62) | `MEDIA#${mediaId}` | `METADATA` |
| `setMediaStatus` (line 154) | `MEDIA#${mediaId}` | `metadata` |

Notice the casing difference: `createMedia` and `getMedia` use `SK: 'METADATA'` (uppercase), but `setMediaStatus` uses `SK: 'metadata'` (lowercase).

DynamoDB keys are case-sensitive. When `setMediaStatus` runs an `UpdateItem` with `SK: 'metadata'`, it targets a **different item** than the one created by `createMedia` (which used `SK: 'METADATA'`). Since there is no `ConditionExpression` on the update, DynamoDB silently **creates a new item** with the lowercase key instead of updating the existing one. No error is thrown.

So what happens on `PUT /resize`:
1. `setMediaStatus({ newStatus: 'PROCESSING' })` → creates a NEW item `(MEDIA#xxx, metadata)` with status=PROCESSING
2. `copyMediaFile(...)` → succeeds
3. `setMediaStatus({ newStatus: 'COMPLETE' })` → updates that same NEW item to status=COMPLETE
4. Response: `{ status: 'COMPLETE' }` — looks correct
5. `GET /status` calls `getMedia` → reads `(MEDIA#xxx, METADATA)` → the **original** item → still `PENDING`

The original record is never touched. The status updates go to a phantom item.

### Code Diff

**Before** (`clients/dynamodb.js:154`):
```js
SK: { S: 'metadata' },
```

**After**:
```js
SK: { S: 'METADATA' },
```

---

## Summary Table

| Ticket | File | Line | Root Cause | Fix |
|--------|------|------|------------|-----|
| #1 | `server.js` | 35 | No default port fallback | `process.env.APP_PORT \|\| 9000` |
| #2 | `clients/dynamodb.js` | 78–84 | `width` not returned in `getMedia` | Add `width: Number(Item.width.N)` |
| #3 | `controllers/media.js` | 111 | Location header missing `http://` | Use `http://${req.get('host')}/...` |
| #4 | `controllers/media.js` | 108 | Wrong status comparison (`!== PROCESSING`) | Change to `!== COMPLETE` |
| Bonus | `clients/dynamodb.js` | 154 | SK casing mismatch (`metadata` vs `METADATA`) | Change to `'METADATA'` |

---

## Verification

After applying all fixes, the application was rebuilt and tested:

1. **Ticket #1** — Server started successfully and logged `Example app listening on port 9000` (previously logged `port undefined`).
2. **Ticket #2** — `getMedia` now returns the `width` field in its response object, confirmed via `GET /v1/media/:id`.
3. **Ticket #3** — The `Location` header in 202 responses now contains a valid absolute URL starting with `http://` and including the host and port.
4. **Ticket #4** — `GET /v1/media/:id/download` correctly returns a 302 redirect when status is `COMPLETE`, instead of always returning 202.
5. **Bonus** — `setMediaStatus` now targets the same DynamoDB item (`SK: 'METADATA'`) as `createMedia` and `getMedia`, so status updates are applied to the correct record.

All endpoints were tested with `curl` and confirmed working as expected.
