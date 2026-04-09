#!/bin/sh
# Release bump script - updates version references in external repositories
# Called from CI with CI_COMMIT_TAG and SSH_KEY environment variables

set -ex
echo "Starting release-bump.sh"
echo "CI_COMMIT_TAG: ${CI_COMMIT_TAG:-<not set>}"

# Validate required environment variables
if [ -z "$CI_COMMIT_TAG" ]; then
    echo "Error: CI_COMMIT_TAG is required"
    exit 1
fi

if [ -z "$SSH_KEY" ]; then
    echo "Error: SSH_KEY is required"
    exit 1
fi

# Determine if this is a major release (breaking change)
VERSION=${CI_COMMIT_TAG#v}
MINOR=$(echo "$VERSION" | cut -d. -f2)
PATCH=$(echo "$VERSION" | cut -d. -f3)
BREAKING=""
[ "$MINOR" = "0" ] && [ "$PATCH" = "0" ] && BREAKING="!" || true

# Setup SSH
mkdir -p ~/.ssh && chmod 700 ~/.ssh
echo "$SSH_KEY" > ~/.ssh/id_ed25519 && chmod 600 ~/.ssh/id_ed25519
echo "Running ssh-keyscan for codefloe.com..."
ssh-keyscan -T 10 codefloe.com >> ~/.ssh/known_hosts || echo "Warning: ssh-keyscan failed, continuing anyway"
echo "SSH setup complete"

# Install prettier globally (repos may not have package.json)
npm install -g prettier

#
# Bump and push homebrew-tap
#
echo "=== Bumping homebrew-tap ==="
rm -rf homebrew-tap
git clone --depth 1 ssh://git@codefloe.com/crowci/homebrew-tap.git
cd homebrew-tap
TARBALL_URL="https://codefloe.com/crowci/crow/releases/download/${CI_COMMIT_TAG}/crow-src.tar.gz"
curl -sL -o crow-src.tar.gz "$TARBALL_URL"
SHA256=$(sha256sum crow-src.tar.gz | cut -d' ' -f1)
sed -i "s|url .*\.tar\.gz.*|url \"$TARBALL_URL\"|" Formula/crow.rb
sed -i "s/sha256 \"[^\"]*\"/sha256 \"$SHA256\"/" Formula/crow.rb
rm -f crow-src.tar.gz
git config user.name "crowci-bot"
git config user.email "admin@crowci.dev"
git add -A
git commit -m "chore${BREAKING}: bump version to ${CI_COMMIT_TAG}" || echo "Nothing to commit (already up to date)"
git push origin main || echo "Nothing to push"
cd ..

#
# Bump, update docs, and release crowci/helm
#
echo "=== Bumping crowci/helm ==="
rm -rf helm
git clone ssh://git@codefloe.com/crowci/helm.git
cd helm
sed -i "s/^appVersion: .*/appVersion: ${CI_COMMIT_TAG#v}/" charts/crow/Chart.yaml

# Regenerate chart docs
echo "Running helm-docs..."
export PATH="$PATH:$HOME/go/bin"
helm-docs --skip-version-footer

# Format with prettier
echo "Running prettier..."
prettier --write .

git config user.name "crowci-bot"
git config user.email "admin@crowci.dev"
git add -A
if git diff --cached --quiet; then
    echo "Nothing to commit (helm already at ${CI_COMMIT_TAG})"
else
    git commit -m "chore${BREAKING}: bump version to ${CI_COMMIT_TAG}"
    git push origin main
fi

# Create release tag (skip if already tagged)
echo "=== Creating release for crowci/helm ==="
eval $(ssh-agent -s)
ssh-add ~/.ssh/id_ed25519
git sv tag || echo "Tag already exists for crowci/helm, skipping"
cd ..

#
# Bump, update docs, and release ansible-cicd
#
echo "=== Bumping ansible-cicd ==="
rm -rf ansible-cicd
git clone ssh://git@codefloe.com/devxy/ansible-cicd.git
cd ansible-cicd
echo "Current version: $(grep 'crow_version:' roles/crowci/defaults/main.yml)"
sed -i "s/^crow_version: .*/crow_version: ${CI_COMMIT_TAG}/" roles/crowci/defaults/main.yml
echo "Updated version: $(grep 'crow_version:' roles/crowci/defaults/main.yml)"

# Regenerate role docs with ansibledoctor
echo "Running ansibledoctor..."
ansible-doctor roles -r

# Format with prettier
echo "Running prettier..."
prettier --write .

git config user.name "crowci-bot"
git config user.email "admin@crowci.dev"
git add -A
if git diff --cached --quiet; then
    echo "Nothing to commit (version already at ${CI_COMMIT_TAG})"
else
    git commit -m "chore${BREAKING}: bump version to ${CI_COMMIT_TAG}"
    git push origin main
fi

# Create release tag (skip if already tagged)
echo "=== Creating release for devxy/ansible-cicd ==="
eval $(ssh-agent -s)
ssh-add ~/.ssh/id_ed25519
git sv tag || echo "Tag already exists for devxy/ansible-cicd, skipping"
cd ..

echo "=== All bumps completed ==="
