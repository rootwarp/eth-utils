#!/bin/bash
# 04-start-execution-layer.sh
# Initialize and start Geth container

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"

echo "=============================================="
echo "  Ethereum Devnet - Execution Layer Startup"
echo "=============================================="
echo

# Check prerequisites
validate_data_exists "JWT secret" "${JWT_DIR}/jwt.hex"
validate_data_exists "EL genesis" "${GENESIS_DIR}/genesis.json"

# Stop existing container if running
if is_container_running "$GETH_CONTAINER"; then
    log_warn "Geth container is already running"
    read -p "Do you want to restart it? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Keeping existing container"
        exit 0
    fi
    stop_container "$GETH_CONTAINER"
fi

# Remove existing container
remove_container "$GETH_CONTAINER"

# Create Docker network
ensure_docker_network

# Check if Geth needs initialization
GETH_INITIALIZED=false
if [[ -d "${EL_DATA_DIR}/geth/chaindata" ]]; then
    GETH_INITIALIZED=true
    log_info "Geth data directory exists, skipping initialization"
else
    log_info "Initializing Geth with genesis..."

    docker run --rm \
        -v "${EL_DATA_DIR}:/data" \
        -v "${GENESIS_DIR}:/genesis:ro" \
        "${GETH_IMAGE}" \
        --datadir=/data \
        init /genesis/genesis.json

    log_success "Geth initialized"
fi

# Start Geth container
log_info "Starting Geth container..."

docker run -d \
    --name "${GETH_CONTAINER}" \
    --network "${DOCKER_NETWORK}" \
    --restart unless-stopped \
    -p "${EL_HTTP_PORT}:8545" \
    -p "${EL_WS_PORT}:8546" \
    -p "${EL_AUTH_PORT}:8551" \
    -p "${EL_P2P_PORT}:30303/tcp" \
    -p "${EL_P2P_PORT}:30303/udp" \
    -v "${EL_DATA_DIR}:/data" \
    -v "${GENESIS_DIR}:/genesis:ro" \
    -v "${JWT_DIR}:/jwt:ro" \
    "${GETH_IMAGE}" \
    --datadir=/data \
    --http \
    --http.addr=0.0.0.0 \
    --http.port=8545 \
    --http.api=eth,net,web3,engine,admin,debug,txpool \
    --http.corsdomain="*" \
    --http.vhosts="*" \
    --ws \
    --ws.addr=0.0.0.0 \
    --ws.port=8546 \
    --ws.api=eth,net,web3,engine,admin,debug,txpool \
    --ws.origins="*" \
    --authrpc.addr=0.0.0.0 \
    --authrpc.port=8551 \
    --authrpc.vhosts="*" \
    --authrpc.jwtsecret=/jwt/jwt.hex \
    --networkid="${NETWORK_ID}" \
    --nodiscover \
    --syncmode=full \
    --gcmode=archive \
    --allow-insecure-unlock

log_success "Geth container started"

# Wait for RPC to be ready
log_info "Waiting for Geth RPC to be ready..."
sleep 3

MAX_ATTEMPTS=30
ATTEMPT=1
while [[ $ATTEMPT -le $MAX_ATTEMPTS ]]; do
    RESPONSE=$(curl -s -X POST http://localhost:${EL_HTTP_PORT} \
        -H "Content-Type: application/json" \
        --data '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' 2>/dev/null || echo "")

    if [[ -n "$RESPONSE" ]] && echo "$RESPONSE" | jq -e '.result' > /dev/null 2>&1; then
        CHAIN_ID_HEX=$(echo "$RESPONSE" | jq -r '.result')
        CHAIN_ID_DEC=$((CHAIN_ID_HEX))
        log_success "Geth RPC is ready (Chain ID: ${CHAIN_ID_DEC})"
        break
    fi

    echo -n "."
    sleep 2
    ((ATTEMPT++))
done

if [[ $ATTEMPT -gt $MAX_ATTEMPTS ]]; then
    log_error "Geth RPC did not become ready"
    log_info "Check container logs: docker logs ${GETH_CONTAINER}"
    exit 1
fi

echo
echo "=============================================="
log_success "Execution Layer startup complete!"
echo "=============================================="
echo
echo "Geth Endpoints:"
echo "  - JSON-RPC HTTP: http://localhost:${EL_HTTP_PORT}"
echo "  - WebSocket:     ws://localhost:${EL_WS_PORT}"
echo "  - Engine API:    http://localhost:${EL_AUTH_PORT} (authenticated)"
echo
echo "Container: ${GETH_CONTAINER}"
echo "View logs: docker logs -f ${GETH_CONTAINER}"
echo
echo "Next step: Run ./05-start-consensus-layer.sh"
