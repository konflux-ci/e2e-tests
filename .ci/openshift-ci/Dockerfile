FROM registry.ci.openshift.org/openshift/release:golang-1.16

SHELL ["/bin/bash", "-c"]

# Install yq, kubectl, kustomize
RUN yum install --assumeyes -d1 python3-pip  httpd-tools && \
    pip3 install --upgrade setuptools && \
    pip3 install yq && \
    curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl && \
    chmod +x ./kubectl && \
    mv ./kubectl /usr/local/bin && \
    curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"  | bash && \
    cp kustomize /usr/bin/ && \
    chmod ug+x /usr/bin/kustomize
