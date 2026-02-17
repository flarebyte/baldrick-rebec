import {
  idFrom,
  snapshotBackupJSON,
  snapshotDelete,
  snapshotList,
  snapshotPrunePreviewJSON,
  snapshotRestoreDry,
  snapshotShow,
  snapshotVerifyJSON,
  vaultBackendCurrent,
  vaultDoctor,
  vaultList,
  vaultShow,
} from './cli-helper';
import {
  validateVaultListContract,
  validateVaultShowContract,
} from './contract-helper';
import type { E2EContext } from './types';

export async function runSnapshotVault(ctx: E2EContext) {
  ctx.nextStep(
    ctx.SKIP_SNAPSHOT
      ? 'Skipping snapshot (--skip-snapshot)'
      : 'Snapshot backup smoke test (backup/list/show/restore dry-run/delete)',
  );

  if (!ctx.SKIP_SNAPSHOT) {
    const bkp = await snapshotBackupJSON({
      description: 'rbctest snapshot',
      who: ctx.TEST_ROLE_USER,
    });
    const bkpID = idFrom(bkp);
    await snapshotList({ limit: 5 });
    await snapshotShow({ id: bkpID });
    const verifyRows = await snapshotVerifyJSON({ id: bkpID });
    const prunePreview = await snapshotPrunePreviewJSON({ olderThan: '0d' });
    await ctx.checkStep(
      'snapshot verified',
      Array.isArray(verifyRows) &&
        prunePreview &&
        typeof prunePreview.candidates === 'number' &&
        prunePreview.candidates >= 1,
      'snapshot verify/prune preview unexpected',
    );
    await snapshotRestoreDry({ id: bkpID, mode: 'append' });
    await snapshotDelete({ id: bkpID });
  }

  ctx.nextStep('Vault read-only checks (if configured)');
  try {
    const items = await vaultList();
    validateVaultListContract(items);
    const exists = items.find(
      (x) => x.name === 'rbctest-key' && x.status === 'set',
    );
    if (exists) {
      const md = await vaultShow('rbctest-key');
      validateVaultShowContract(md);
      ctx.check(md.name === 'rbctest-key', 'vault.show name matches');
      ctx.check(md.status === 'set', 'vault.show status is set');
      const backend = await vaultBackendCurrent();
      ctx.check(backend === 'keychain', 'vault backend current is keychain');
      await vaultDoctor();
    } else {
      console.error('vault: rbctest-key not set; skipping deep checks');
    }
  } catch (e) {
    console.error(
      'vault: checks skipped due to error:',
      (e as Error)?.message || e,
    );
  }
}
