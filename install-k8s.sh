#!/bin/bash
# turnoff swap (no need to reboot)
swapoff -a && sed -i.bak '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab

# set mirrors
# apt-get mirrors
echo "deb http://mirrors.aliyun.com/ubuntu/ bionic main restricted universe multiverse" > /etc/apt/sources.list
echo "deb-src http://mirrors.aliyun.com/ubuntu/ bionic main restricted universe multiverse" >> /etc/apt/sources.list
echo "deb http://mirrors.aliyun.com/ubuntu/ bionic-security main restricted universe multiverse" >> /etc/apt/sources.list
echo "deb-src http://mirrors.aliyun.com/ubuntu/ bionic-security main restricted universe multiverse" >> /etc/apt/sources.list
echo "deb http://mirrors.aliyun.com/ubuntu/ bionic-updates main restricted universe multiverse" >> /etc/apt/sources.list
echo "deb-src http://mirrors.aliyun.com/ubuntu/ bionic-updates main restricted universe multiverse" >> /etc/apt/sources.list
echo "deb http://mirrors.aliyun.com/ubuntu/ bionic-proposed main restricted universe multiverse" >> /etc/apt/sources.list
echo "deb-src http://mirrors.aliyun.com/ubuntu/ bionic-proposed main restricted universe multiverse" >> /etc/apt/sources.list
echo "deb http://mirrors.aliyun.com/ubuntu/ bionic-backports main restricted universe multiverse" >> /etc/apt/sources.list
echo "deb-src http://mirrors.aliyun.com/ubuntu/ bionic-backports main restricted universe multiverse" >> /etc/apt/sources.list

# prepare install utils
apt-get -y update
apt-get install -y apt-transport-https ca-certificates curl software-properties-common

# install docker ce
curl -fsSL http://mirrors.aliyun.com/docker-ce/linux/ubuntu/gpg | apt-key add -
add-apt-repository "deb [arch=amd64] http://mirrors.aliyun.com/docker-ce/linux/ubuntu $(lsb_release -cs) stable"
apt-get -y update
apt-get install -y docker-ce=5:18.09.2~3-0~ubuntu-bionic

# set docker mirrors
echo "{" > /etc/docker/daemon.json
echo "  \"registry-mirrors\": [\"https://qohso9cl.mirror.aliyuncs.com/\"]" >> /etc/docker/daemon.json
echo "}" >> /etc/docker/daemon.json

# install kubeadm
curl https://mirrors.aliyun.com/kubernetes/apt/doc/apt-key.gpg | apt-key add -
add-apt-repository "deb https://mirrors.aliyun.com/kubernetes/apt/ kubernetes-xenial main"
apt-get -y update
apt-get install -y kubernetes-cni=0.6.0-00
apt-get install -y kubelet=1.13.3-00
apt-get install -y kubectl=1.13.3-00
apt-get install -y kubeadm=1.13.3-00 
apt-mark hold kubernetes-cni
apt-mark hold kubelet
apt-mark hold kubectl
apt-mark hold kubeadm
