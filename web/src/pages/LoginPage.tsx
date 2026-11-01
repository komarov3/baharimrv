import { useState, type FormEvent } from 'react';
import { ArrowRight, Check, KeyRound, Leaf, ShieldCheck, Waves } from 'lucide-react';
import { Link, Navigate, useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext';
import { beginJahaziAuthorization, jahaziConfiguration } from '../auth/identity-sdk';
import { safeReturnPath, validateLogin } from '../auth/pkce';
import { Brand, Button, FormError, InlineNotice } from '../components/ui';

export function LoginPage() {
  const { login, isAuthenticated } = useAuth();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState<'local' | 'jahazi' | null>(null);
  const navigate = useNavigate();
  const location = useLocation();
  const returnTo = safeReturnPath(new URLSearchParams(location.search).get('returnTo'));

  if (isAuthenticated) return <Navigate to={returnTo} replace />;

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const validation = validateLogin(email, password);
    if (validation) { setError(validation); return; }
    setSubmitting('local');
    setError('');
    try {
      await login(email.trim(), password);
      navigate(returnTo, { replace: true });
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Sign in failed. Check your details and try again.');
    } finally {
      setSubmitting(null);
    }
  };

  const jahaziLogin = async () => {
    setSubmitting('jahazi');
    setError('');
    try {
      await beginJahaziAuthorization('login', returnTo);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Jahazi sign-in could not be started.');
      setSubmitting(null);
    }
  };

  return (
    <main className="auth-layout">
      <section className="auth-story" aria-label="About BahariMRV">
        <div className="auth-story-top"><Brand /><span className="auth-family">BahariLink / 04</span></div>
        <div className="auth-story-body">
          <div className="auth-symbol" aria-hidden="true"><Waves size={30} /><Leaf size={23} /></div>
          <h1>Field evidence.<br />Lasting proof.</h1>
          <p>Conservation operations for Kenya’s coast, from the first plot observation to an independently checkable achievement.</p>
          <ul>
            <li><Check size={17} />Offline-first monitoring and signed evidence</li>
            <li><Check size={17} />Independent verification before achievement claims</li>
            <li><Check size={17} />Provenance read back from the ledger, never assumed</li>
          </ul>
        </div>
        <p className="auth-story-foot">Evidence precedes claims. Privacy stays on shore.</p>
      </section>
      <section className="auth-form-side">
        <div className="auth-mobile-brand"><Brand /></div>
        <div className="login-form-wrap">
          <div className="login-heading"><span className="section-label">OPERATOR ACCESS</span><h2>Welcome back</h2><p>Sign in to your local BahariMRV account.</p></div>
          <form onSubmit={submit} noValidate>
            <div className="field"><label htmlFor="email">Email address</label><input id="email" name="email" type="email" autoComplete="username" value={email} onChange={(event) => setEmail(event.target.value)} placeholder="you@organisation.org" required /></div>
            <div className="field"><div className="label-row"><label htmlFor="password">Password</label><span>Local account</span></div><input id="password" name="password" type="password" autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} placeholder="Your password" required /></div>
            <FormError message={error} />
            <Button type="submit" className="full-width" loading={submitting === 'local'} icon={KeyRound}>Sign in to BahariMRV</Button>
          </form>
          <div className="auth-divider"><span>or use a linked platform</span></div>
          <Button type="button" variant="secondary" className="full-width jahazi-button" loading={submitting === 'jahazi'} onClick={jahaziLogin} icon={ShieldCheck}>Continue with Jahazi</Button>
          {!jahaziConfiguration() && <p className="configuration-note">Jahazi SSO requires public OIDC client configuration. Local sign-in remains available.</p>}
          <div className="auth-help"><p>First time connecting an existing account?</p><Link to="/auth/link">Link Jahazi after local sign in <ArrowRight size={14} /></Link></div>
          <InlineNotice tone="info" title="Identity provisioning is mandatory">New and federated accounts remain pending until their ledger identity and role capability are active. The sidebar names the network the server is actually writing to.</InlineNotice>
        </div>
        <footer className="auth-footer"><span>BahariMRV · Conservation evidence ledger</span><Link to="/proof/example">Public proof verification</Link></footer>
      </section>
    </main>
  );
}
