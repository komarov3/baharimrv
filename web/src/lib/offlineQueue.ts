import { api } from './api';
import type { MonitoringRecordPayload, MonitoringStatus } from '../types/api';

const DB_NAME = 'baharimrv-field-ledger';
const STORE = 'monitoring-queue';

export interface EvidenceDraft {
  name: string;
  kind: 'photo' | 'voice';
  mediaType: string;
  size: number;
  sha256: string;
  blob: Blob;
}

export interface QueuedMonitoringRecord {
  clientId: string;
  payload: MonitoringRecordPayload;
  evidence: EvidenceDraft[];
  status: MonitoringStatus;
  createdAt: string;
  updatedAt: string;
  monitoringRecordId?: string;
  error?: string;
  conflict?: { serverVersion: string; detail: string };
}

interface CaptureInput extends Omit<MonitoringRecordPayload, 'evidenceManifest'> {
  files: Array<{ file: File; kind: 'photo' | 'voice' }>;
}

function openDatabase(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    if (!('indexedDB' in window)) {
      reject(new Error('Offline device storage is unavailable in this browser. Your record has not been saved.'));
      return;
    }
    const request = indexedDB.open(DB_NAME, 1);
    request.onupgradeneeded = () => {
      const database = request.result;
      if (!database.objectStoreNames.contains(STORE)) database.createObjectStore(STORE, { keyPath: 'clientId' });
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(new Error('Offline storage could not be opened. Check browser storage permissions.'));
    request.onblocked = () => reject(new Error('Offline storage is blocked by another tab. Close other BahariMRV tabs and retry.'));
  });
}

async function withStore<T>(mode: IDBTransactionMode, operation: (store: IDBObjectStore) => IDBRequest<T>): Promise<T> {
  const database = await openDatabase();
  return new Promise<T>((resolve, reject) => {
    const transaction = database.transaction(STORE, mode);
    const request = operation(transaction.objectStore(STORE));
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(new Error('The offline record could not be updated. Your browser storage may be full.'));
    transaction.oncomplete = () => database.close();
    transaction.onerror = () => { database.close(); reject(new Error('Offline storage transaction failed. Retry without closing the tab.')); };
  });
}

export async function sha256(blob: Blob): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', await blob.arrayBuffer());
  return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('');
}

export async function saveCapture(input: CaptureInput): Promise<QueuedMonitoringRecord> {
  const evidence: EvidenceDraft[] = [];
  for (const { file, kind } of input.files) {
    evidence.push({
      name: file.name,
      kind,
      mediaType: file.type || 'application/octet-stream',
      size: file.size,
      sha256: await sha256(file),
      blob: file,
    });
  }
  const now = new Date().toISOString();
  const record: QueuedMonitoringRecord = {
    clientId: input.clientId,
    payload: {
      clientId: input.clientId,
      activityId: input.activityId,
      plotId: input.plotId,
      methodologyVersion: input.methodologyVersion,
      schemaVersion: input.schemaVersion,
      deviceTimestamp: input.deviceTimestamp,
      location: input.location,
      aliveCount: input.aliveCount,
      plantedCount: input.plantedCount,
      averageHeightMetres: input.averageHeightMetres,
      species: input.species,
      observations: input.observations,
      attestation: input.attestation,
      evidenceManifest: evidence.map(({ name, kind, mediaType, size, sha256: hash }) => ({ name, kind, mediaType, size, sha256: hash })),
    },
    evidence,
    status: 'on_device',
    createdAt: now,
    updatedAt: now,
  };
  await withStore('readwrite', (store) => store.put(record));
  return record;
}

export async function listCaptures(): Promise<QueuedMonitoringRecord[]> {
  const records = await withStore('readonly', (store) => store.getAll() as IDBRequest<QueuedMonitoringRecord[]>);
  return records.sort((a, b) => b.updatedAt.localeCompare(a.updatedAt));
}

export async function putCapture(record: QueuedMonitoringRecord): Promise<void> {
  await withStore('readwrite', (store) => store.put({ ...record, updatedAt: new Date().toISOString() }));
}

export async function deleteCapture(record: QueuedMonitoringRecord): Promise<void> {
  if (record.monitoringRecordId || !['on_device', 'failed', 'sync_conflict'].includes(record.status)) {
    throw new Error('Submitted records cannot be deleted. Create a corrected version instead.');
  }
  await withStore('readwrite', (store) => store.delete(record.clientId));
}

export async function syncCapture(record: QueuedMonitoringRecord): Promise<QueuedMonitoringRecord> {
  let current: QueuedMonitoringRecord = { ...record, status: 'uploading', error: undefined, conflict: undefined };
  await putCapture(current);
  try {
    if (!current.monitoringRecordId) {
      const response = await api.syncBatch([current.payload]);
      const result = response.results.find((item) => item.clientId === current.clientId);
      if (!result) throw new Error('The sync response did not include this record. Retry; the client ID prevents duplication.');
      if (result.status === 'sync_conflict') {
        current = { ...current, status: 'sync_conflict', conflict: result.conflict, error: result.error ?? 'The server has a newer version.' };
        await putCapture(current);
        return current;
      }
      if (result.status === 'failed' || result.status === 'verification_failed') {
        throw new Error(result.error ?? 'The server rejected this record. Review the values and retry.');
      }
      current = { ...current, monitoringRecordId: result.monitoringRecordId, status: result.status };
    }

    if (current.monitoringRecordId) {
      for (const evidence of current.evidence) {
        await api.uploadEvidence(current.monitoringRecordId, {
          file: evidence.blob,
          name: evidence.name,
          kind: evidence.kind,
          sha256: evidence.sha256,
        });
      }
    }
    if (current.status === 'on_device' || current.status === 'uploading') current.status = 'submitted';
    await putCapture(current);
    return current;
  } catch (error) {
    current = { ...current, status: 'failed', error: error instanceof Error ? error.message : 'Synchronization failed. Retry when connected.' };
    await putCapture(current);
    return current;
  }
}
