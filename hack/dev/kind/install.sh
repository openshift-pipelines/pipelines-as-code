#!/usr/bin/env bash
# This should create a dashboard and a PAAC URL at the end on http
#
# You should forward the URL via smee,
# - create a URL in there by going to https://smee.io
# - install gosmee: go install -v github.com/chmouel/gosmee@latest
# - run somewhere in a terminal :
#    gosmee client https://smee.io/aBcDeF http://controller.paac-127-0-0-1.nip.io
#
# You probably need to install passwordstore https://www.passwordstore.org/ and
# add your github secrets : github-application-id github-private-key
# webhook.secret in a folder which needs to be specified  in
# the PAC_PASS_SECRET_FOLDER variable. or otheriwse you have to create the
# pipelines-as-code-secret manually
#
# If you need to install old pac as shipped in OSP1.7, you check it out somewhere
# and set the PAC_DIR to it. It will automatically set the ingress to the right
# place.
set -euf
cd $(dirname $(readlink -f ${0}))

export KIND_CLUSTER_NAME=${KIND_CLUSTER_NAME:-kind}
export KUBECONFIG=${HOME}/.kube/config.${KIND_CLUSTER_NAME}
export TARGET=kubernetes
export DOMAIN_NAME=paac-127-0-0-1.nip.io

kind=$(type -p kind)
[[ -z ${kind} ]] && { echo "Install kind"; exit 1 ;}

TMPD=$(mktemp -d /tmp/.GITXXXX)
REG_PORT='5000'
REG_NAME='kind-registry'
INSTALL_FROM_RELEASE=
PAC_PASS_SECRET_FOLDER=${PAC_PASS_SECRET_FOLDER:-""}
SUDO=sudo
PAC_DIR=${PAC_DIR:-$GOPATH/src/github.com/openshift-pipelines/pipelines-as-code}
INSTALL_GITEA=yes
GITEA_HOST=${GITEA_HOST:-"localhost:3000"}


[[ $(uname -s) == "Darwin" ]] && {
    SUDO=
}

# cleanup on exit (useful for running locally)
cleanup() { rm -rf ${TMPD} ;}
trap cleanup EXIT

function start_registry() {
    running="$(docker inspect -f '{{.State.Running}}' ${REG_NAME} 2>/dev/null || echo false)"

    if [[ ${running} != "true" ]];then
        docker rm -f kind-registry || true
        docker run \
               -d --restart=always -p "127.0.0.1:${REG_PORT}:5000" \
               --name "${REG_NAME}" \
               registry:2
    fi
}

function reinstall_kind() {
	${SUDO} $kind delete cluster --name ${KIND_CLUSTER_NAME} || true
	sed "s,%DOCKERCFG%,${HOME}/.docker/config.json,"  kind.yaml > ${TMPD}/kconfig.yaml

       cat <<EOF >> ${TMPD}/kconfig.yaml
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${REG_PORT}"]
    endpoint = ["http://${REG_NAME}:5000"]
EOF

	${SUDO} ${kind} create cluster --image kindest/node:v1.24.0 --name ${KIND_CLUSTER_NAME} --config  ${TMPD}/kconfig.yaml
	mkdir -p $(dirname ${KUBECONFIG})
	${SUDO} ${kind} --name ${KIND_CLUSTER_NAME} get kubeconfig > ${KUBECONFIG}


    docker network connect "kind" "${REG_NAME}" 2>/dev/null || true
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${REG_PORT}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

}

function install_nginx() {
    echo "Installing nginx"
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml >/dev/null
    i=0
    echo -n "Waiting for nginx to come up: "
	while true;do
		[[ ${i} == 120 ]] && exit 1
		ep=$(kubectl wait --namespace ingress-nginx --for=condition=ready pod --selector=app.kubernetes.io/component=controller --timeout=180s 2>/dev/null || true)
		[[ -n ${ep} ]] && break
		sleep 5
		i=$((i+1))
	done
    echo "done."
}


function install_tekton() {
    echo "Installing Tekton Pipeline"
	kubectl apply --filename https://storage.googleapis.com/tekton-releases-nightly/pipeline/latest/release.yaml >/dev/null
    echo "Installing Tekton Dashboard"
	kubectl apply --filename https://storage.googleapis.com/tekton-releases/dashboard/latest/tekton-dashboard-release.yaml >/dev/null
	i=0
    echo -n "Waiting for tekton pipeline to come up: "
    tt=pipelines
    while true;do
        [[ ${i} == 120 ]] && exit 1
        ep=$(kubectl get ep -n tekton-pipelines tekton-${tt}-webhook -o jsonpath='{.subsets[*].addresses[*].ip}')
        [[ -n ${ep} ]] && break
        sleep 2
        i=$((i+1))
    done
    echo "done."
}

