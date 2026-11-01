const TRANSACTION_KEY = 'baharimrv.oidc.transaction';

export interface PkceMaterial {
  verifier: string;
  challenge: string;
  state: string;
  nonce: string;
}

export interface OidcTransaction extends PkceMaterial {
  mode: 'login' | 'link';
  redirectUri: string;
  returnTo: string;
  createdAt: number;
}

function base64Url(bytes: Uint8Array): string {
  let binary = '';
  bytes.forEach((byte) => { binary += String.fromCharCode(byte); });
  return btoa(binary).replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/, '');
}

function randomValue(length = 32): string {
  const bytes = crypto.getRandomValues(new Uint8Array(length));
  return base64Url(bytes);
}

export async function createPkceMaterial(): Promise<PkceMaterial> {
  const verifier = randomValue(48);
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier));
  return {
    verifier,
    challenge: base64Url(new Uint8Array(digest)),
    state: randomValue(32),
    nonce: randomValue(32),
  };
}

export function safeReturnPath(value: string | null | undefined, fallback = '/app/dashboard'): string {
  if (!value || !value.startsWith('/') || value.startsWith('//') || value.includes('\\')) return fallback;
  return value;
}

export function validateLogin(email: string, password: string): string | null {
  if (!/^\S+@\S+\.\S+$/.test(email.trim())) return 'Enter a valid email address.';
  if (password.length < 8) return 'Password must be at least 8 characters.';
  return null;
}

export function saveOidcTransaction(transaction: OidcTransaction): void {
  sessionStorage.setItem(TRANSACTION_KEY, JSON.stringify(transaction));
}

export function readOidcTransaction(): OidcTransaction | null {
  const raw = sessionStorage.getItem(TRANSACTION_KEY);
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as OidcTransaction;
    if (Date.now() - parsed.createdAt > 10 * 60 * 1000) {
      sessionStorage.removeItem(TRANSACTION_KEY);
      return null;
    }
    return parsed;
  } catch {
    sessionStorage.removeItem(TRANSACTION_KEY);
    return null;
  }
}

export function consumeOidcTransaction(state: string): OidcTransaction {
  const transaction = readOidcTransaction();
  sessionStorage.removeItem(TRANSACTION_KEY);
  if (!transaction) throw new Error('This sign-in attempt expired or was not started in this browser. Return to sign in and try again.');
  if (transaction.state !== state) throw new Error('The sign-in state did not match. No credentials were accepted; start again.');
  return transaction;
}

export function callbackError(search: string): string | null {
  const params = new URLSearchParams(search);
  const error = params.get('error');
  if (!error) return null;
  const description = params.get('error_description');
  return description ? `${error}: ${description}` : `Jahazi returned: ${error}`;
}
