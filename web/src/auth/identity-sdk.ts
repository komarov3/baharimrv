import { createPkceMaterial, safeReturnPath, saveOidcTransaction, type OidcTransaction } from './pkce';

interface AuthorizationRequestInput {
  issuer: string;
  clientId: string;
  redirectUri: string;
  scopes: string[];
  state: string;
  nonce: string;
  codeVerifier: string;
  codeChallenge: string;
}

interface IdentitySdkBrowser {
  createAuthorizationRequest?: (input: AuthorizationRequestInput) => Promise<{ url: string } | string>;
}

const SDK_PACKAGE = '@baharilink/identity-sdk';

/**
 * Clean optional boundary for the shared BahariLink SDK. The peer package can be
 * added when Nanga publishes it; standards-based PKCE remains usable meanwhile.
 */
async function loadIdentitySdk(): Promise<IdentitySdkBrowser | null> {
  try {
    return (await import(/* @vite-ignore */ SDK_PACKAGE)) as IdentitySdkBrowser;
  } catch {
    return null;
  }
}

export function jahaziConfiguration(): { issuer: string; clientId: string; redirectUri: string } | null {
  const issuer = import.meta.env.VITE_JAHAZI_ISSUER?.replace(/\/$/, '');
  const clientId = import.meta.env.VITE_JAHAZI_CLIENT_ID;
  const redirectUri = import.meta.env.VITE_JAHAZI_REDIRECT_URI ?? `${window.location.origin}/auth/callback`;
  if (!issuer || !clientId) return null;
  return { issuer, clientId, redirectUri };
}

export async function beginJahaziAuthorization(mode: OidcTransaction['mode'], returnTo?: string): Promise<void> {
  const config = jahaziConfiguration();
  if (!config) throw new Error('Jahazi sign-in is not configured. Add the public OIDC client settings or use local sign in.');

  const material = await createPkceMaterial();
  const transaction: OidcTransaction = {
    ...material,
    mode,
    redirectUri: config.redirectUri,
    returnTo: safeReturnPath(returnTo),
    createdAt: Date.now(),
  };
  saveOidcTransaction(transaction);

  const requestInput: AuthorizationRequestInput = {
    ...config,
    scopes: ['openid', 'profile', 'email'],
    state: material.state,
    nonce: material.nonce,
    codeVerifier: material.verifier,
    codeChallenge: material.challenge,
  };

  const sdk = await loadIdentitySdk();
  let authorizationUrl: string;
  if (sdk?.createAuthorizationRequest) {
    const request = await sdk.createAuthorizationRequest(requestInput);
    authorizationUrl = typeof request === 'string' ? request : request.url;
  } else {
    const url = new URL(`${config.issuer}/authorize`);
    url.search = new URLSearchParams({
      response_type: 'code',
      client_id: config.clientId,
      redirect_uri: config.redirectUri,
      scope: requestInput.scopes.join(' '),
      state: material.state,
      nonce: material.nonce,
      code_challenge: material.challenge,
      code_challenge_method: 'S256',
    }).toString();
    authorizationUrl = url.toString();
  }

  window.location.assign(authorizationUrl);
}
