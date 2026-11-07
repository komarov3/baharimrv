/**
 * BahariMRV IOTA Executor Service
 *
 * HTTP service that receives requests from the BahariMRV Go backend
 * (internal/chain/iota.go), builds Move PTBs for conservation_chain,
 * signs and submits them to IOTA Rebased Testnet, then returns the
 * transaction reference and created object ID.
 *
 * Supported operations:
 *   provision_identity        → identity::provision_identity
 *   anchor                    → notary::anchor_sponsored
 *   create_ledger             → project_ledger::create_ledger
 *   link_jahazi               → project_ledger::link_jahazi
 *   issue_contribution_cap    → project_ledger::issue_contribution_cap
 *   anchor_restoration_claim  → restoration::anchor_restoration_claim
 *   planting_record           → planting_record::create_planting_record
 *   planting_entry            → project_ledger::append_planting_entry
 *   claim_fulfillment         → restoration::record_fulfillment
 *   contribution_entry        → project_ledger::append_contribution_entry
 *   issue_certificate         → certificates::issue_certificate
 */

import express, { Request, Response, NextFunction } from 'express';
import { IotaClient, getFullnodeUrl } from '@iota/iota-sdk/client';
import { Transaction } from '@iota/iota-sdk/transactions';
import { Ed25519Keypair } from '@iota/iota-sdk/keypairs/ed25519';
import { fromB64 } from '@iota/iota-sdk/utils';
import { decodeIotaPrivateKey } from '@iota/iota-sdk/cryptography';
import * as dotenv from 'dotenv'; dotenv.config();

// ── Config ───────────────────────────────────────────────────────────────────

const PORT       = parseInt(process.env.PORT ?? '9000', 10);
const API_KEY    = process.env.API_KEY ?? '';
const RPC_URL    = process.env.IOTA_RPC_URL ?? 'https://api.testnet.iota.cafe';
// conservation_chain package (domain operations)
const PKG        = process.env.IOTA_PACKAGE_ID ?? '';
// bahari_identity package (shared identity layer — separate package)
const IDENTITY_PKG = process.env.BAHARI_IDENTITY_PACKAGE_ID ?? '';
const ANCHOR_REG = process.env.ANCHOR_REGISTRY_OBJECT_ID ?? '';
const IDENTITY_REG = process.env.IDENTITY_REGISTRY_OBJECT_ID ?? '';
const MRV_REG    = process.env.MRV_REGISTRY_OBJECT_ID ?? '';
const CARBON_REG = process.env.CARBON_CREDIT_REGISTRY_OBJECT_ID ?? '';
const ADMIN_CAP  = process.env.ADMIN_CAP_OBJECT_ID ?? '';
const GAS_BUDGET = parseInt(process.env.GAS_BUDGET ?? '50000000', 10);

function loadKeypair(): Ed25519Keypair {
  const key = process.env.SPONSOR_PRIVATE_KEY;
  if (!key) throw new Error('SPONSOR_PRIVATE_KEY not set');
  if (key.startsWith('iotaprivkey')) {
    const { secretKey } = decodeIotaPrivateKey(key);
    return Ed25519Keypair.fromSecretKey(secretKey);
  }
  return Ed25519Keypair.fromSecretKey(fromB64(key));
}

// ── IOTA client ───────────────────────────────────────────────────────────────

const client = new IotaClient({ url: RPC_URL });

// ── Express app ───────────────────────────────────────────────────────────────

const app = express();
app.use(express.json({ limit: '2mb' }));

// Auth middleware
app.use((req: Request, res: Response, next: NextFunction) => {
  if (req.path === '/health') return next();
  const key = req.headers['x-api-key'];
  if (!API_KEY || key === API_KEY) return next();
  res.status(401).json({ error: 'unauthorized' });
});

app.get('/health', (_req: Request, res: Response) => {
  res.json({
    status: 'ok',
    rpcUrl: RPC_URL,
    packageId: PKG,
    identityPackage:      IDENTITY_PKG  || '(not configured)',
    anchorRegistry:       ANCHOR_REG   || '(not configured)',
    identityRegistry:     IDENTITY_REG || '(not configured)',
    mrvRegistry:          MRV_REG      || '(not configured)',
    carbonCreditRegistry: CARBON_REG   || '(not configured)',
  });
});

