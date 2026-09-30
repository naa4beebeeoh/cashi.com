# cashi.com

Monorepo starter for the Cashi backend API and React Native app.

## Projects

- `backend/`: Go HTTP API, listening on `:8080` by default.
- `frontend/`: Expo React Native app with TypeScript.

## Run the backend

```sh
cd backend
go run ./cmd/api
```

Check that it is running:

```sh
curl http://localhost:8080/healthz
```

The endpoint returns `{"status":"ok"}`. Set `API_ADDR` to change the listen address.

Run backend tests with `cd backend && go test ./...`.

## Run the frontend

```sh
cd frontend
npm install
npm start
```

Use Expo Go or an emulator to open the app. The app pings the backend at `EXPO_PUBLIC_API_URL` (defaults to `http://localhost:8080`). For an Android emulator, set it to `http://10.0.2.2:8080`; for a physical device, use the development machine's LAN IP. Copy `.env.example` to `.env` to set the URL.