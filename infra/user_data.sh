#!/bin/bash
# cloud-init: Go 1.26 + ベンチに必要な最小ツールをセットアップ
set -eux

apt-get update
apt-get install -y rsync make git

curl -fsSL https://go.dev/dl/go1.26.4.linux-amd64.tar.gz | tar -C /usr/local -xz

cat > /etc/profile.d/go.sh <<'EOF'
export PATH=$PATH:/usr/local/go/bin
export GOEXPERIMENT=simd
EOF

# rsync/ssh の非ログインシェルでも go が見えるように
ln -sf /usr/local/go/bin/go /usr/local/bin/go
