import { Navigate, Route, Routes } from 'react-router-dom';
import { DisputesPage } from './pages/DisputesPage';

import { useAuth } from './auth/AuthContext';
import { AppShell } from './components/AppShell';
import { LoginPage } from './pages/LoginPage';
import { OverviewPage } from './pages/OverviewPage';
import { UsersPage } from './pages/UsersPage';
import { IdentitiesPage } from './pages/IdentitiesPage';
import { ProjectsPage } from './pages/ProjectsPage';
import { ActivitiesPage } from './pages/ActivitiesPage';
import { VerificationPage } from './pages/VerificationPage';
import { AnchorsPage } from './pages/AnchorsPage';
import { AlertsPage } from './pages/AlertsPage';
import { ConfigPage } from './pages/ConfigPage';
import { ExportsPage } from './pages/ExportsPage';
import { ProofsPage } from './pages/ProofsPage';
import { AuditPage } from './pages/AuditPage';
import { IntegrationsPage } from './pages/IntegrationsPage';
import { DataQualityPage } from './pages/DataQualityPage';
import { EvidencePage } from './pages/EvidencePage';
import { CorrectionsPage } from './pages/CorrectionsPage';
import { ProvenancePage } from './pages/ProvenancePage';
import { CreditBatchesPage } from './pages/CreditBatchesPage';
import { MarketplacePage } from './pages/MarketplacePage';
import { PurchasesPage } from './pages/PurchasesPage';
import { RetirementsPage } from './pages/RetirementsPage';
import { FinancePage } from './pages/FinancePage';
import { SatellitePage } from './pages/SatellitePage';
import { SyncPage } from './pages/SyncPage';
import { MethodologiesPage } from './pages/MethodologiesPage';
import { SearchPage } from './pages/SearchPage';

function RequireAuth({ children }: { children: JSX.Element }) {
  const { token, user } = useAuth();
  if (!token || !user) return <Navigate to="/login" replace />;
  if (user.role !== 'platform_administrator') return <div className="state-panel">Administrator access required.</div>;
  return children;
}

export default function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<RequireAuth><AppShell /></RequireAuth>}>
        <Route index element={<Navigate to="overview" replace />} />
        <Route path="overview" element={<OverviewPage />} />
        <Route path="users" element={<UsersPage />} />
        <Route path="identities" element={<IdentitiesPage />} />
        <Route path="projects" element={<ProjectsPage />} />
        <Route path="activities" element={<ActivitiesPage />} />
        <Route path="verification" element={<VerificationPage />} />
        <Route path="anchors" element={<AnchorsPage />} />
        <Route path="proofs" element={<ProofsPage />} />
        <Route path="audit" element={<AuditPage />} />
        <Route path="integrations" element={<IntegrationsPage />} />
        <Route path="disputes" element={<DisputesPage />} />
        <Route path="corrections" element={<CorrectionsPage />} />
        <Route path="provenance" element={<ProvenancePage />} />
        <Route path="credit-batches" element={<CreditBatchesPage />} />
        <Route path="marketplace" element={<MarketplacePage />} />
        <Route path="purchases" element={<PurchasesPage />} />
        <Route path="retirements" element={<RetirementsPage />} />
        <Route path="finance" element={<FinancePage />} />
        <Route path="satellite" element={<SatellitePage />} />
        <Route path="sync" element={<SyncPage />} />
        <Route path="search" element={<SearchPage />} />
        <Route path="methodologies" element={<MethodologiesPage />} />
        <Route path="evidence" element={<EvidencePage />} />
        <Route path="data-quality" element={<DataQualityPage />} />
        <Route path="alerts" element={<AlertsPage />} />
        <Route path="config" element={<ConfigPage />} />
        <Route path="exports" element={<ExportsPage />} />
      </Route>
    </Routes>
  );
}
