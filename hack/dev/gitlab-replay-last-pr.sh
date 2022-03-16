#!/usr/bin/env bash
set -euf

uselast=
runit=

options=$(getopt -o ln --long color: -- "$@")
[ $? -eq 0 ] || {
    echo "Incorrect options provided"
    exit 1
}
eval set -- "$options"
while true; do
    case "$1" in
    -l)
        uselast=true
        ;;
    -r)
        runit=true
        ;;
    --)
        shift
        break
        ;;
    esac
    shift
done

TMPD=/tmp/gitlab-pac-last-run
rm -rf ${TMPD};mkdir -p ${TMPD}

if [[ -n ${uselast} ]];then
    arg="-L"
elif [[ -n ${1:-""} ]];then
    arg="${1}"
else
   pr=$(tkn tr ls --no-headers|fzf  -1 --preview 'tkn tr describe `echo {}|sed "s/ .*//"`')
   [[ -z ${pr} ]] && exit 0
   arg=$(echo ${pr}|awk '{print $1}')
   echo "Selected ${arg}"
fi
tkn tr logs ${arg} --prefix=false 2>/dev/null > ${TMPD}/last
[[ -s ${TMPD}/last ]] || { echo "payload could not be found"; exit 1 ;}


export PAC_PAYLOAD_FILE=${TMPD}/payload.json
sed -n '1,2{N;s/\n//;p}' ${TMPD}/last|jq .  > ${TMPD}/payload.json
[[ -s ${TMPD}/payload.json ]] || { echo "payload json could not be found"; exit 1 ;}

grep -e "^PAC_.[a-zA-Z0-9_-]*=" ${TMPD}/last|grep -v PAC_PAYLOAD_FILE|sed -e 's/=\(.*\)/="\1"/' -e 's/^/export /' > ${TMPD}/run.sh
chmod +x ${TMPD}/run.sh
echo "export PAC_PAYLOAD_FILE=$PAC_PAYLOAD_FILE" >> ${TMPD}/run.sh
sed 's/^export //' ${TMPD}/run.sh > ${TMPD}/env
echo "go run cmd/pipelines-as-code/main.go" >> ${TMPD}/run.sh

echo "Generated the env files in ${TMPD} use this to rerun the tr locally: "
echo "${TMPD}/run.sh"
[[ -n ${runit} ]] && ${TMPD}/run.sh