function install_pac() {
	if [[ -n ${INSTALL_FROM_RELEASE} ]];then
		kubectl apply -f ${PAC_RELEASE:-https://github.com/openshift-pipelines/pipelines-as-code/raw/stable/release.k8s.yaml}
	else
        [[ -d ${PAC_DIR} ]] || {
            echo "I cannot find the PAC installation directory, set the variable \$PAC_DIR to define it.
        It default to \$GOPATH/src/github.com/openshift-pipelines/pipelines-as-code."
            exit 1
        }
        oldPwd=${PWD}
        cd ${PAC_DIR}
        echo "Deploying PAC from ${PAC_DIR}"
        env KO_DOCKER_REPO=localhost:5000 ko apply -f config -B >/dev/null
        cd ${oldPwd}
    fi
    configure_pac
    echo "controller: http://controller.${DOMAIN_NAME}"
    echo "dashboard: http://dashboard.${DOMAIN_NAME}"
}

function configure_pac() {
    kubectl get service -n pipelines-as-code
    service_name=pipelines-as-code-controller
    kubectl get service -n pipelines-as-code -o name | \
        sed 's/.*\///' | \
        grep -q el-pipelines-as-code-interceptor && \
        service_name=el-pipelines-as-code-interceptor

    sed -e "s,%DOMAIN_NAME%,${DOMAIN_NAME}," -e "s,%SERVICE_NAME%,${service_name}," ingress.yaml |kubectl apply -f-

    kubectl patch configmap -n pipelines-as-code -p "{\"data\":{\"bitbucket-cloud-check-source-ip\": \"false\"}}"  pipelines-as-code && \
    kubectl patch configmap -n pipelines-as-code -p "{\"data\":{\"tekton-dashboard-url\": \"http://dashboard.${DOMAIN_NAME}\"}}" --type merge pipelines-as-code

    set +x
    if [[ -n ${PAC_PASS_SECRET_FOLDER} ]];then
        echo "Installing PAC secrets"
        kubectl create secret generic pipelines-as-code-secret -n pipelines-as-code
        for passk in github-application-id github-private-key webhook.secret;do
            if [[ ${passk} == *-key ]];then
                b64d=$(pass show ${PAC_PASS_SECRET_FOLDER}/${passk}|base64 -w0)
            else
                b64d=$(echo -n $(pass show ${PAC_PASS_SECRET_FOLDER}/${passk})| base64 -w0)
            fi
            kubectl patch secret -n pipelines-as-code -p "{\"data\":{\"${passk}\": \"${b64d}\"}}" \
               --type merge pipelines-as-code-secret >/dev/null
        done
    else
        echo "No secret has been installed"
        echo "you need to create a pass https://www.passwordstore.org/ folder with"
        echo "github-application-id github-private-key webhook.secret information in there"
        echo "and export the PAC_PASS_SECRET_FOLDER vairable to that folder"
        echo "or install your pipelines-as-code-secret manually"
        kubectl delete secret -n pipelines-as-code pipelines-as-code-secret >/dev/null 2>/dev/null || true
    fi

    echo "Set Active Namespace to pipelines-as-code"
    kubectl config set-context --current --namespace=pipelines-as-code >/dev/null
    type -p gosmee || echo "You may want to install psmee with: go install -v github.com/chmouel/gosmee@latest and run:
gosmee client --saveDir /tmp/replays https://smee.io/SMEEID http://controller.${DOMAIN_NAME}"
}

function install_gitea ()
{
    env GITEA_URL="http://${GITEA_HOST}" GITEA_HOST=$GITEA_HOST GITEA_USER="pac" \
        GITEA_PASSWORD="pac" GITEA_REPO_NAME="pac-e2e" ./gitea/deploy.py
}

main() {
    start_registry
	reinstall_kind
	install_nginx
	install_tekton
	install_pac
    install_gitea
    echo "And we are done :): "
}

while getopts "Gpcrb" o; do
    case "${o}" in
        b)
            start_registry
            reinstall_kind
            install_nginx
            install_tekton
            exit
            ;;
        c)
            configure_pac
            exit
            ;;
        p)
            install_pac
            echo "Restarting controller POD: "
            kubectl delete pod -l app.kubernetes.io/part-of=pipelines-as-code -n pipelines-as-code || true
            exit
            ;;
	    r)
		    INSTALL_FROM_RELEASE=yes
            ;;
        G)
            install_gitea
            exit
            ;;

        *)
            echo "Invalid option"; exit 1;
            ;;
    esac
done
shift $((OPTIND-1))

main
