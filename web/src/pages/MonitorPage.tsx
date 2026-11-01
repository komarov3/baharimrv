import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { Camera, Check, CloudUpload, Crosshair, FileAudio, Leaf, MapPin, RefreshCw, Save, ShieldCheck, Trash2, WifiOff } from 'lucide-react';
import { Button, FormError, InlineNotice, PageHeader, Panel, Skeleton, StatePanel, StatusBadge } from '../components/ui';
import { isPositiveDecimal } from '../lib/decimal';
import { formatDate, titleCase } from '../lib/format';
import { deleteCapture, listCaptures, putCapture, saveCapture, syncCapture, type QueuedMonitoringRecord } from '../lib/offlineQueue';

interface CaptureForm {
  activityId: string;
  plotId: string;
  methodologyVersion: string;
  schemaVersion: string;
  latitude: string;
  longitude: string;
  accuracy: string;
  alive: string;
  planted: string;
  height: string;
  species: string;
  observations: string;
  identityDid: string;
}

const initialForm: CaptureForm = { activityId: '', plotId: '', methodologyVersion: '', schemaVersion: '1.0', latitude: '', longitude: '', accuracy: '', alive: '', planted: '', height: '', species: '', observations: '', identityDid: '' };

export function MonitorPage() {
  const [form, setForm] = useState(initialForm);
  const [photos, setPhotos] = useState<File[]>([]);
  const [voice, setVoice] = useState<File[]>([]);
  const [attested, setAttested] = useState(false);
  const [clientId, setClientId] = useState(() => crypto.randomUUID());
  const [queue, setQueue] = useState<QueuedMonitoringRecord[]>([]);
  const [queueLoading, setQueueLoading] = useState(true);
  const [queueError, setQueueError] = useState('');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [saving, setSaving] = useState(false);
  const [syncing, setSyncing] = useState<string | 'all' | null>(null);
  const [online, setOnline] = useState(navigator.onLine);

  const refreshQueue = useCallback(async () => {
    try { setQueue(await listCaptures()); setQueueError(''); }
    catch (cause) { setQueueError(cause instanceof Error ? cause.message : 'The offline queue could not be loaded.'); }
    finally { setQueueLoading(false); }
  }, []);

  useEffect(() => {
    void refreshQueue();
    const update = () => setOnline(navigator.onLine);
    window.addEventListener('online', update);
    window.addEventListener('offline', update);
    return () => { window.removeEventListener('online', update); window.removeEventListener('offline', update); };
  }, [refreshQueue]);

  const update = (key: keyof CaptureForm, value: string) => setForm((current) => ({ ...current, [key]: value }));

  const locate = () => {
    setError('');
    if (!navigator.geolocation) { setError('GPS is not supported by this browser. Enter verified coordinates manually.'); return; }
    navigator.geolocation.getCurrentPosition(
      (position) => setForm((current) => ({ ...current, latitude: String(position.coords.latitude), longitude: String(position.coords.longitude), accuracy: String(position.coords.accuracy) })),
      (cause) => setError(`GPS capture failed: ${cause.message}. Enable location access or enter verified coordinates.`),
      { enableHighAccuracy: true, timeout: 15000, maximumAge: 0 },
    );
  };

  const validate = (): string | null => {
    if (!form.activityId.trim() || !form.plotId.trim() || !form.methodologyVersion.trim()) return 'Activity, plot, and methodology version are required.';
    const latitude = Number(form.latitude);
    const longitude = Number(form.longitude);
    const accuracy = Number(form.accuracy);
    if (!Number.isFinite(latitude) || latitude < -90 || latitude > 90 || !Number.isFinite(longitude) || longitude < -180 || longitude > 180) return 'Enter valid GPS coordinates.';
    if (!Number.isFinite(accuracy) || accuracy <= 0) return 'Enter GPS accuracy in metres; zero or blank accuracy is not accepted.';
    if (!/^\d+$/.test(form.alive) || !/^\d+$/.test(form.planted)) return 'Alive and planted counts must be whole numbers.';
    if (Number(form.alive) > Number(form.planted)) return 'Alive count cannot exceed planted count. Review the measurements.';
    if (!isPositiveDecimal(form.height)) return 'Average height must be a positive decimal in metres.';
    if (!form.species.trim()) return 'Record at least one observed species.';
    if (!photos.length) return 'Add at least one original geo-tagged photo before saving.';
    if (!attested) return 'Confirm the monitor attestation before saving this record.';
    return null;
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const validation = validate();
    if (validation) { setError(validation); return; }
    setSaving(true);
    setError('');
    setNotice('');
    try {
      await saveCapture({
        clientId,
        activityId: form.activityId.trim(),
        plotId: form.plotId.trim(),
        methodologyVersion: form.methodologyVersion.trim(),
        schemaVersion: form.schemaVersion.trim(),
        deviceTimestamp: new Date().toISOString(),
        location: { latitude: Number(form.latitude), longitude: Number(form.longitude), accuracyMetres: Number(form.accuracy) },
        aliveCount: Number(form.alive),
        plantedCount: Number(form.planted),
        averageHeightMetres: form.height,
        species: form.species.trim(),
        observations: form.observations.trim(),
        attestation: { statement: 'I attest that this monitoring record and its original evidence accurately represent my field observation.', identityDid: form.identityDid.trim() || undefined },
        files: [...photos.map((file) => ({ file, kind: 'photo' as const })), ...voice.map((file) => ({ file, kind: 'voice' as const }))],
      });
      setNotice(online ? 'Saved on this device with evidence hashes. Synchronize to submit it to the server.' : 'Saved offline on this device. It remains On device until you reconnect and synchronize.');
      setForm(initialForm);
      setPhotos([]);
      setVoice([]);
      setAttested(false);
      setClientId(crypto.randomUUID());
      await refreshQueue();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'The record could not be saved on this device.');
    } finally { setSaving(false); }
  };

  const syncOne = async (record: QueuedMonitoringRecord) => {
    if (!online) { setQueueError('Reconnect before synchronizing. Your record remains safely on this device.'); return; }
    setSyncing(record.clientId);
    setQueueError('');
    const result = await syncCapture(record);
    if (result.status === 'failed') setQueueError(result.error ?? 'Synchronization failed.');
    setSyncing(null);
    await refreshQueue();
  };

  const syncAll = async () => {
    if (!online) { setQueueError('Reconnect before synchronizing. Your records remain on this device.'); return; }
    setSyncing('all');
    setQueueError('');
    const pending = queue.filter((record) => ['on_device', 'failed'].includes(record.status));
    for (const record of pending) await syncCapture(record);
    setSyncing(null);
    await refreshQueue();
  };

  const remove = async (record: QueuedMonitoringRecord) => {
    try { await deleteCapture(record); await refreshQueue(); }
    catch (cause) { setQueueError(cause instanceof Error ? cause.message : 'The record cannot be deleted.'); }
  };

  const correctedCopy = async (record: QueuedMonitoringRecord) => {
    const nextId = crypto.randomUUID();
    await putCapture({ ...record, clientId: nextId, payload: { ...record.payload, clientId: nextId }, monitoringRecordId: undefined, status: 'on_device', conflict: undefined, error: undefined });
    setNotice('A new on-device correction was created. The conflicting original remains in the queue for audit.');
    await refreshQueue();
  };

  const pendingCount = queue.filter((record) => ['on_device', 'failed', 'sync_conflict'].includes(record.status)).length;

  return (
    <>
      <PageHeader title="MRV field capture" description="Preserve original field evidence on this device, then synchronize without duplication." meta={<StatusBadge status={online ? 'online' : 'offline'} label={online ? 'Ready to capture' : 'Offline mode'} />} actions={<Button variant="secondary" icon={CloudUpload} onClick={syncAll} loading={syncing === 'all'} disabled={!pendingCount || !online}>Sync queue ({pendingCount})</Button>} />
      {!online && <InlineNotice tone="warning" title="Working offline">Complete records and original media stay on this device. Queued records are not submitted, anchored, or verified.</InlineNotice>}
      <div className="capture-layout">
        <form className="capture-form" onSubmit={submit} noValidate>
          <Panel title="Plot & methodology" description="Identifiers bind this observation to the authorized monitoring plan.">
            <div className="form-grid form-grid-2">
              <div className="field"><label htmlFor="activity-id">Activity ID</label><input id="activity-id" value={form.activityId} onChange={(e) => update('activityId', e.target.value)} placeholder="Authorized activity identifier" required /></div>
              <div className="field"><label htmlFor="plot-id">Plot identity</label><div className="input-with-icon"><MapPin size={16} /><input id="plot-id" value={form.plotId} onChange={(e) => update('plotId', e.target.value)} placeholder="Plot identifier" required /></div></div>
              <div className="field"><label htmlFor="methodology-version">Methodology version</label><input id="methodology-version" value={form.methodologyVersion} onChange={(e) => update('methodologyVersion', e.target.value)} placeholder="e.g. methodology-version from plan" required /></div>
              <div className="field"><label htmlFor="schema-version">Record schema version</label><input id="schema-version" value={form.schemaVersion} onChange={(e) => update('schemaVersion', e.target.value)} required /></div>
            </div>
            <div className="gps-block"><div className="gps-heading"><span><Crosshair size={17} />GPS position</span><Button type="button" variant="quiet" onClick={locate}>Use device GPS</Button></div><div className="form-grid form-grid-3">
              <div className="field"><label htmlFor="latitude">Latitude</label><input id="latitude" inputMode="decimal" value={form.latitude} onChange={(e) => update('latitude', e.target.value)} placeholder="Decimal degrees" required /></div>
              <div className="field"><label htmlFor="longitude">Longitude</label><input id="longitude" inputMode="decimal" value={form.longitude} onChange={(e) => update('longitude', e.target.value)} placeholder="Decimal degrees" required /></div>
              <div className="field"><label htmlFor="accuracy">Accuracy</label><div className="input-unit"><input id="accuracy" inputMode="decimal" value={form.accuracy} onChange={(e) => update('accuracy', e.target.value)} required /><span>m</span></div></div>
            </div></div>
          </Panel>

          <Panel title="Survival observation" description="Record measured values only. Estimated impact is calculated later under the methodology.">
            <div className="form-grid form-grid-3">
              <div className="field"><label htmlFor="alive">Alive count</label><input id="alive" inputMode="numeric" value={form.alive} onChange={(e) => update('alive', e.target.value)} required /></div>
              <div className="field"><label htmlFor="planted">Planted count</label><input id="planted" inputMode="numeric" value={form.planted} onChange={(e) => update('planted', e.target.value)} required /></div>
              <div className="field"><label htmlFor="height">Average height</label><div className="input-unit"><input id="height" inputMode="decimal" value={form.height} onChange={(e) => update('height', e.target.value)} required /><span>m</span></div></div>
            </div>
            <div className="field"><label htmlFor="species">Species observed</label><div className="input-with-icon"><Leaf size={16} /><input id="species" value={form.species} onChange={(e) => update('species', e.target.value)} placeholder="Species names, separated by commas" required /></div></div>
            <div className="field"><label htmlFor="observations">Field observations <span className="optional">Optional</span></label><textarea id="observations" rows={3} value={form.observations} onChange={(e) => update('observations', e.target.value)} placeholder="Conditions, threats, maintenance, or measurement notes" /></div>
          </Panel>

          <Panel title="Original evidence" description="SHA-256 hashes are computed on this device before upload. Original files are preserved offline.">
            <div className="upload-grid">
              <label className="upload-zone"><Camera size={24} /><strong>Geo-tagged photos</strong><span>{photos.length ? `${photos.length} selected · originals retained` : 'At least one required'}</span><input type="file" accept="image/*" capture="environment" multiple onChange={(e) => setPhotos(Array.from(e.target.files ?? []))} /></label>
              <label className="upload-zone"><FileAudio size={24} /><strong>Voice observation</strong><span>{voice.length ? `${voice.length} audio file selected` : 'Optional · transcription can follow'}</span><input type="file" accept="audio/*" multiple onChange={(e) => setVoice(Array.from(e.target.files ?? []))} /></label>
            </div>
          </Panel>

          <Panel title="Monitor attestation" description="The server must verify the signature against your provisioned identity during secure submission.">
            <div className="field"><label htmlFor="identity-did">IOTA-linked DID <span className="optional">If already provisioned</span></label><input id="identity-did" className="mono" value={form.identityDid} onChange={(e) => update('identityDid', e.target.value)} placeholder="did:iota:testnet:…" /></div>
            <label className="attestation-check"><input type="checkbox" checked={attested} onChange={(e) => setAttested(e.target.checked)} /><span><strong>I attest that this record and its evidence accurately represent my field observation.</strong><small>No anchored or independently verified status is claimed by saving.</small></span></label>
            <FormError message={error} />
            {notice && <InlineNotice tone="success" title="Record preserved">{notice}</InlineNotice>}
            <div className="capture-submit"><span className="mono">Client ID · {clientId.slice(0, 13)}…</span><Button type="submit" icon={Save} loading={saving}>{saving ? 'Hashing evidence…' : 'Save on device'}</Button></div>
          </Panel>
        </form>

        <aside className="queue-column">
          <Panel title="Offline queue" description={`${queue.length} local ${queue.length === 1 ? 'record' : 'records'} · status is never inferred from connectivity.`} action={<button className="icon-button" onClick={() => void refreshQueue()} aria-label="Refresh offline queue"><RefreshCw size={17} /></button>}>
            {queueLoading ? <Skeleton rows={4} label="Loading offline queue" /> : <>
              {queueError && <FormError message={queueError} />}
              {!queue.length && <StatePanel kind="empty" title="No records on this device"><p>Save a complete field observation to create an offline record with a unique client ID.</p></StatePanel>}
              <div className="queue-list">{queue.map((record) => <article className="queue-record" key={record.clientId}>
                <div className="queue-record-head"><div><strong>{record.payload.plotId}</strong><span>{formatDate(record.createdAt)}</span></div><StatusBadge status={record.status} /></div>
                <dl><div><dt>Activity</dt><dd>{record.payload.activityId}</dd></div><div><dt>Evidence</dt><dd>{record.evidence.length} hashed files</dd></div><div><dt>Observation</dt><dd>{record.payload.aliveCount} / {record.payload.plantedCount} alive</dd></div></dl>
                {record.error && <p className="record-error">{record.error}</p>}
                {record.conflict && <p className="record-conflict">Server version {record.conflict.serverVersion}: {record.conflict.detail}</p>}
                <div className="queue-record-actions">
                  {['on_device', 'failed'].includes(record.status) && <Button variant="secondary" icon={CloudUpload} loading={syncing === record.clientId} disabled={!online || syncing === 'all'} onClick={() => void syncOne(record)}>{record.status === 'failed' ? 'Retry sync' : 'Synchronize'}</Button>}
                  {record.status === 'sync_conflict' && <Button variant="secondary" onClick={() => void correctedCopy(record)}>Create corrected copy</Button>}
                  {record.status === 'on_device' && <Button variant="quiet" icon={Trash2} onClick={() => void remove(record)}>Cancel draft</Button>}
                  {['submitted', 'awaiting_verification', 'anchoring', 'anchored', 'verification_failed'].includes(record.status) && <span className="queue-lock"><ShieldCheck size={14} />Server history preserved</span>}
                </div>
                <span className="record-client-id mono">{record.clientId}</span>
              </article>)}</div>
            </>}
          </Panel>
          <div className="queue-explainer"><h2>What each state means</h2><dl><div><dt><WifiOff size={15} />On device</dt><dd>Local only; not submitted.</dd></div><div><dt><CloudUpload size={15} />Submitted</dt><dd>Received by the server; not independently verified.</dd></div><div><dt><Check size={15} />Anchored</dt><dd>Chain readback confirmed by the backend.</dd></div></dl><p>{titleCase('sync_conflict')} requires an additive correction, never an overwrite.</p></div>
        </aside>
      </div>
    </>
  );
}
