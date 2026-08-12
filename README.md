# BrandRelay OAuth Broker v1

Small standard-library Go service for BrandRelay/Hall Monitor secure provider authorization.

It exposes:

- `GET /healthz`
- `POST /v1/connect/start`
- `GET /v1/connect/status`
- `GET /callback/{provider}`

The public deployment must be HTTPS. Provider client secrets are read only from server environment variables. OAuth sessions are held in memory for 15 minutes and are protected by both provider `state` and a separate random Hall Monitor polling secret.

Use `.env.example` for configuration and the Hall Monitor `SECURE_CONNECT_DEPLOYMENT.md` file for callback URLs.