app.post('/anchor', async (req: Request, res: Response) => {
  try {
    const { operation, package_id, ...params } = req.body as Record<string, any>;
    const pkg = package_id ?? PKG;
    if (!pkg) return res.status(400).json({ error: 'IOTA_PACKAGE_ID not configured' });

    const keypair = loadKeypair();
    let result: ExecuteResult;

    switch (operation) {
      case 'provision_identity':
        result = await provisionIdentity(keypair, pkg, params as any);
        break;
        break;
      case 'link_role_object':
        result = await linkRoleObject(keypair, params as any);
        break;
      case 'issue_manager_cap':
        result = await issueManagerCap(keypair, pkg, params as any);
        break;
      case 'issue_monitor_cap':
        result = await issueMonitorCap(keypair, pkg, params as any);
        break;
      case 'issue_verifier_cap':
        result = await issueVerifierCap(keypair, pkg, params as any);
        break;
      case 'anchor':
        result = await anchorProvenance(keypair, pkg, params as any);
      case 'create_ledger':
        result = await createLedger(keypair, pkg, params as any);
        break;
      case 'link_jahazi':
        result = await linkJahazi(keypair, pkg, params as any);
        break;
      case 'issue_contribution_cap':
        result = await issueContributionCap(keypair, pkg, params as any);
        break;
      case 'anchor_restoration_claim':
        result = await anchorRestorationClaim(keypair, pkg, params as any);
        break;
      case 'planting_record':
        result = await createPlantingRecord(keypair, pkg, params as any);
        break;
      case 'planting_entry':
        result = await appendPlantingEntry(keypair, pkg, params as any);
        break;
      case 'claim_fulfillment':
        result = await recordFulfillment(keypair, pkg, params as any);
        break;
      case 'contribution_entry':
        result = await appendContributionEntry(keypair, pkg, params as any);
        break;
      case 'issue_certificate':
        result = await issueCertificate(keypair, pkg, params as any);
        break;
      // ── Carbon credit & MRV attestation operations ──────────────────────
      case 'issue_carbon_credit':
        result = await issueCarbonCredit(keypair, pkg, params as any);
        break;
      case 'transfer_carbon_credit':
        result = await transferCarbonCredit(keypair, pkg, params as any);
        break;
      case 'retire_carbon_credit':
        result = await retireCarbonCredit(keypair, pkg, params as any);
        break;
      case 'register_mrv_attestation_self':
        result = await registerMrvAttestationSelf(keypair, pkg, params as any);
        break;
      case 'register_mrv_attestation_manager':
        result = await registerMrvAttestationManager(keypair, pkg, params as any);
        break;
      case 'register_mrv_attestation_verifier':
        result = await registerMrvAttestationVerifier(keypair, pkg, params as any);
        break;
      default:
        return res.status(400).json({ error: `Unknown operation: ${operation}` });
    }

    res.json(result);
  } catch (err: any) {
    console.error('[executor] error:', err?.message ?? err);
    res.status(500).json({ error: err?.message ?? 'executor error' });
  }
});

// ── Types ────────────────────────────────────────────────────────────────────

interface ExecuteResult {
  txHash:       string;  // IOTA transaction digest
  objectId:     string;  // created/mutated object ID
  did?:         string;
  address?:     string;
  transaction_reference: string;
  did_document_reference?: string;
}

// ── Submit helper ─────────────────────────────────────────────────────────────

async function submitTx(keypair: Ed25519Keypair, tx: Transaction): Promise<ExecuteResult> {
  tx.setGasBudget(GAS_BUDGET);
  tx.setSender(keypair.getPublicKey().toIotaAddress());

  const bytes  = await tx.build({ client });
  const { signature } = await keypair.signTransaction(bytes);

  const result = await client.executeTransactionBlock({
    transactionBlock: bytes,
    signature,
    options: { showEffects: true, showObjectChanges: true },
  });

  const digest   = result.digest;
  const effects  = result.effects;
  const changes  = (result as any).objectChanges ?? [];

  if (effects?.status?.status !== 'success') {
    throw new Error(`Transaction failed: ${effects?.status?.error ?? 'unknown'}`);
  }

  // Find the first created object (the frozen record or shared object)
  const created = changes.find((c: any) => c.type === 'created');
  const objectId = created?.objectId ?? effects?.created?.[0]?.reference?.objectId ?? '';

  return {
    txHash:                 digest,
    objectId,
    transaction_reference:  digest,
  };
}

