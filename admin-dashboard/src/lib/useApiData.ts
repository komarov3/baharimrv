import { useCallback, useEffect, useState } from 'react';

export function useApiData<T>(loader: () => Promise<T>) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [revision, setRevision] = useState(0);
  const reload = useCallback(() => setRevision((v) => v + 1), []);
  useEffect(() => {
    let active = true;
    setLoading(true);
    setError('');
    loader().then((result) => { if (active) setData(result); }).catch((e) => { if (active) setError(e instanceof Error ? e.message : 'Failed to load'); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [loader, revision]);
  return { data, error, loading, reload };
}
