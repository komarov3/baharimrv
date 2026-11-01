import { useEffect, useState } from 'react';
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { BadgeCheck, FileCheck2, FolderOpen, Home, Leaf, LogOut, Menu, MonitorSmartphone, UserRound, WalletCards, Waves, X, Coins, ShieldCheck, Wifi, WifiOff } from 'lucide-react';
import { useAuth } from '../auth/AuthContext';
import { api } from '../lib/api';
import { useProject } from './ProjectContext';
import { Brand, Button } from './ui';

const primaryNavigation = [
  { to: '/app/dashboard', label: 'Home', desktopLabel: 'Project dashboard', icon: Home },
  { to: '/app/monitor', label: 'Monitor', desktopLabel: 'Field capture', icon: MonitorSmartphone },
  { to: '/app/credits', label: 'Credits', desktopLabel: 'Carbon marketplace', icon: Coins },
  { to: '/app/wallet', label: 'Wallet', desktopLabel: 'Project wallet', icon: WalletCards },
  { to: '/app/profile', label: 'Profile', desktopLabel: 'Identity & platforms', icon: UserRound },
];

const taskNavigation = [
  { to: '/app/projects', label: 'Projects', icon: FolderOpen },
  { to: '/app/verification', label: 'Verification pipeline', icon: ShieldCheck },
  { to: '/app/proofs', label: 'Proof center', icon: FileCheck2 },
];

const pageNames: Record<string, string> = {
  '/app/dashboard': 'Project overview',
  '/app/monitor': 'MRV field capture',
  '/app/verification': 'Independent verification',
  '/app/credits': 'Registry-backed credits',
  '/app/wallet': 'Financial evidence',
  '/app/profile': 'Account & identity',
  '/app/proofs': 'Proof center',
};

function NetworkStatus() {
  const [online, setOnline] = useState(navigator.onLine);
  useEffect(() => {
    const update = () => setOnline(navigator.onLine);
    window.addEventListener('online', update);
    window.addEventListener('offline', update);
    return () => { window.removeEventListener('online', update); window.removeEventListener('offline', update); };
  }, []);
  return <span className={`network-status ${online ? '' : 'is-offline'}`}>{online ? <Wifi size={15} /> : <WifiOff size={15} />}{online ? 'Online' : 'Offline · capture remains available'}</span>;
}

// ChainLabel states what the server's own adapter is doing. A sandbox anchor is
// never displayed as a public-ledger fact.
function ChainLabel() {
  const [mode, setMode] = useState<string | null>(null);
  useEffect(() => {
    const controller = new AbortController();
    api.getHealth(controller.signal)
      .then((health) => setMode(health.modes.chain))
      .catch(() => { if (!controller.signal.aborted) setMode('unavailable'); });
    return () => controller.abort();
  }, []);
  const label = mode === 'iota' ? 'IOTA anchoring'
    : mode === 'sandbox' ? 'Sandbox ledger · not a public chain'
    : mode === 'unavailable' ? 'Chain status unavailable'
    : 'Checking chain mode…';
  return <div className={`testnet-label ${mode === 'sandbox' ? 'is-sandbox' : ''}`}><span className="status-dot" />{label}</div>;
}

export function AppShell() {
  const { user, logout } = useAuth();
  const { projectId, rows, setProjectId } = useProject();
  const [menuOpen, setMenuOpen] = useState(false);
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => { setMenuOpen(false); }, [location.pathname]);

  return (
    <div className="app-shell">
      <a className="skip-link" href="#main-content">Skip to main content</a>
      <aside className="sidebar" aria-label="Primary navigation">
        <div className="sidebar-brand"><Brand /></div>
        <div className="operate-label"><Leaf size={14} aria-hidden="true" /> Operate console</div>
        <nav className="desktop-nav">
          <span className="nav-section-label">Conservation</span>
          {primaryNavigation.slice(0, 2).map(({ to, desktopLabel, icon: Icon }) => <NavLink key={to} to={to}><Icon size={18} />{desktopLabel}</NavLink>)}
          {taskNavigation.map(({ to, label, icon: Icon }) => <NavLink key={to} to={to}><Icon size={18} />{label}</NavLink>)}
          <span className="nav-section-label">Assets & account</span>
          {primaryNavigation.slice(2).map(({ to, desktopLabel, icon: Icon }) => <NavLink key={to} to={to}><Icon size={18} />{desktopLabel}</NavLink>)}
        </nav>
        <div className="sidebar-foot">
          <ChainLabel />
          <p>Evidence before claims.<br />Independent checks, always.</p>
        </div>
      </aside>

      <div className="app-column">
        <header className="topbar">
          <div className="mobile-brand"><Brand compact /><span>{pageNames[location.pathname] ?? 'BahariMRV'}</span></div>
          <div className="topbar-context"><Waves size={17} aria-hidden="true" /><span>{pageNames[location.pathname] ?? 'Conservation operations'}</span></div>
          <div className="topbar-actions">
            <NetworkStatus />
            <div className="project-switcher">
              <label htmlFor="project-context">Project</label>
              {rows.length
                ? <select id="project-context" value={projectId} onChange={(event) => { const next = rows.find((row) => row.id === event.target.value); setProjectId(event.target.value, next?.name); }}>
                  {!projectId && <option value="">Select a project</option>}
                  {rows.map((row) => <option key={row.id} value={row.id}>{row.name}</option>)}
                </select>
                : <span className="switcher-empty">No project selected</span>}
            </div>
            <div className="user-menu-wrap">
              <button className="user-trigger" onClick={() => setMenuOpen((open) => !open)} aria-expanded={menuOpen} aria-controls="account-menu">
                <span className="avatar">{user?.name?.slice(0, 1).toUpperCase() ?? 'U'}</span>
                <span className="user-trigger-copy"><strong>{user?.name ?? 'Account'}</strong><small>{user?.role ?? 'Operator'}</small></span>
                {menuOpen ? <X size={16} /> : <Menu size={16} />}
              </button>
              {menuOpen && <div className="account-menu" id="account-menu"><div><strong>{user?.email}</strong><span>Local BahariMRV session</span></div><Button variant="quiet" icon={LogOut} onClick={() => { void logout().then(() => navigate('/login')); }}>Sign out</Button></div>}
            </div>
          </div>
        </header>
        <main id="main-content" className="main-content" tabIndex={-1}><Outlet /></main>
      </div>

      <nav className="mobile-nav" aria-label="Mobile navigation">
        {primaryNavigation.map(({ to, label, icon: Icon }) => <NavLink key={to} to={to}><Icon size={21} strokeWidth={1.8} /><span>{label}</span></NavLink>)}
      </nav>
      <div className="mobile-task-links" aria-label="Additional tools">
        {(location.pathname === '/app/verification' || location.pathname === '/app/proofs') && <NavLink to="/app/dashboard"><BadgeCheck size={16} />Back to project</NavLink>}
      </div>
    </div>
  );
}
