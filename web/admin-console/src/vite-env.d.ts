/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_BASE_DOMAIN?: string
  readonly VITE_DEV_ORG?: string
  readonly VITE_API_BASE?: string
  readonly VITE_METRICS_BASE?: string
  readonly VITE_OIDC_ISSUER_TEMPLATE?: string
  readonly VITE_OIDC_CLIENT_ID?: string
  readonly VITE_OIDC_SCOPE?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
