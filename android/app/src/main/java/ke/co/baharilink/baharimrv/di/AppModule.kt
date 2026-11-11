package ke.co.baharilink.baharimrv.di

import android.content.Context
import androidx.room.Room
import androidx.work.WorkManager
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import java.io.File
import java.util.concurrent.TimeUnit
import javax.inject.Named
import javax.inject.Singleton
import ke.co.baharilink.baharimrv.BuildConfig
import ke.co.baharilink.baharimrv.data.local.BahariMrvDatabase
import ke.co.baharilink.baharimrv.data.local.SecureSessionStore
import ke.co.baharilink.baharimrv.data.remote.BahariMrvApi
import okhttp3.Cache
import okhttp3.Interceptor
import okhttp3.OkHttpClient
import okhttp3.Protocol
import retrofit2.Retrofit
import retrofit2.converter.gson.GsonConverterFactory

@Module
@InstallIn(SingletonComponent::class)
object AppModule {
    private const val TILES_CLIENT = "tiles"
    // 20 MB disk cache for satellite tiles. Spec section 10.2: recent zoom bands
    // survive offline, so the map degrades to cached imagery rather than a blank
    // pane when the link drops.
    private const val TILES_CACHE_BYTES = 20L * 1024 * 1024

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
    fun authInterceptor(secureSessionStore: SecureSessionStore): Interceptor = Interceptor { chain ->
        val request = chain.request().newBuilder()
        secureSessionStore.accessToken()?.let { token ->
            request.header("Authorization", "Bearer $token")
        }
        chain.proceed(request.build())
    }

    @Provides
    @Singleton
    fun okHttp(authInterceptor: Interceptor): OkHttpClient = OkHttpClient.Builder()
        .protocols(listOf(Protocol.HTTP_1_1))  // HTTP/1.1 for Fly.io compatibility
        .connectTimeout(30, TimeUnit.SECONDS)
        .readTimeout(45, TimeUnit.SECONDS)
        .writeTimeout(45, TimeUnit.SECONDS)
        .addInterceptor(authInterceptor)
        .build()

    /**
     * Dedicated tile client. Same bearer as the JSON client, but with a disk
     * cache so the last viewed zoom band survives offline, and a longer read
     * timeout because a tile fetch at z16 can take a moment on a cold cache.
     */
    @Provides
    @Singleton
    @Named(TILES_CLIENT)
    fun tilesOkHttp(
        @ApplicationContext context: Context,
        authInterceptor: Interceptor,
    ): OkHttpClient {
        val cacheDir = File(context.cacheDir, "satellite-tiles").apply { mkdirs() }
        return OkHttpClient.Builder()
            .protocols(listOf(Protocol.HTTP_1_1))
            .connectTimeout(30, TimeUnit.SECONDS)
            .readTimeout(60, TimeUnit.SECONDS)
            .cache(Cache(cacheDir, TILES_CACHE_BYTES))
            .addInterceptor(authInterceptor)
            .build()
    }

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
