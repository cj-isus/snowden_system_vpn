plugins {
    id("com.android.application")
    id("kotlin-android")
    // The Flutter Gradle Plugin must be applied after the Android and Kotlin Gradle plugins.
    id("dev.flutter.flutter-gradle-plugin")
}

android {
    namespace = "com.snowden.system.snowden_android"
    compileSdk = flutter.compileSdkVersion
    ndkVersion = flutter.ndkVersion

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = JavaVersion.VERSION_17.toString()
    }

    defaultConfig {
        applicationId = "com.snowden.system.snowden_android"
        minSdk = flutter.minSdkVersion
        targetSdk = flutter.targetSdkVersion
        versionCode = flutter.versionCode
        versionName = flutter.versionName
    }

    buildTypes {
        release {
            signingConfig = signingConfigs.getByName("debug")
        }
    }

    lint {
        // The MVP release flow intentionally skips lintVitalAnalyzeRelease:
        // resolve of com.android.tools.lint:lint-checks:31.11.1 from
        // dl.google.com is blocked by the same TLS-MITM layer that already
        // bit plugins.gradle.org / Maven Central, and the warnings Lint would
        // surface are local-style issues that don't affect runtime. Phase 2
        // re-enables this once a fully-routable Maven mirror is wired in.
        checkReleaseBuilds = false
        abortOnError = false
    }
}

dependencies {
    implementation(files("libs/libbox.aar"))
}

flutter {
    source = "../.."
}
