import java.net.URI
import java.util.Properties

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
    id("com.google.devtools.ksp")
    id("com.google.dagger.hilt.android")
    id("com.google.gms.google-services")
}

val localProperties = Properties().apply {
    val file = rootProject.file("local.properties")
    if (file.exists()) file.inputStream().use(::load)
}

fun config(name: String): String = localProperties.getProperty(name)?.trim().orEmpty()
fun quoted(value: String): String = "\"${value.replace("\\", "\\\\").replace("\"", "\\\"")}\""
fun redirectPart(part: (URI) -> String?, fallback: String): String = runCatching {
    config("JAHAZI_REDIRECT_URI").takeIf { it.isNotBlank() }?.let(::URI)?.let(part)
}.getOrNull()?.takeIf { it.isNotBlank() } ?: fallback

android {
    namespace = "ke.co.baharilink.baharimrv"
    compileSdk = 35

    defaultConfig {
        applicationId = "ke.co.baharilink.baharimrv"
        minSdk = 26
        targetSdk = 35
        versionCode = 1
        versionName = "0.1.0"
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"

        val redirectUri = config("JAHAZI_REDIRECT_URI")
        val authConfigured = listOf(
            config("BAHARIMRV_API_BASE_URL"),
            config("JAHAZI_ISSUER_URL"),
            config("JAHAZI_CLIENT_ID"),
            redirectUri,
        ).all { it.isNotBlank() }

        buildConfigField("String", "API_BASE_URL", quoted(config("BAHARIMRV_API_BASE_URL")))
        buildConfigField("String", "JAHAZI_ISSUER_URL", quoted(config("JAHAZI_ISSUER_URL")))
        buildConfigField("String", "JAHAZI_CLIENT_ID", quoted(config("JAHAZI_CLIENT_ID")))
        buildConfigField("String", "JAHAZI_REDIRECT_URI", quoted(redirectUri))
        buildConfigField("boolean", "AUTH_CONFIGURED", authConfigured.toString())

        manifestPlaceholders["ssoScheme"] = redirectPart(URI::getScheme, "https")
        manifestPlaceholders["ssoHost"] = redirectPart(URI::getHost, "callback.invalid")
        manifestPlaceholders["ssoPath"] = redirectPart(URI::getPath, "/auth/callback")
        manifestPlaceholders["googleMapsApiKey"] = config("GOOGLE_MAPS_API_KEY")

        ksp {
            arg("room.schemaLocation", "$projectDir/schemas")
            arg("room.incremental", "true")
            arg("room.generateKotlin", "true")
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions {
        jvmTarget = "17"
    }
    buildFeatures {
        compose = true
        buildConfig = true
    }
    packaging {
        resources.excludes += "/META-INF/{AL2.0,LGPL2.1}"
    }
    testOptions {
        unitTests.all { it.useJUnit() }
    }
}

dependencies {
    val composeBom = platform("androidx.compose:compose-bom:2024.12.01")
    implementation(composeBom)
    androidTestImplementation(composeBom)

    val firebaseBom = platform("com.google.firebase:firebase-bom:35.1.0")
    implementation(firebaseBom)

    implementation("com.google.firebase:firebase-analytics")
    implementation("com.google.firebase:firebase-crashlytics")
    implementation("com.google.firebase:firebase-config")

    implementation("androidx.core:core-ktx:1.15.0")
    implementation("androidx.activity:activity-compose:1.9.3")
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material:material-icons-extended")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.7")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.8.7")
    implementation("androidx.navigation:navigation-compose:2.8.5")
    implementation("androidx.browser:browser:1.8.0")
    implementation("androidx.datastore:datastore-preferences:1.1.1")
    implementation("androidx.security:security-crypto:1.1.0-alpha06")

    implementation("com.google.dagger:hilt-android:2.54")
    ksp("com.google.dagger:hilt-compiler:2.54")
    implementation("androidx.hilt:hilt-navigation-compose:1.2.0")
    implementation("androidx.hilt:hilt-work:1.2.0")
    ksp("androidx.hilt:hilt-compiler:1.2.0")

    implementation("androidx.room:room-runtime:2.6.1")
    implementation("androidx.room:room-ktx:2.6.1")
    ksp("androidx.room:room-compiler:2.6.1")

    implementation("androidx.work:work-runtime-ktx:2.10.0")
    implementation("com.squareup.retrofit2:retrofit:2.11.0")
    implementation("com.squareup.retrofit2:converter-gson:2.11.0")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation("com.google.android.gms:play-services-location:21.3.0")
    implementation("com.google.maps.android:maps-compose:4.4.1")
    implementation("com.google.android.gms:play-services-maps:18.2.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.9.0")

    testImplementation("junit:junit:4.13.2")
    testImplementation("org.jetbrains.kotlinx:kotlinx-coroutines-test:1.9.0")
    debugImplementation("androidx.compose.ui:ui-tooling")
}
