#!/usr/bin/env bash
# Description: Run the hugo development server and automatically open
# the server in a web browser when it has started
# Usage:
#   ./hack/start-hugo-dev-server.sh [path-to-hugo-binary] [hugo command] [hugo command options...]
#   If no hugo binary is specified, hugo will be launched from $PATH.
#   See `hugo -h` for hugo commands and hugo command options
#
#   Example:
#    # Start development server for docs directory
#   ./hack/start-hugo-dev-server.sh server -s /docs
#
#    # Start development server using specific binary and port
#   ./hack/start-hugo-dev-server.sh server tmp/hugo/hugo -p 9090
#
# Author: Andrew Thorp <andrew.thorp.dev@gmail.com>
set -euo pipefail

HUGO_BIN=${1:-}
if [[ -z "${HUGO_BIN}" ]] || [[ ! -x "${HUGO_BIN}" ]]; then
    HUGO_BIN=$(which hugo)
    echo "No hugo binary path specified, defaulting to '${HUGO_BIN}'" >&2
else
    shift
fi

# Since hugo does not automatically open the server in the browser, the
# build time may vary, and if the default port is in use another port
# is selected, we use a named pipe to listen for hugo to start and
# extract the chosen host+port to open
HUGO_OUTPUT_FIFO=$(mktemp --suffix "pac-dev-docs")
test -e "${HUGO_OUTPUT_FIFO}" && rm "${HUGO_OUTPUT_FIFO}"
mkfifo "${HUGO_OUTPUT_FIFO}"

function listen_and_open() {
    HUGO_SERVER="$(timeout 10 grep -i -m 1 'Web server is available' "${HUGO_OUTPUT_FIFO}" | grep -o 'localhost:[0-9]*')"
    if [[ -z "${HUGO_SERVER}" ]]; then
        echo "Unable to find hugo server address" >&2
        exit 1
    else
        HUGO_SERVER="http://${HUGO_SERVER}"
    fi

    if type -p xdg-open 2>/dev/null >/dev/null; then
        xdg-open "${HUGO_SERVER}"
    elif type -p open 2>/dev/null >/dev/null; then
        open "${HUGO_SERVER}"
    fi
}

listen_and_open &

set -x
exec "${HUGO_BIN}" "${@}" | tee "${HUGO_OUTPUT_FIFO}"
