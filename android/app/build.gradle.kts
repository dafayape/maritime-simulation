plugins {
    id("com.android.application")
    // The Flutter Gradle Plugin must be applied after the Android and Kotlin Gradle plugins.
    id("dev.flutter.flutter-gradle-plugin")
}

android {
    namespace = "com.maritimesimulation.node"
    compileSdk = flutter.compileSdkVersion
    ndkVersion = flutter.ndkVersion

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    defaultConfig {
        applicationId = "com.maritimesimulation.node"
        // versionCode/versionName mengikuti `version:` pada pubspec.yaml
        // (1.0.0+1 → versionName=1.0.0, versionCode=1) agar satu sumber
        // kebenaran versi untuk CI/CD dan CHANGELOG.
        minSdk = flutter.minSdkVersion
        targetSdk = flutter.targetSdkVersion
        versionCode = flutter.versionCode
        versionName = flutter.versionName
    }

    buildTypes {
        release {
            // Simulator internal: ditandatangani dengan debug key agar APK
            // dapat langsung dipasang untuk pengujian. Ganti dengan keystore
            // rilis bila didistribusikan lebih luas.
            signingConfig = signingConfigs.getByName("debug")
        }
    }
}

kotlin {
    compilerOptions {
        jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_17
    }
}

flutter {
    source = "../.."
}