// ── Operation implementations ─────────────────────────────────────────────────

async function provisionIdentity(
  keypair: Ed25519Keypair, pkg: string,
  params: { idempotency_key: string; public_key: string; network: string; platform?: string }
): Promise<ExecuteResult> {
  // Identity operations target bahari_identity package, not conservation_chain
  const identityPkg = IDENTITY_PKG || pkg;
  if (!IDENTITY_REG) throw new Error('IDENTITY_REGISTRY_OBJECT_ID not set');

  const userIdBytes   = textToBytes(params.idempotency_key);
  const did           = `did:iota:testnet:${params.idempotency_key}`;
  const didBytes      = textToBytes(did);
  // public_key is stored as a hex-encoded String in the contract — pass as text, not raw bytes
  const pubKeyBytes   = textToBytes(params.public_key);
  // Platform defaults to "baharimrv"; callers (e.g. Jahazi) can override
  const platformBytes = textToBytes(params.platform ?? 'baharimrv');

  const tx = new Transaction();
  tx.moveCall({
    target:    `${identityPkg}::identity::provision_identity`,
    arguments: [
      tx.object(IDENTITY_REG),
      tx.pure.vector('u8', userIdBytes),
      tx.pure.vector('u8', didBytes),
      tx.pure.vector('u8', pubKeyBytes),
      tx.pure.vector('u8', platformBytes),
    ],
  });

  const res = await submitTx(keypair, tx);
  return { ...res, did, address: keypair.getPublicKey().toIotaAddress(), did_document_reference: res.objectId };
}

async function anchorProvenance(
  keypair: Ed25519Keypair, pkg: string,
  params: {
    idempotency_key: string; kind: string; commitment: string;
    issuer_did: string; previous_object_id?: string;
  }
): Promise<ExecuteResult> {
  if (!ANCHOR_REG) throw new Error('ANCHOR_REGISTRY_OBJECT_ID not set');

  const tx = new Transaction();
  const prevOpt = params.previous_object_id
    ? tx.pure.option('id', params.previous_object_id)
    : tx.pure.option('id', null);

  tx.moveCall({
    target: `${pkg}::notary::anchor_sponsored`,
    arguments: [
      tx.object(ANCHOR_REG),
      tx.pure.vector('u8', textToBytes(params.idempotency_key)),
      tx.pure.vector('u8', textToBytes(params.kind)),
      tx.pure.vector('u8', textToBytes(params.commitment)),
      tx.pure.vector('u8', textToBytes(params.issuer_did)),
      prevOpt,
    ],
  });

  return submitTx(keypair, tx);
}

async function createLedger(
  keypair: Ed25519Keypair, pkg: string,
  params: { project_did: string; mrv_project_id: string }
): Promise<ExecuteResult> {
  const tx = new Transaction();
  const clock = tx.object('0x6'); // IOTA system clock object

  tx.moveCall({
    target: `${pkg}::project_ledger::create_ledger`,
    arguments: [
      tx.pure.vector('u8', textToBytes(params.project_did)),
      tx.pure.vector('u8', textToBytes(params.mrv_project_id)),
      clock,
    ],
  });

  return submitTx(keypair, tx);
}

async function linkJahazi(
  keypair: Ed25519Keypair, pkg: string,
  params: { ledger_object_id: string; jahazi_project_did: string }
): Promise<ExecuteResult> {
  const tx = new Transaction();
  const clock = tx.object('0x6');

  tx.moveCall({
    target: `${pkg}::project_ledger::link_jahazi`,
    arguments: [
      tx.object(params.ledger_object_id),
      tx.pure.vector('u8', textToBytes(params.jahazi_project_did)),
      clock,
    ],
  });

  return submitTx(keypair, tx);
}

