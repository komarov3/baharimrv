# BahariMRV ProGuard Rules
# Applied when isMinifyEnabled = true in release builds

# ============================================================================
# Retrofit & OkHttp
# ============================================================================

-keepattributes Signature, InnerClasses, EnclosingMethod
-keepattributes RuntimeVisibleAnnotations, RuntimeVisibleParameterAnnotations
-keepattributes *Annotation*

-keepclassmembers,allowstorage class * {
  @com.google.gson.annotations.SerializedName <fields>;
}

-keep,allowobfuscation,allowshrinking interface retrofit2.Call
-keep,allowobfuscation,allowshrinking class retrofit2.Response
-keep,allowobfuscation,allowshrinking class kotlin.coroutines.Continuation

-dontwarn okhttp3.**
-dontwarn okio.**
-keep class okhttp3.** { *; }
-keep interface okhttp3.** { *; }

# ============================================================================
# Room Database
# ============================================================================

-keep class * extends androidx.room.RoomDatabase
-keep @androidx.room.Entity class *
-keep @androidx.room.Dao class *
-dontwarn androidx.room.paging.**

# ============================================================================
# Hilt/Dagger
# ============================================================================

-keep class dagger.hilt.** { *; }
-keep class javax.inject.** { *; }
-keep class * extends dagger.hilt.android.internal.managers.ViewComponentManager$FragmentContextWrapper { *; }
-keepclassmembers class * {
  @javax.inject.* <fields>;
  @javax.inject.* <init>(...);
}

# ============================================================================
# BahariMRV DTOs and Models
# ============================================================================

# Keep all data transfer objects used with Retrofit/Gson
-keep class ke.co.baharilink.baharimrv.data.remote.** { *; }

# Keep Room entities and their fields
-keep class ke.co.baharilink.baharimrv.data.local.entity.** { *; }

# Keep domain models
-keep class ke.co.baharilink.baharimrv.domain.model.** { *; }

# ============================================================================
# Gson
# ============================================================================

-keepclassmembers,allowobfuscation class * {
  @com.google.gson.annotations.SerializedName <fields>;
}

# Keep generic signatures for Gson reflection
-keep class com.google.gson.reflect.TypeToken { *; }
-keep class * extends com.google.gson.reflect.TypeToken

# ============================================================================
# Kotlin Coroutines
# ============================================================================

-keepnames class kotlinx.coroutines.internal.MainDispatcherFactory {}
-keepnames class kotlinx.coroutines.CoroutineExceptionHandler {}
-keepclassmembers class kotlinx.coroutines.** {
    volatile <fields>;
}

# ============================================================================
# Google Play Services & Maps
# ============================================================================

-keep class com.google.android.gms.maps.** { *; }
-keep interface com.google.android.gms.maps.** { *; }
-dontwarn com.google.android.gms.**

# ============================================================================
# AndroidX & Jetpack
# ============================================================================

-keep class androidx.lifecycle.** { *; }
-keep class androidx.compose.** { *; }
-dontwarn androidx.compose.**

# ============================================================================
# Security & Encryption
# ============================================================================

-keep class androidx.security.crypto.** { *; }

# ============================================================================
# General Android
# ============================================================================

-keepclassmembers class * implements android.os.Parcelable {
  public static final android.os.Parcelable$Creator *;
}

-keepclassmembers enum * {
    public static **[] values();
    public static ** valueOf(java.lang.String);
}

