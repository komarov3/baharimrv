package ke.co.baharilink.baharimrv.di

import android.content.Context
import androidx.room.Room
import androidx.work.WorkManager
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import java.util.concurrent.TimeUnit
import javax.inject.Singleton
import ke.co.baharilink.baharimrv.BuildConfig
import ke.co.baharilink.baharimrv.data.local.BahariMrvDatabase
import ke.co.baharilink.baharimrv.data.local.SecureSessionStore
import ke.co.baharilink.baharimrv.data.remote.BahariMrvApi
import okhttp3.OkHttpClient
import okhttp3.Protocol
import retrofit2.Retrofit
import retrofit2.converter.gson.GsonConverterFactory

@Module
@InstallIn(SingletonComponent::class)
object AppModule {
    @Provides
    @Singleton
    fun database(@ApplicationContext context: Context): BahariMrvDatabase = Room.databaseBuilder(
        context,
        BahariMrvDatabase::class.java,
        "baharimrv.db",
    ).build()

    @Provides fun monitoringDao(database: BahariMrvDatabase) = database.monitoringDao()
    @Provides fun evidenceDao(database: BahariMrvDatabase) = database.evidenceDao()

    @Provides
    @Singleton
    fun workManager(@ApplicationContext context: Context): WorkManager = WorkManager.getInstance(context)

    @Provides
    @Singleton
    fun okHttp(secureSessionStore: SecureSessionStore): OkHttpClient = OkHttpClient.Builder()
        .protocols(listOf(Protocol.HTTP_1_1))  // Fix ngrok HTTP/2 compatibility issue
        .connectTimeout(30, TimeUnit.SECONDS)
        .readTimeout(45, TimeUnit.SECONDS)
        .writeTimeout(45, TimeUnit.SECONDS)
        .addInterceptor { chain ->
            val request = chain.request().newBuilder()
            secureSessionStore.accessToken()?.let { token ->
                request.header("Authorization", "Bearer $token")
            }
            chain.proceed(request.build())
        }
        .build()

    @Provides
    @Singleton
    fun retrofit(client: OkHttpClient): Retrofit {
        val configured = BuildConfig.API_BASE_URL.trim()
        val baseUrl = if (configured.isBlank()) "https://configuration.invalid/" else configured
        return Retrofit.Builder()
            .baseUrl(if (baseUrl.endsWith('/')) baseUrl else "$baseUrl/")
            .client(client)
            .addConverterFactory(GsonConverterFactory.create())
            .build()
    }

    @Provides fun api(retrofit: Retrofit): BahariMrvApi = retrofit.create(BahariMrvApi::class.java)
}
