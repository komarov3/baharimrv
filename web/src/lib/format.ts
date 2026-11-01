import type { DecimalAmount, EvidenceState } from '../types/api';
import { formatDecimal } from './decimal';

export function formatAmount(value: DecimalAmount): string {
  try {
    return `${value.currency} ${formatDecimal(value.amount, 2)}`;
  } catch {
    return `${value.currency} ${value.amount}`;
  }
}

export function formatQuantity(value: string, unit = 'tCO₂e'): string {
  try {
    return `${formatDecimal(value)} ${unit}`;
  } catch {
    return `${value} ${unit}`;
  }
}

export function formatDate(value?: string): string {
  if (!value) return 'Not recorded';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat('en-KE', { dateStyle: 'medium', timeStyle: 'short' }).format(date);
}

export function titleCase(value: string): string {
  return value.replaceAll('_', ' ').replace(/\b\w/g, (letter) => letter.toUpperCase());
}

// networkName labels the ledger the server reports, from either vocabulary the
// API uses (adapter mode or network). Unknown values never claim IOTA.
export function networkName(network: string): string {
  switch (network) {
    case 'iota_testnet':
    case 'testnet':
      return 'IOTA testnet';
    case 'iota_mainnet':
    case 'mainnet':
      return 'IOTA mainnet';
    case 'iota':
      return 'IOTA';
    default:
      return 'Sandbox ledger';
  }
}

export function statusTone(status: string): 'success' | 'warning' | 'danger' | 'neutral' | 'info' {
  if (['verified', 'anchored', 'active', 'complete', 'completed', 'confirmed', 'passed', 'valid', 'reconciled', 'registry_issued'].includes(status)) return 'success';
  if (['failed', 'verification_failed', 'rejected', 'invalid', 'revoked', 'critical', 'mismatch'].includes(status)) return 'danger';
  if (['pending', 'identity_pending', 'proof_pending', 'in_progress', 'uploading', 'anchoring', 'submitted', 'awaiting_verification', 'warning', 'reconciliation_pending', 'achieved_unverified', 'unverified'].includes(status)) return 'warning';
  if (['unavailable', 'on_device', 'not_applicable', 'estimated', 'claimed', 'superseded'].includes(status)) return 'neutral';
  if (['sandbox', 'measured', 'info', 'test', 'sync_conflict', 'disputed'].includes(status)) return 'info';
  return 'neutral';
}

export function evidenceLabel(status: EvidenceState | string): string {
  if (status === 'achieved_unverified') return 'Manager attested · unverified';
  if (status === 'verified') return 'Independently verified';
  return titleCase(status);
}
