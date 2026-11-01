package ke.co.baharilink.baharimrv.sync

import android.content.Context
import androidx.hilt.work.HiltWorker
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import dagger.assisted.Assisted
import dagger.assisted.AssistedInject
import ke.co.baharilink.baharimrv.data.repository.MonitoringRepository
import ke.co.baharilink.baharimrv.data.repository.SyncRunResult

@HiltWorker
class SyncWorker @AssistedInject constructor(
    @Assisted context: Context,
    @Assisted parameters: WorkerParameters,
    private val repository: MonitoringRepository,
) : CoroutineWorker(context, parameters) {
    override suspend fun doWork(): Result = when (repository.syncPending()) {
        SyncRunResult.EMPTY, SyncRunResult.SUCCESS -> Result.success()
        SyncRunResult.RETRY -> if (runAttemptCount < 20) Result.retry() else Result.failure()
        SyncRunResult.FAILURE -> Result.failure()
    }
}
