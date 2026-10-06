/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE?: string
  readonly VITE_API_PROXY?: string
  readonly VITE_SESSION_IDLE_TIMEOUT_MIN?: string
  readonly VITE_LOGIN_FAIL_LOCK_THRESHOLD?: string
  readonly VITE_LOGIN_FAIL_LOCK_MINUTES?: string
  readonly VITE_PASSWORD_MIN_LENGTH?: string
  readonly VITE_PASSWORD_REQUIRE_UPPER?: string
  readonly VITE_PASSWORD_REQUIRE_LOWER?: string
  readonly VITE_PASSWORD_REQUIRE_DIGIT?: string
  readonly VITE_PASSWORD_REQUIRE_SPECIAL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
