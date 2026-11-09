/*
THESIS: A working field-evidence ledger, not a decorative impact dashboard.
OWN-WORLD: White work surfaces, fine coastal borders, emerald actions, and navy
financial/proof summaries; compact rows carry explicit evidence states.
STORY: Operators move from plot observation to independent verification, proof,
registry-backed custody, and retirement without conflating those milestones.
FIRST VIEWPORT: A 232px task sidebar and contextual bar frame the project map,
API-returned metric ledger, wallet strip, alerts, and Field visit action.
FORM: The binding C1–C4 hierarchy extends into a responsive Operate console;
below 900px it returns to the five-destination mobile flow.
*/
import type { ReactNode } from 'react';
import { Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { useAuth } from './auth/AuthContext';
import { AppShell } from './components/AppShell';
import { Brand, StatePanel } from './components/ui';
import { AuthCallbackPage } from './pages/AuthCallbackPage';
import { CreditsPage } from './pages/CreditsPage';
import { DashboardPage } from './pages/DashboardPage';
import { LinkAccountPage } from './pages/LinkAccountPage';
import { LoginPage } from './pages/LoginPage';
import { MonitorPage } from './pages/MonitorPage';
import { PlantingActivityPage } from './pages/PlantingActivityPage';
import { ProfilePage } from './pages/ProfilePage';
import { ProjectsPage } from './pages/ProjectsPage';
import { ModuleEntryPage } from './pages/ModuleEntryPage';
import { ProofsPage } from './pages/ProofsPage';
import { PublicProofPage } from './pages/PublicProofPage';
import { SatelliteMonitoringPage } from './pages/SatelliteMonitoringPage';
import { VerificationPage } from './pages/VerificationPage';
import { WalletPage } from './pages/WalletPage';
function RequireAuth({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth();
  const location = useLocation();
  if (!isAuthenticated) return <Navigate to={`/login?returnTo=${encodeURIComponent(location.pathname + location.search)}`} replace />;
  return children;
}

function NotFoundPage() {
  return <main className="not-found"><Brand /><StatePanel kind="empty" title="This BahariMRV surface does not exist"><p>Use the project dashboard to return to conservation operations, or open a permanent public proof URL.</p><a className="button button-primary" href="/app/dashboard">Open project dashboard</a></StatePanel></main>;
}

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/app/dashboard" replace />} />
      <Route path="/login" element={<LoginPage />} />
      <Route path="/auth/callback" element={<AuthCallbackPage />} />
      <Route path="/auth/link" element={<LinkAccountPage />} />
      <Route path="/proof/:id" element={<PublicProofPage />} />
      <Route path="/app" element={<RequireAuth><AppShell /></RequireAuth>}>
        <Route index element={<Navigate to="dashboard" replace />} />
        <Route path="dashboard" element={<DashboardPage />} />
        <Route path="module-entry" element={<ModuleEntryPage />} />
        <Route path="monitor" element={<MonitorPage />} />
        <Route path="verification" element={<VerificationPage />} />
        <Route path="credits" element={<CreditsPage />} />
        <Route path="planting" element={<PlantingActivityPage />} />
        <Route path="satellite" element={<SatelliteMonitoringPage />} />
        <Route path="wallet" element={<WalletPage />} />
        <Route path="profile" element={<ProfilePage />} />
        <Route path="projects" element={<ProjectsPage />} />
"
        <Route path="proofs" element={<ProofsPage />} />
      </Route>
      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  );
}
