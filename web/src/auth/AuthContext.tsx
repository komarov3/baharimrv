import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react';
import { api } from '../lib/api';
import type { AuthSession, User } from '../types/api';

const TOKEN_KEY = 'baharimrv.session.token';
const USER_KEY = 'baharimrv.session.user';

interface AuthContextValue {
  token: string | null;
  user: User | null;
  isAuthenticated: boolean;
  acceptSession: (session: AuthSession) => void;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

function storedUser(): User | null {
  try {
    const value = sessionStorage.getItem(USER_KEY);
    return value ? (JSON.parse(value) as User) : null;
  } catch {
    return null;
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState(() => sessionStorage.getItem(TOKEN_KEY));
  const [user, setUser] = useState<User | null>(storedUser);

  const acceptSession = useCallback((session: AuthSession) => {
    sessionStorage.setItem(TOKEN_KEY, session.sessionToken);
    sessionStorage.setItem(USER_KEY, JSON.stringify(session.user));
    api.setToken(session.sessionToken);
    setToken(session.sessionToken);
    setUser(session.user);
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    acceptSession(await api.login(email, password));
  }, [acceptSession]);

  const logout = useCallback(async () => {
    // The server revokes the session token; a failed call never traps the user
    // in a signed-in shell they cannot leave.
    try { await api.logout(); } catch { /* the local clear below still runs */ }
    sessionStorage.removeItem(TOKEN_KEY);
    sessionStorage.removeItem(USER_KEY);
    api.setToken(undefined);
    setToken(null);
    setUser(null);
  }, []);

  const value = useMemo(() => ({ token, user, isAuthenticated: Boolean(token), acceptSession, login, logout }), [token, user, acceptSession, login, logout]);
  api.setToken(token ?? undefined);
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) throw new Error('useAuth must be used inside AuthProvider');
  return context;
}
