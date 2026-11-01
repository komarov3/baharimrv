import { useState } from 'react';
import { ArrowLeft, Link2, LockKeyhole, ShieldCheck, UserCheck } from 'lucide-react';
import { Link } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext';
import { beginJahaziAuthorization } from '../auth/identity-sdk';
import { Brand, Button, FormError, InlineNotice } from '../components/ui';

export function LinkAccountPage() {
  const { user, isAuthenticated } = useAuth();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const begin = async () => {
    setLoading(true);
    setError('');
    try { await beginJahaziAuthorization('link', '/app/profile'); }
    catch (cause) { setError(cause instanceof Error ? cause.message : 'Account linking could not be started.'); setLoading(false); }
  };

  return (
    <main className="link-page">
      <header><Brand /><Link to={isAuthenticated ? '/app/profile' : '/login'}><ArrowLeft size={16} />{isAuthenticated ? 'Back to profile' : 'Back to sign in'}</Link></header>
      <section className="link-card">
        <div className="link-visual" aria-hidden="true"><span><UserCheck size={28} /></span><Link2 size={24} /><span><ShieldCheck size={28} /></span></div>
        <span className="section-label">EXPLICIT ACCOUNT LINKING</span>
        <h1>Connect Jahazi to this account</h1>
        <p>Linking binds the exact Jahazi issuer and subject to your authenticated BahariMRV account. Email addresses and phone numbers are never used to guess a match.</p>
        {isAuthenticated ? <div className="link-local-account"><LockKeyhole size={18} /><div><strong>{user?.name}</strong><span>{user?.email} · local session confirmed</span></div></div> : <InlineNotice tone="warning" title="Local sign-in required">Sign in to the existing BahariMRV account before starting a separate Jahazi authorization.</InlineNotice>}
        <FormError message={error} />
        {isAuthenticated ? <Button icon={Link2} loading={loading} onClick={begin}>Authorize with Jahazi</Button> : <Link className="button button-primary" to="/login?returnTo=/auth/link">Sign in to continue</Link>}
        <ol className="link-steps"><li><span>1</span>Approve identity scopes in Jahazi</li><li><span>2</span>Return with a single-use authorization code</li><li><span>3</span>BahariMRV validates and stores the issuer/subject link</li></ol>
        <p className="privacy-note">No passwords, refresh tokens, or legacy signing secrets are shared between platforms. Local roles remain local.</p>
      </section>
    </main>
  );
}
