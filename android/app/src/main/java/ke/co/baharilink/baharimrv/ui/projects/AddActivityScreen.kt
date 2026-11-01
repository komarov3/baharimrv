package ke.co.baharilink.baharimrv.ui.projects

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp

@Composable
fun AddActivityScreen(
    projectId: String,
    onBack: () -> Unit,
) {
    var plotId by remember { mutableStateOf("") }
    var activityType by remember { mutableStateOf("mangrove_restoration") }
    var methodologyVersion by remember { mutableStateOf("1.0") }
    var start by remember { mutableStateOf("") }
    var end by remember { mutableStateOf("") }

    Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        Text("New activity", style = MaterialTheme.typography.titleLarge)
        OutlinedTextField(value = plotId, onValueChange = { plotId = it }, label = { Text("Plot ID") }, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(value = activityType, onValueChange = { activityType = it }, label = { Text("Activity type") }, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(value = methodologyVersion, onValueChange = { methodologyVersion = it }, label = { Text("Methodology version") }, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(value = start, onValueChange = { start = it }, label = { Text("Reporting start (ISO-8601)") }, modifier = Modifier.fillMaxWidth())
        OutlinedTextField(value = end, onValueChange = { end = it }, label = { Text("Reporting end (ISO-8601)") }, modifier = Modifier.fillMaxWidth())
        Button(onClick = onBack, modifier = Modifier.fillMaxWidth()) { Text("Save (wire to repository in next step)") }
        OutlinedButton(onClick = onBack, modifier = Modifier.fillMaxWidth()) { Text("Back") }
    }
}
