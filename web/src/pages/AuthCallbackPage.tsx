import { useEffect, useRef, useState } from 'react';
import { ShieldCheck } from 'lucide-react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext';
import { callbackError, consumeOidcTransaction } from '../auth/pkce';
import { jahaziConfiguration } from '../auth/identity-sdk';
import { api } from '../lib/api';
import { Brand, Button, FormError } from '../components/ui';

export function AuthCallbackPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const { acceptSession } = useAuth();
  const [error, setError] = useState('');
  const [mode, setMode] = useState<'login' | 'link'>('login');
  const attempted = useRef(false);

  useEffect(() => {
    if (attempted.current) return;
    attempted.current = true;
    const complete = async () => {
      const providerError = callbackError(location.search);
      if (providerError) { setError(providerError); return; }
      const params = new URLSearchParams(location.search);
      const code = params.get('code');
      const state = params.get('state');
      if (!code || !state) { setError('The callback is missing an authorization code or state. No session was created.'); return; }
      try {
        const transaction = consumeOidcTransaction(state);
        setMode(transaction.mode);
        const config = jahaziConfiguration();
        if (!config) throw new Error('Jahazi sign-in is not configured for this deployment.');
        const input = { issuer: config.issuer, code, codeVerifier: transaction.verifier, nonce: transaction.nonce, redirectUri: transaction.redirectUri };
        if (transaction.mode === 'link') {
          await api.linkJahazi(input);
          navigate('/app/profile', { replace: true, state: { linked: true } });
        } else {
          acceptSession(await api.exchangeJahazi(input));
          navigate(transaction.returnTo, { replace: true });
        }
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : 'The authorization could not be completed. Start again.');
      }
    };
    void complete();
  }, [acceptSession, location.search, navigate]);

  return (
    <main className="callback-page">
      <Brand />
      <section className="callback-card" aria-live="polite">
        <div className={`callback-icon ${error ? 'has-error' : ''}`}><ShieldCheck size={28} /></div>
        <span className="section-label">SECURE FEDERATION</span>
        <h1>{error ? 'Authorization not completed' : mode === 'link' ? 'Linking your Jahazi identity' : 'Establishing your local session'}</h1>
        {error ? <>
          <FormError message={error} />
          <p>Authorization codes and tokens are never placed in BahariMRV URLs. Return to the start to create a fresh PKCE transaction.</p>
          <div className="button-row"><Link className="button button-primary" to={mode === 'link' ? '/auth/link' : '/login'}>Start again</Link><Link className="button button-secondary" to="/login">Local sign in</Link></div>
        </> : <><p>Validating state and exchanging the single-use code with the BahariMRV backend. Keep this tab open.</p><div className="callback-progress" role="status"><span /><span /><span /><span className="sr-only">Validating authorization</span></div></>}
      </section>
      {!error && <Button variant="quiet" onClick={() => navigate('/login')}>Cancel and return to sign in</Button>}
    </main>
  );
}
