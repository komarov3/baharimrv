import { NavLink, Outlet, useLocation } from 'react-router-dom';
import { useAuth } from '../auth/AuthContext';
import { LayoutDashboard, Users, KeyRound, FolderOpen, Activity, ShieldCheck, Anchor, FileCheck, ScrollText, Plug, AlertTriangle, Edit3, GitCommit, Coins, ShoppingCart, Receipt, Trash2, DollarSign, Satellite, RefreshCw, BookOpen, Search, Settings, Download } from 'lucide-react';

const nav = [
  { to: '/search', label: 'Search', icon: Search },
  { to: '/overview', label: 'Overview', icon: LayoutDashboard },
  { to: '/users', label: 'Users', icon: Users },
  { to: '/identities', label: 'Identities', icon: KeyRound },
  { to: '/projects', label: 'Projects', icon: FolderOpen },
  { to: '/activities', label: 'Activities', icon: Activity },
  { to: '/verification', label: 'Verification', icon: ShieldCheck },
  { to: '/anchors', label: 'Anchors', icon: Anchor },
  { to: '/proofs', label: 'Proofs', icon: FileCheck },
  { to: '/disputes', label: 'Disputes', icon: AlertTriangle },
  { to: '/corrections', label: 'Corrections', icon: Edit3 },
  { to: '/provenance', label: 'Provenance', icon: GitCommit },
  { to: '/credit-batches', label: 'Credit batches', icon: Coins },
  { to: '/marketplace', label: 'Marketplace', icon: ShoppingCart },
  { to: '/purchases', label: 'Purchases', icon: Receipt },
  { to: '/retirements', label: 'Retirements', icon: Trash2 },
  { to: '/finance', label: 'Finance', icon: DollarSign },
  { to: '/satellite', label: 'Satellite', icon: Satellite },
  { to: '/sync', label: 'Sync', icon: RefreshCw },
  { to: '/methodologies', label: 'Methodologies', icon: BookOpen },
{ to: '/alerts', label: 'Alerts', icon: AlertTriangle },
{ to: '/config', label: 'Config', icon: Settings },
{ to: '/exports', label: 'Exports', icon: Download },
  { to: '/audit', label: 'Audit', icon: ScrollText },
  { to: '/integrations', label: 'Integrations', icon: Plug },
];

export function AppShell() {
  const { user, logout } = useAuth();
  const location = useLocation();
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="sidebar-brand"><strong>BahariMRV Admin</strong></div>
        <nav className="desktop-nav">
          {nav.map(({ to, label, icon: Icon }) => (
            <NavLink key={to} to={to}><Icon size={18} />{label}</NavLink>
          ))}
        </nav>
        <div className="sidebar-foot">
          <p>{user?.email}</p>
          <button className="button button-quiet" onClick={logout}>Sign out</button>
        </div>
      </aside>
      <div className="app-column">
        <header className="topbar">
          <div className="topbar-context"><span>{location.pathname}</span></div>
        </header>
        <main className="main-content"><Outlet /></main>
      </div>
    </div>
  );
}
