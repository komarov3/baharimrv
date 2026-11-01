package ke.co.baharilink.baharimrv.location

import android.content.Context
import android.media.MediaRecorder
import android.net.Uri
import android.os.Build
import java.io.File
import java.util.UUID
import ke.co.baharilink.baharimrv.data.local.EvidenceKind
import ke.co.baharilink.baharimrv.data.repository.EvidenceSelection

class AudioRecorder(private val context: Context) {
    private var recorder: MediaRecorder? = null
    private var output: File? = null

    fun start() {
        check(recorder == null) { "A voice observation is already recording" }
        val file = File(context.cacheDir, "voice-${UUID.randomUUID()}.m4a")
        val mediaRecorder = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) MediaRecorder(context) else MediaRecorder()
        mediaRecorder.apply {
            setAudioSource(MediaRecorder.AudioSource.MIC)
            setOutputFormat(MediaRecorder.OutputFormat.MPEG_4)
            setAudioEncoder(MediaRecorder.AudioEncoder.AAC)
            setAudioEncodingBitRate(96_000)
            setAudioSamplingRate(44_100)
            setOutputFile(file.absolutePath)
            prepare()
            start()
        }
        recorder = mediaRecorder
        output = file
    }

    fun stop(): EvidenceSelection {
        val mediaRecorder = recorder ?: error("No voice observation is recording")
        val file = output ?: error("The voice recording has no output file")
        try {
            mediaRecorder.stop()
        } finally {
            mediaRecorder.release()
            recorder = null
            output = null
        }
        return EvidenceSelection(
            uri = Uri.fromFile(file),
            kind = EvidenceKind.VOICE,
            originalFileName = file.name,
            mimeType = "audio/mp4",
            capturedAtEpochMs = System.currentTimeMillis(),
        )
    }

    fun cancel() {
        recorder?.runCatching { stop() }
        recorder?.release()
        recorder = null
        output?.delete()
        output = null
    }
}
