import type { ReactNode } from 'react';
import { ArrowRight, FolderOpen } from 'lucide-react';
import { useAuth } from '../auth/AuthContext';
import { Link } from 'react-router-dom';
import { useProject } from './ProjectContext';
import { Panel, Skeleton, StatePanel, StatusBadge } from './ui';

export function ProjectGate({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  const { projectId, rows, loading, error, setProjectId, reload } = useProject();
  const isManager = user?.role === 'project_manager';
  if (projectId) return children;

  if (loading) {
    return <Panel title="Loading authorized projects" description="The register comes from the BahariMRV API; nothing is assumed while it loads."><Skeleton rows={3} /></Panel>;
  }
  if (error) {
    return <StatePanel kind="error" title="Project list unavailable" onRetry={reload}><p>{error}</p><p>No dashboard, plot, or financial values are substituted for the records this account has not been given.</p></StatePanel>;
  }
  if (!rows.length) {
    return (
      <StatePanel kind="empty" title={isManager ? 'Create your first community project' : 'No projects are authorized for this account'} action={isManager ? <Link className="button button-primary" to="/app/projects">Start project onboarding</Link> : undefined}>
        <p>{isManager ? 'You signed up as a community project manager, but no project has been created yet. Use project onboarding to create your first project, site, and plot.' : 'This sign-in is valid, but no conservation project has been assigned to it yet. Ask a BahariMRV administrator to grant project access.'}</p>
      </StatePanel>
    );
  }

  return (
    <Panel title="Choose a project" description="Every screen below loads only the records this account is authorized to see for the project selected here.">
      <div className="project-picker">
        {rows.map((row) => (
          <button key={row.id} type="button" className="project-picker-row" onClick={() => setProjectId(row.id, row.name)}>
            <span className="picker-mark"><FolderOpen size={18} /></span>
            <span className="picker-body">
              <strong>{row.name}</strong>
              <small>{row.communityGroup || 'Community group not recorded'} · {row.site || 'Site not recorded'}</small>
              <span className="picker-meta">
                <StatusBadge status={row.verificationState} />
                {row.sandboxDemo && <StatusBadge status="sandbox" />}
                <span className="count-label mono">{row.id}</span>
              </span>
            </span>
            <ArrowRight size={17} className="picker-arrow" />
          </button>
        ))}
      </div>
    </Panel>
  );
}
