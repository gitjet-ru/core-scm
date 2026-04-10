#!/usr/bin/env bash
set -euo pipefail

# Usage:
#   tools/release-image-tag.sh --image registry.gitjet.ru/core-scm --source registry.gitjet.ru/core-scm:build-tag
#   tools/release-image-tag.sh --image registry.gitjet.ru/core-scm --source registry.gitjet.ru/core-scm:build-tag --version v0.0.3

IMAGE_REPO=""
SOURCE_REF=""
TARGET_VERSION=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --image)
      IMAGE_REPO="${2:-}"
      shift 2
      ;;
    --source)
      SOURCE_REF="${2:-}"
      shift 2
      ;;
    --version)
      TARGET_VERSION="${2:-}"
      shift 2
      ;;
    *)
      echo "Unknown arg: $1" >&2
      exit 1
      ;;
  esac
done

if [[ -z "$IMAGE_REPO" || -z "$SOURCE_REF" ]]; then
  echo "Required args: --image <repo> --source <image-ref>" >&2
  exit 1
fi

if [[ -z "$TARGET_VERSION" ]]; then
  # Try remote tags first (works if registry allows tags API for current credentials).
  # Expected output shape: {"name":"core-scm","tags":["v0.0.1","v0.0.2",...]}
  registry_host="${IMAGE_REPO%%/*}"
  repo_path="${IMAGE_REPO#*/}"
  remote_tags="$(curl -fsS "https://${registry_host}/v2/${repo_path}/tags/list" 2>/dev/null || true)"

  max_patch=0
  if [[ -n "$remote_tags" ]]; then
    while IFS= read -r tag; do
      [[ "$tag" =~ ^v0\.0\.([0-9]+)$ ]] || continue
      patch="${BASH_REMATCH[1]}"
      if (( patch > max_patch )); then
        max_patch="$patch"
      fi
    done < <(printf "%s" "$remote_tags" | tr '[],"' '\n')
  fi

  # Fallback to local images if remote tag listing is unavailable.
  if (( max_patch == 0 )); then
    while IFS= read -r full; do
      tag="${full##*:}"
      [[ "$tag" =~ ^v0\.0\.([0-9]+)$ ]] || continue
      patch="${BASH_REMATCH[1]}"
      if (( patch > max_patch )); then
        max_patch="$patch"
      fi
    done < <(docker images --format '{{.Repository}}:{{.Tag}}' "$IMAGE_REPO")
  fi

  next_patch=$((max_patch + 1))
  TARGET_VERSION="v0.0.${next_patch}"
fi

TARGET_REF="${IMAGE_REPO}:${TARGET_VERSION}"
echo "Tagging ${SOURCE_REF} -> ${TARGET_REF}"
docker tag "${SOURCE_REF}" "${TARGET_REF}"
docker push "${TARGET_REF}"
echo "PUSHED_RELEASE_IMAGE=${TARGET_REF}"
