set -euo pipefail

: "${INGRESS_NGINX_VERSION:=controller-v1.9.6}"

echo "Creating KinD cluster"
cat <<EOF | kind create cluster --name kwok --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
kubeadmConfigPatches:
- |-
  kind: ClusterConfiguration
  controllerManager:
    extraArgs:
      bind-address: 0.0.0.0
  etcd:
    local:
      extraArgs:
        listen-metrics-urls: http://0.0.0.0:2381
  scheduler:
    extraArgs:
      bind-address: 0.0.0.0
- |-
  kind: KubeProxyConfiguration
  metricsBindAddress: 0.0.0.0
nodes:
  - role: control-plane
    image: kindest/node:v1.29.2@sha256:51a1434a5397193442f0be2a297b488b6c919ce8a3931be0ce822606ea5ca245
    extraPortMappings:
    - containerPort: 80
      hostPort: 80
      protocol: TCP
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."192.168.0.1:5001"]
    endpoint = ["http://192.168.0.1:5001"]
EOF

# Deploy the NGINX Ingress controller
echo "Deploying Ingress controller"
curl -s https://raw.githubusercontent.com/kubernetes/ingress-nginx/"${INGRESS_NGINX_VERSION}"/deploy/static/provider/kind/deploy.yaml | sed "s/--publish-status-address=localhost/--report-node-internal-ip-address\\n        - --status-update-interval=10/g" | kubectl apply -f -
kubectl annotate ingressclass nginx "ingressclass.kubernetes.io/is-default-class=true"
kubectl -n ingress-nginx wait --timeout=300s --for=condition=Available deployments --all

# Deploy KWOK
echo "Deploying KWOK"
kubectl apply -f config/kwok/kwok.yaml
kubectl wait --for condition=Established --all CustomResourceDefinition
kubectl apply -f config/kwok/stage-fast.yaml

# Deploy Prometheus
kubectl apply -f config/prometheus/crds
kubectl wait --for condition=Established --all CustomResourceDefinition
kubectl apply -k config/prometheus

# Deploy kube-state-metrics
kubectl apply -k config/kube-state-metrics

# Deploy Prometheus monitors
kubectl apply -f config/kube-system

# Deploy Grafana
kubectl apply -k config/grafana
