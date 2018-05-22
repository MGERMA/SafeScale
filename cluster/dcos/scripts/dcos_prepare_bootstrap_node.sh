#!/usr/bin/env bash
#
# Deploys a DCOS bootstrap/upgrade server with minimum requirements
#
# This script has to be executed on the bootstrap/upgrade server

# Installs docker
sudo tee /etc/modules-load.d/overlay.conf <<-'EOF'
overlay
EOF
sudo modprobe overlay

sudo tee /etc/yum.repos.d/docker.repo <<-'EOF'
[dockerrepo]
name=Docker Repository
baseurl=https://yum.dockerproject.org/repo/main/centos/$releasever/
enabled=1
gpgcheck=1
gpgkey=https://yum.dockerproject.org/gpg
EOF

sudo yum upgrade --assumeyes --tolerant
sudo yum update --assumeyes
sudo yum install -y docker-ce-17.05 docker-ce-selinux-17.05

sudo mkdir -p /etc/systemd/system/docker.service.d && sudo tee /etc/systemd/system/docker.service.d/override.conf <<- EOF
[Service]
ExecStart=
ExecStart=/usr/bin/dockerd --storage-driver=overlay
EOF

sudo systemctl start docker
sudo systemctl enable docker

# Prepares the folder to contain cluster Bootstrap/Upgrade data
mkdir -p /usr/local/dcos/

# Creates the ssh key to use
ssh-keygen -b 4096 -t rsa -f genconf/ssh_key -q -P ""
chmod 0600 genconf/ssh_key

exit 0