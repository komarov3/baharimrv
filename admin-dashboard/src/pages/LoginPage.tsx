import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext';

export function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  async function submit(event: React.FormEvent) {
    event.preventDefault();
    try {
      await login(email, password);
      navigate('/overview');
    } catch {
      setError('Login failed. Administrator access required.');
    }
  }
  return (
    <main className="login-page">
      <form className="card" onSubmit={submit}>
        <h1>Admin Console</h1>
        {error && <p className="error">{error}</p>}
        <label>Email<input value={email} onChange={(e) => setEmail(e.target.value)} /></label>
        <label>Password<input type="password" value={password} onChange={(e) => setPassword(e.target.value)} /></label>
        <button className="button button-primary" type="submit">Sign in</button>
      </form>
    </main>
  );
}