async function issueContributionCap(
  keypair: Ed25519Keypair, pkg: string,
  params: { recipient: string; ledger_object_id: string; jahazi_platform_did: string }
): Promise<ExecuteResult> {
  const tx = new Transaction();

  tx.moveCall({
    target: `${pkg}::project_ledger::issue_contribution_cap`,
    arguments: [
      tx.pure.address(params.recipient),
      tx.object(params.ledger_object_id),
      tx.pure.vector('u8', textToBytes(params.jahazi_platform_did)),
    ],
  });

  return submitTx(keypair, tx);
}

async function anchorRestorationClaim(
  keypair: Ed25519Keypair, pkg: string,
  params: {
    jahazi_claim_id: string; jahazi_project_did: string; mrv_project_did: string;
    period_start_ms: number; period_end_ms: number; total_cost_cents: number;
    total_units: number; species: string; contributor_count: number;
    contributor_merkle_root: string; location_hash: string;
    jahazi_platform_did: string; nonce: string;
  }
): Promise<ExecuteResult> {
  const tx = new Transaction();
  const clock = tx.object('0x6');

  tx.moveCall({
    target: `${pkg}::restoration::anchor_restoration_claim`,
    arguments: [
      tx.pure.vector('u8', textToBytes(params.jahazi_claim_id)),
      tx.pure.vector('u8', textToBytes(params.jahazi_project_did)),
      tx.pure.vector('u8', textToBytes(params.mrv_project_did)),
      tx.pure.u64(BigInt(params.period_start_ms)),
      tx.pure.u64(BigInt(params.period_end_ms)),
      tx.pure.u64(BigInt(params.total_cost_cents)),
      tx.pure.u64(BigInt(params.total_units)),
      tx.pure.vector('u8', textToBytes(params.species)),
      tx.pure.u64(BigInt(params.contributor_count)),
      tx.pure.vector('u8', textToBytes(params.contributor_merkle_root)),
      tx.pure.vector('u8', textToBytes(params.location_hash)),
      tx.pure.vector('u8', textToBytes(params.jahazi_platform_did)),
      tx.pure.vector('u8', textToBytes(params.nonce)),
      clock,
    ],
  });

  return submitTx(keypair, tx);
}

async function createPlantingRecord(
  keypair: Ed25519Keypair, pkg: string,
  params: {
    project_did: string; ledger_object_id: string;
    provenance_anchor_id: string; claim_anchor_id?: string;
    species: string; seedlings_planted: number; seedlings_alive: number;
    survival_pct_hundredths: number; area_ha_millionths: number;
    methodology_version: string; period_start_ms: number; period_end_ms: number;
    evidence_root: string; satellite_corrob_hash?: string;
    photo_hash?: string; location_hash?: string;
    verifier_did?: string; verifier_attestation_hash?: string;
    is_verified: boolean; previous_record_id?: string;
    mrv_activity_id: string;
  }
): Promise<ExecuteResult> {
  const tx = new Transaction();
  const clock = tx.object('0x6');

  const claimOpt = params.claim_anchor_id
    ? tx.pure.option('id', params.claim_anchor_id)
    : tx.pure.option('id', null);

  const prevOpt = params.previous_record_id
    ? tx.pure.option('id', params.previous_record_id)
    : tx.pure.option('id', null);

  tx.moveCall({
    target: `${pkg}::planting_record::create_planting_record`,
    arguments: [
      tx.pure.vector('u8', textToBytes(params.project_did)),
      tx.pure.id(params.ledger_object_id),
      tx.pure.id(params.provenance_anchor_id),
      claimOpt,
      tx.pure.vector('u8', textToBytes(params.species)),
      tx.pure.u64(BigInt(params.seedlings_planted)),
      tx.pure.u64(BigInt(params.seedlings_alive)),
      tx.pure.u64(BigInt(params.survival_pct_hundredths)),
      tx.pure.u64(BigInt(params.area_ha_millionths)),
      tx.pure.vector('u8', textToBytes(params.methodology_version)),
      tx.pure.u64(BigInt(params.period_start_ms)),
      tx.pure.u64(BigInt(params.period_end_ms)),
      tx.pure.vector('u8', textToBytes(params.evidence_root)),
      tx.pure.vector('u8', textToBytes(params.satellite_corrob_hash ?? '')),
      tx.pure.vector('u8', textToBytes(params.photo_hash ?? '')),
      tx.pure.vector('u8', textToBytes(params.location_hash ?? '')),
      tx.pure.vector('u8', textToBytes(params.verifier_did ?? '')),
      tx.pure.vector('u8', textToBytes(params.verifier_attestation_hash ?? '')),
      tx.pure.bool(params.is_verified),
      prevOpt,
      tx.pure.vector('u8', textToBytes(params.mrv_activity_id)),
      clock,
    ],
  });

  return submitTx(keypair, tx);
}

