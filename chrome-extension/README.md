# Quka Chrome Extension

Builds a Manifest V3 Chrome popup for QukaAI.

## API mapping

- `GET /api/v1/user/info` with `X-Access-Token` or `X-Authorization`
- `GET /api/v1/space/list`
- `GET /api/v1/:spaceid/resource/list`
- `POST /api/v1/:spaceid/chat`
- `POST /api/v1/:spaceid/chat/:session/message/id`
- `POST /api/v1/:spaceid/chat/:session/message`
- `GET /api/v1/:spaceid/chat/:session/history/list`
- `POST /api/v1/:spaceid/knowledge`

## Development

```bash
cd chrome-extension
npm install
npm run build
```

Then load `chrome-extension/dist` in `chrome://extensions` with Developer mode enabled.
