import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useAuth } from '../auth/AuthContext';
import { api } from '../lib/api';
import type { ProjectRow } from '../types/api';

const PROJECT_KEY = 'baharimrv.current-project';

interface ProjectContextValue {
  projectId: string;
  projectName?: string;
  rows: ProjectRow[];
  loading: boolean;
  error: string;
  setProjectId: (id: string, name?: string) => void;
  reload: () => void;
}

const ProjectContext = createContext<ProjectContextValue | null>(null);

export function ProjectProvider({ children }: { children: ReactNode }) {
  const { isAuthenticated } = useAuth();
  const [rows, setRows] = useState<ProjectRow[]>([]);
  const [projectId, setProjectIdState] = useState(() => localStorage.getItem(PROJECT_KEY) ?? '');
  const [projectName, setProjectName] = useState<string>();
  const [loading, setLoading] = useState(isAuthenticated);
  const [error, setError] = useState('');
  const [revision, setRevision] = useState(0);
  const autoSelected = useRef(false);

  const setProjectId = useCallback((id: string, name?: string) => {
    setProjectIdState(id);
    setProjectName(name);
    if (id) localStorage.setItem(PROJECT_KEY, id);
    else localStorage.removeItem(PROJECT_KEY);
  }, []);

  const reload = useCallback(() => setRevision((value) => value + 1), []);

  useEffect(() => {
    if (!isAuthenticated) { setLoading(false); return; }
    const controller = new AbortController();
    setLoading(true);
    setError('');
    api.getProjects(controller.signal)
      .then((table) => {
        if (controller.signal.aborted) return;
        setRows(table.rows);
        // A stored ID can outlive the project or the caller's access to it, so
        // only an ID the API actually returned is trusted.
        const stored = table.rows.find((row) => row.id === localStorage.getItem(PROJECT_KEY));
        if (stored) {
          setProjectIdState(stored.id);
          setProjectName(stored.name);
        } else if (table.rows.length === 1 && !autoSelected.current) {
          autoSelected.current = true;
          setProjectId(table.rows[0].id, table.rows[0].name);
        }
      })
      .catch((cause: unknown) => {
        if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : 'The project list could not be loaded.');
      })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [isAuthenticated, revision, setProjectId]);

  const value = useMemo<ProjectContextValue>(
    () => ({ projectId, projectName, rows, loading, error, setProjectId, reload }),
    [projectId, projectName, rows, loading, error, setProjectId, reload],
  );

  return <ProjectContext.Provider value={value}>{children}</ProjectContext.Provider>;
}

export function useProject(): ProjectContextValue {
  const context = useContext(ProjectContext);
  if (!context) throw new Error('useProject must be used inside ProjectProvider');
  return context;
}