async function appendPlantingEntry(
  keypair: Ed25519Keypair, pkg: string,
  params: {
    ledger_object_id: string; planting_record_id: string;
    provenance_anchor_id: string; seedlings_added: number;
    alive_added: number; co2e_mg_added: number;
  }
): Promise<ExecuteResult> {
  const tx = new Transaction();
  const clock = tx.object('0x6');

  tx.moveCall({
    target: `${pkg}::project_ledger::append_planting_entry`,
    arguments: [
      tx.object(params.ledger_object_id),
      tx.pure.id(params.planting_record_id),
      tx.pure.id(params.provenance_anchor_id),
      tx.pure.u64(BigInt(params.seedlings_added)),
      tx.pure.u64(BigInt(params.alive_added)),
      tx.pure.u64(BigInt(params.co2e_mg_added)),
      clock,
    ],
  });

  return submitTx(keypair, tx);
}

async function recordFulfillment(
  keypair: Ed25519Keypair, pkg: string,
  params: {
    claim_anchor_id: string; planting_record_id: string;
    provenance_anchor_id: string; actual_units: number;
    claimed_units: number; mrv_project_did: string;
    certificates_issued_count: number;
  }
): Promise<ExecuteResult> {
  const tx = new Transaction();
  const clock = tx.object('0x6');

  tx.moveCall({
    target: `${pkg}::restoration::record_fulfillment`,
    arguments: [
      tx.pure.id(params.claim_anchor_id),
      tx.pure.id(params.planting_record_id),
      tx.pure.id(params.provenance_anchor_id),
      tx.pure.u64(BigInt(params.actual_units)),
      tx.pure.u64(BigInt(params.claimed_units)),
      tx.pure.vector('u8', textToBytes(params.mrv_project_did)),
      tx.pure.u64(BigInt(params.certificates_issued_count)),
      clock,
    ],
  });

  return submitTx(keypair, tx);
}

async function appendContributionEntry(
  keypair: Ed25519Keypair, pkg: string,
  params: {
    contribution_cap_object_id: string; ledger_object_id: string;
    claim_anchor_id: string; amount_cents: number; contributor_count: number;
  }
): Promise<ExecuteResult> {
  const tx = new Transaction();
  const clock = tx.object('0x6');

  tx.moveCall({
    target: `${pkg}::project_ledger::append_contribution_entry`,
    arguments: [
      tx.object(params.contribution_cap_object_id),
      tx.object(params.ledger_object_id),
      tx.pure.id(params.claim_anchor_id),
      tx.pure.u64(BigInt(params.amount_cents)),
      tx.pure.u64(BigInt(params.contributor_count)),
      clock,
    ],
  });

  return submitTx(keypair, tx);
}

