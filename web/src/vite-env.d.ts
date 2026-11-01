/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_URL?: string;
  readonly VITE_DEFAULT_PROJECT_ID?: string;
  readonly VITE_JAHAZI_CLIENT_ID?: string;
  readonly VITE_JAHAZI_ISSUER?: string;
  readonly VITE_JAHAZI_REDIRECT_URI?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
