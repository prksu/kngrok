# -*- mode: Python -*-
# pyright: reportUndefinedVariable=false

os.putenv('PATH', './bin' + ':' + os.getenv('PATH'))


load('ext://cert_manager', 'deploy_cert_manager')
deploy_cert_manager()

local("make kustomize", quiet=True)

manager_deps = ["controllers", "ngrok",
                "webhooks", "go.mod", "go.sum", "main.go"]
manager_ignore = ['*/*/zz_generated.deepcopy.go']

local_resource("kngrok-manager-manifests",
               cmd='make manifests',
               trigger_mode=TRIGGER_MODE_AUTO,
               deps=manager_deps,
               ignore=manager_ignore,
               resource_deps=[],
               labels=[])

local_resource("kngrok-manager-binary",
               cmd='go build -o bin/manager main.go',
               env={
                   "CGO_ENABLED": "0",
                   "GOOS": "linux",
                   "GOARCH": "amd64"
               },
               trigger_mode=TRIGGER_MODE_AUTO,
               deps=manager_deps,
               ignore=manager_ignore,
               resource_deps=[],
               labels=[])

dockerfile = """
FROM golang:1.17 as tilt-helper
# Support live reloading with Tilt
RUN wget --output-document /restart.sh --quiet https://raw.githubusercontent.com/tilt-dev/rerun-process-wrapper/master/restart.sh  && \
    wget --output-document /start.sh --quiet https://raw.githubusercontent.com/tilt-dev/rerun-process-wrapper/master/start.sh && \
    chmod +x /start.sh && chmod +x /restart.sh

FROM gcr.io/distroless/base:debug as manager
WORKDIR /
COPY --from=tilt-helper /start.sh .
COPY --from=tilt-helper /restart.sh .
COPY manager .
"""

docker_build(
    ref="ghcr.io/prksu/kngrok",
    context="bin/",
    dockerfile_contents=dockerfile,
    target="manager",
    entrypoint=["sh", "/start.sh", "/manager"],
    only="manager",
    live_update=[
        sync("bin/manager", "/manager"),
        run("sh /restart.sh"),
    ])


k8s_yaml(kustomize("config/default"))
k8s_resource('kngrok-manager', resource_deps=['kngrok-manager-manifests', 'kngrok-manager-binary'], objects=[
    "kngrok-agent-config:configmap",
    "kngrok-system:namespace",
    "kngrok-manager:serviceaccount",
    "kngrok-leader-election-role:role",
    "kngrok-manager-role:clusterrole",
    "kngrok-metrics-reader:clusterrole",
    "kngrok-proxy-role:clusterrole",
    "kngrok-leader-election-rolebinding:rolebinding",
    "kngrok-manager-rolebinding:clusterrolebinding",
    "kngrok-proxy-rolebinding:clusterrolebinding",
    "kngrok-manager-config:configmap",
    "kngrok-serving-cert:certificate",
    "kngrok-selfsigned-issuer:issuer",
])

k8s_resource(new_name='kngrok-manager-webhook', resource_deps=['kngrok-manager'], objects=[
    "kngrok-mutating-webhook-configuration:mutatingwebhookconfiguration",
    "kngrok-validating-webhook-configuration:validatingwebhookconfiguration",
])