async function issueCertificate(
  keypair: Ed25519Keypair, pkg: string,
  params: {
    recipient: string; certificate_code: string;
    holder_name: string; holder_did: string;
    project_name: string; project_did: string;
    species: string; seedlings_funded: number;
    co2e_mg_5yr: number; planting_date_ms: number;
    planting_record_id: string; claim_anchor_id?: string;
    provenance_anchor_id: string; fulfillment_record_id?: string;
    funding_status: string; anchoring_status: string;
  }
): Promise<ExecuteResult> {
  const tx = new Transaction();
  const clock = tx.object('0x6');

  const claimOpt = params.claim_anchor_id
    ? tx.pure.option('id', params.claim_anchor_id)
    : tx.pure.option('id', null);

  const fulfilOpt = params.fulfillment_record_id
    ? tx.pure.option('id', params.fulfillment_record_id)
    : tx.pure.option('id', null);

  tx.moveCall({
    target: `${pkg}::certificates::issue_certificate`,
    arguments: [
      tx.pure.address(params.recipient),
      tx.pure.vector('u8', textToBytes(params.certificate_code)),
      tx.pure.vector('u8', textToBytes(params.holder_name)),
      tx.pure.vector('u8', textToBytes(params.holder_did)),
      tx.pure.vector('u8', textToBytes(params.project_name)),
      tx.pure.vector('u8', textToBytes(params.project_did)),
      tx.pure.vector('u8', textToBytes(params.species)),
      tx.pure.u64(BigInt(params.seedlings_funded)),
      tx.pure.u64(BigInt(params.co2e_mg_5yr)),
      tx.pure.u64(BigInt(params.planting_date_ms)),
      tx.pure.id(params.planting_record_id),
      claimOpt,
      tx.pure.id(params.provenance_anchor_id),
      fulfilOpt,
      tx.pure.vector('u8', textToBytes(params.funding_status)),
      tx.pure.vector('u8', textToBytes(params.anchoring_status)),
      clock,
    ],
  });

  return submitTx(keypair, tx);
}

// ── Identity role linking (bahari_identity package) ──────────────────────────

async function linkRoleObject(
  keypair: Ed25519Keypair,
  params: { doc_object_id: string; role_object_id: string }
): Promise<ExecuteResult> {
  const identityPkg = IDENTITY_PKG || PKG;
  if (!identityPkg) throw new Error('BAHARI_IDENTITY_PACKAGE_ID not set');
  const tx = new Transaction();
  tx.moveCall({
    target: `${identityPkg}::identity::link_role_object`,
    arguments: [
      tx.object(params.doc_object_id),
      tx.pure.id(params.role_object_id),
    ],
  });
  return submitTx(keypair, tx);
}

// ── Cap issuance (conservation_chain package) ─────────────────────────────────

async function issueManagerCap(
  keypair: Ed25519Keypair, pkg: string,
  params: { recipient: string; manager_did: string; project_id: string }
): Promise<ExecuteResult> {
  if (!ADMIN_CAP) throw new Error('ADMIN_CAP_OBJECT_ID not set');
  const tx = new Transaction();
  tx.moveCall({
    target: `${pkg}::roles::issue_manager_cap`,
    arguments: [
      tx.object(ADMIN_CAP),
      tx.pure.address(params.recipient),
      tx.pure.vector('u8', textToBytes(params.manager_did)),
      tx.pure.vector('u8', textToBytes(params.project_id)),
    ],
  });
  return submitTx(keypair, tx);
}

async function issueMonitorCap(
  keypair: Ed25519Keypair, pkg: string,
  params: { recipient: string; monitor_did: string; project_id: string; plot_id: string }
): Promise<ExecuteResult> {
  if (!ADMIN_CAP) throw new Error('ADMIN_CAP_OBJECT_ID not set');
  const tx = new Transaction();
  tx.moveCall({
    target: `${pkg}::roles::issue_monitor_cap`,
    arguments: [
      tx.object(ADMIN_CAP),
      tx.pure.address(params.recipient),
      tx.pure.vector('u8', textToBytes(params.monitor_did)),
      tx.pure.vector('u8', textToBytes(params.project_id)),
      tx.pure.vector('u8', textToBytes(params.plot_id)),
    ],
  });
  return submitTx(keypair, tx);
}

async function issueVerifierCap(
  keypair: Ed25519Keypair, pkg: string,
  params: { recipient: string; verifier_did: string; scope: string }
): Promise<ExecuteResult> {
  if (!ADMIN_CAP) throw new Error('ADMIN_CAP_OBJECT_ID not set');
  const tx = new Transaction();
  tx.moveCall({
    target: `${pkg}::roles::issue_verifier_cap`,
    arguments: [
      tx.object(ADMIN_CAP),
      tx.pure.address(params.recipient),
      tx.pure.vector('u8', textToBytes(params.verifier_did)),
      tx.pure.vector('u8', textToBytes(params.scope)),
    ],
  });
  return submitTx(keypair, tx);
}

