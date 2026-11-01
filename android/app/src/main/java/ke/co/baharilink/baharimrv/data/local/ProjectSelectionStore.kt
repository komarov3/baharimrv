package ke.co.baharilink.baharimrv.data.local

import android.content.Context
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

private val Context.projectDataStore by preferencesDataStore(name = "project_selection")

@Singleton
class ProjectSelectionStore @Inject constructor(@ApplicationContext private val context: Context) {
    private val projectId = stringPreferencesKey("selected_project_id")
    val selectedProjectId: Flow<String> = context.projectDataStore.data.map { it[projectId].orEmpty() }

    suspend fun select(id: String) {
        context.projectDataStore.edit { it[projectId] = id }
    }
}
