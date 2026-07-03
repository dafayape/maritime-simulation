#!/usr/bin/env bash
# Build APK release dengan penamaan artefak standar CI/CD:
#   dist/maritim-node-v<versi>-build<build>-release.apk  (+ .sha256)
#
# Versi diambil dari satu sumber kebenaran: `version:` pada pubspec.yaml.
# Naikkan versi di sana + catat di CHANGELOG.md setiap rilis, maka nama
# artefak otomatis mengikuti — memudahkan developer melacak perubahan.
#
# Pemakaian:
#   tool/build_apk.sh                                  # build standar
#   tool/build_apk.sh --dart-define=FOO=bar            # argumen diteruskan
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="$(grep '^version:' pubspec.yaml | awk '{print $2}')"
SEMVER="${VERSION%+*}"
BUILD="${VERSION#*+}"

echo "==> Membangun Maritim Node v${SEMVER} (build ${BUILD})"
flutter build apk --release "$@"

mkdir -p dist
OUT="dist/maritim-node-v${SEMVER}-build${BUILD}-release.apk"
cp build/app/outputs/flutter-apk/app-release.apk "${OUT}"
( cd dist && sha256sum "$(basename "${OUT}")" > "$(basename "${OUT}").sha256" )

echo "==> Artefak siap:"
ls -lh "${OUT}" "${OUT}.sha256"