// ── Carbon Credit & MRV Attestation ──────────────────────────────────────────
async function issueCarbonCredit(
  keypair: Ed25519Keypair, pkg: string,
  params: {
    recipient: string;
    conservation_record_id: string;
    activity_type: string;
    earner_did: string;
    kg_co2e: number;
    vintage_year: number;
    provenance_anchor_id?: string;   // optional: link to existing ProvenanceAnchor
    provenance_tx_digest?: string;
    standard: string;
  }
): Promise<ExecuteResult> {
  const carbonReg = CARBON_REG;
  if (!carbonReg) throw new Error('CARBON_CREDIT_REGISTRY_OBJECT_ID not configured');

  const tx = new Transaction();
  const clock = tx.object('0x6');
  const adminCap = process.env.ADMIN_CAP_OBJECT_ID;
  if (!adminCap) throw new Error('ADMIN_CAP_OBJECT_ID not configured');

  const anchorOpt = params.provenance_anchor_id
    ? tx.pure.option('id', params.provenance_anchor_id)
    : tx.pure.option('id', null);

  tx.moveCall({
    target: `${pkg}::carbon_credit::issue_credit`,
    arguments: [
      tx.object(carbonReg),
      tx.object(adminCap),
      tx.pure.address(params.recipient),
      tx.pure.vector('u8', textToBytes(params.conservation_record_id)),
      tx.pure.vector('u8', textToBytes(params.activity_type)),
      tx.pure.vector('u8', textToBytes(params.earner_did)),
      tx.pure.u64(BigInt(params.kg_co2e)),
      tx.pure.u64(BigInt(params.vintage_year)),
      anchorOpt,
      tx.pure.vector('u8', textToBytes(params.provenance_tx_digest ?? '')),
      tx.pure.vector('u8', textToBytes(params.standard)),
      clock,
    ],
  });
  return submitTx(keypair, tx);
}

async function transferCarbonCredit(
  keypair: Ed25519Keypair, pkg: string,
  params: {
    credit_object_id: string;
    new_holder: string;
  }
): Promise<ExecuteResult> {
  const tx = new Transaction();
  tx.moveCall({
    target: `${pkg}::carbon_credit::transfer_credit`,
    arguments: [
      tx.object(params.credit_object_id),
      tx.pure.address(params.new_holder),
    ],
  });
  return submitTx(keypair, tx);
}

async function retireCarbonCredit(
  keypair: Ed25519Keypair, pkg: string,
  params: {
    credit_object_id: string;
    registry_object_id?: string;   // defaults to CARBON_CREDIT_REGISTRY_OBJECT_ID
    reason: string;
  }
): Promise<ExecuteResult> {
  const registry = params.registry_object_id ?? CARBON_REG;
  if (!registry) throw new Error('CARBON_CREDIT_REGISTRY_OBJECT_ID not configured');
  const tx = new Transaction();
  tx.moveCall({
    target: `${pkg}::carbon_credit::retire_credit`,
    arguments: [
      tx.object(registry),
      tx.object(params.credit_object_id),
      tx.pure.vector('u8', textToBytes(params.reason)),
    ],
  });
  return submitTx(keypair, tx);
}

// ── MRV Attestation helpers ───────────────────────────────────────────────────

function buildMrvArgs(
  tx: Transaction, pkg: string,
  params: {
    record_id: string;
    record_type: string;
    actor_did: string;
    anchor_object_id?: string;
    anchor_tx_digest?: string;
    commitment: string;
    weight_grams?: number;
    activity_type?: string;
    attested_at_ms: number;
  }
) {
  const anchorOpt = params.anchor_object_id
    ? tx.pure.option('id', params.anchor_object_id)
    : tx.pure.option('id', null);
  const weightOpt = params.weight_grams !== undefined
    ? tx.pure.option('u64', BigInt(params.weight_grams))
    : tx.pure.option('u64', null);

  return [
    tx.pure.vector('u8', textToBytes(params.record_id)),
    tx.pure.vector('u8', textToBytes(params.record_type)),
    tx.pure.vector('u8', textToBytes(params.actor_did)),
    anchorOpt,
    tx.pure.vector('u8', textToBytes(params.anchor_tx_digest ?? '')),
    tx.pure.vector('u8', textToBytes(params.commitment)),
    weightOpt,
    tx.pure.vector('u8', textToBytes(params.activity_type ?? '')),
    tx.pure.u64(BigInt(params.attested_at_ms)),
  ];
}

