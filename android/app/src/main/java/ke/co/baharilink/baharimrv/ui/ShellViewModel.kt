package ke.co.baharilink.baharimrv.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import ke.co.baharilink.baharimrv.data.local.ProjectSelectionStore
import ke.co.baharilink.baharimrv.data.remote.NetworkMonitor
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.stateIn

@HiltViewModel
class ShellViewModel @Inject constructor(
    networkMonitor: NetworkMonitor,
    private val selection: ProjectSelectionStore,
) : ViewModel() {
    val online = networkMonitor.isOnline.stateIn(viewModelScope, SharingStarted.WhileSubscribed(5_000), true)
    val selectedProjectId = selection.selectedProjectId
}