async function registerMrvAttestationSelf(
  keypair: Ed25519Keypair, pkg: string,
  params: {
    record_id: string; record_type: string; actor_did: string;
    anchor_object_id?: string; anchor_tx_digest?: string;
    commitment: string; weight_grams?: number;
    activity_type?: string; attested_at_ms: number;
  }
): Promise<ExecuteResult> {
  if (!MRV_REG) throw new Error('MRV_REGISTRY_OBJECT_ID not configured');
  const tx = new Transaction();
  tx.moveCall({
    target: `${pkg}::mrv_registry::register_self_attestation`,
    arguments: [tx.object(MRV_REG), ...buildMrvArgs(tx, pkg, params)],
  });
  return submitTx(keypair, tx);
}

async function registerMrvAttestationManager(
  keypair: Ed25519Keypair, pkg: string,
  params: {
    manager_cap_object_id: string;
    record_id: string; record_type: string; actor_did: string;
    anchor_object_id?: string; anchor_tx_digest?: string;
    commitment: string; weight_grams?: number;
    activity_type?: string; attested_at_ms: number;
  }
): Promise<ExecuteResult> {
  if (!MRV_REG) throw new Error('MRV_REGISTRY_OBJECT_ID not configured');
  const tx = new Transaction();
  tx.moveCall({
    target: `${pkg}::mrv_registry::register_manager_attestation`,
    arguments: [
      tx.object(MRV_REG),
      tx.object(params.manager_cap_object_id),
      ...buildMrvArgs(tx, pkg, params),
    ],
  });
  return submitTx(keypair, tx);
}

async function registerMrvAttestationVerifier(
  keypair: Ed25519Keypair, pkg: string,
  params: {
    verifier_cap_object_id: string;
    record_id: string; record_type: string; actor_did: string;
    anchor_object_id?: string; anchor_tx_digest?: string;
    commitment: string; weight_grams?: number;
    activity_type?: string; attested_at_ms: number;
  }
): Promise<ExecuteResult> {
  if (!MRV_REG) throw new Error('MRV_REGISTRY_OBJECT_ID not configured');
  const tx = new Transaction();
  tx.moveCall({
    target: `${pkg}::mrv_registry::register_verifier_attestation`,
    arguments: [
      tx.object(MRV_REG),
      tx.object(params.verifier_cap_object_id),
      ...buildMrvArgs(tx, pkg, params),
    ],
  });
  return submitTx(keypair, tx);
}

// ── Util ─────────────────────────────────────────────────────────────────────

function textToBytes(s: string): number[] {
  return Array.from(new TextEncoder().encode(s));
}

function hexToBytes(hex: string): number[] {
  const clean = hex.replace(/^0x/, '');
  const bytes: number[] = [];
  for (let i = 0; i < clean.length; i += 2) {
    bytes.push(parseInt(clean.slice(i, i + 2), 16));
  }
  return bytes;
}

// ── Start ─────────────────────────────────────────────────────────────────────

app.listen(PORT, () => {
  console.log(`[executor] BahariMRV IOTA executor listening on :${PORT}`);
  console.log(`[executor] RPC: ${RPC_URL}`);
  console.log(`[executor] Package: ${PKG || '(not configured)'}`);
  console.log(`[executor] AnchorRegistry:       ${ANCHOR_REG || '(not configured)'}`);
  console.log(`[executor] IdentityRegistry:     ${IDENTITY_REG || '(not configured)'}`);
  console.log(`[executor] MRVRegistry:           ${MRV_REG || '(not configured)'}`);
  console.log(`[executor] CarbonCreditRegistry: ${CARBON_REG || '(not configured)'}`);
  console.log(`[executor] AdminCap:             ${ADMIN_CAP || '(not configured)'}`);
  if (!process.env.SPONSOR_PRIVATE_KEY) {
    console.warn('[executor] WARNING: SPONSOR_PRIVATE_KEY not set — transactions will fail');
  }
});
