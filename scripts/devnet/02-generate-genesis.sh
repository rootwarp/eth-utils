#!/bin/bash
# 02-generate-genesis.sh
# Generate EL and CL genesis configurations

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib/common.sh"

echo "=============================================="
echo "  Ethereum Devnet - Genesis Generation"
echo "=============================================="
echo

# Check if genesis already exists
if [[ -f "${GENESIS_DIR}/genesis.json" ]] && [[ -f "${GENESIS_DIR}/genesis.ssz" ]]; then
    log_warn "Genesis files already exist"
    read -p "Do you want to regenerate them? This will require restarting from scratch. (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Keeping existing genesis files"
        exit 0
    fi
fi

# Create directories
mkdir -p "${GENESIS_DIR}"

# Calculate genesis time (current time + genesis delay)
GENESIS_TIME=$(get_genesis_time)
GENESIS_TIME_HEX=$(printf '%x' "$GENESIS_TIME")

log_info "Genesis time: ${GENESIS_TIME} ($(date -r ${GENESIS_TIME} 2>/dev/null || date -d @${GENESIS_TIME}))"

# Generate EL genesis.json
log_info "Generating Execution Layer genesis.json..."

EL_TEMPLATE="${SCRIPT_DIR}/config/el/genesis-template.json"
EL_GENESIS="${GENESIS_DIR}/genesis.json"

# Substitute variables in template
sed -e "s/\${CHAIN_ID}/${CHAIN_ID}/g" \
    -e "s/\${GENESIS_TIME_HEX}/${GENESIS_TIME_HEX}/g" \
    -e "s/\${DEV_ACCOUNT}/${DEV_ACCOUNT}/g" \
    "${EL_TEMPLATE}" > "${EL_GENESIS}"

# Validate JSON
if ! jq empty "${EL_GENESIS}" 2>/dev/null; then
    log_error "Generated genesis.json is not valid JSON"
    exit 1
fi

log_success "EL genesis.json generated"

# Generate CL config.yaml
log_info "Generating Consensus Layer config.yaml..."

CL_TEMPLATE="${SCRIPT_DIR}/config/cl/config-template.yaml"
CL_CONFIG="${GENESIS_DIR}/config.yaml"

sed -e "s/\${CHAIN_ID}/${CHAIN_ID}/g" \
    -e "s/\${NETWORK_ID}/${NETWORK_ID}/g" \
    -e "s/\${NUM_VALIDATORS}/${NUM_VALIDATORS}/g" \
    -e "s/\${GENESIS_TIME}/${GENESIS_TIME}/g" \
    -e "s/\${GENESIS_DELAY}/${GENESIS_DELAY}/g" \
    -e "s/\${SECONDS_PER_SLOT}/${SECONDS_PER_SLOT}/g" \
    -e "s/\${SLOTS_PER_EPOCH}/${SLOTS_PER_EPOCH}/g" \
    "${CL_TEMPLATE}" > "${CL_CONFIG}"

log_success "CL config.yaml generated"

# Generate genesis.ssz and deploy_block.txt using ethereum-genesis-generator
log_info "Generating CL genesis.ssz using ethereum-genesis-generator..."

# Create a temporary directory for the generator
TEMP_DIR=$(mktemp -d)
trap "rm -rf ${TEMP_DIR}" EXIT

# Create generator config
cat > "${TEMP_DIR}/values.env" <<EOF
PRESET_BASE=mainnet
CHAIN_ID=${CHAIN_ID}
DEPOSIT_CONTRACT_ADDRESS=0x4242424242424242424242424242424242424242
EL_AND_CL_MNEMONIC="${VALIDATOR_MNEMONIC}"
CL_EXEC_BLOCK=0
SLOT_DURATION_IN_SECONDS=${SECONDS_PER_SLOT}
SLOTS_PER_EPOCH=${SLOTS_PER_EPOCH}
GENESIS_TIMESTAMP=${GENESIS_TIME}
GENESIS_DELAY=${GENESIS_DELAY}
NUMBER_OF_VALIDATORS=${NUM_VALIDATORS}
GENESIS_FORK_VERSION=0x10000000
ALTAIR_FORK_VERSION=0x20000000
BELLATRIX_FORK_VERSION=0x30000000
CAPELLA_FORK_VERSION=0x40000000
DENEB_FORK_VERSION=0x50000000
ELECTRA_FORK_VERSION=0x60000000
ALTAIR_FORK_EPOCH=0
BELLATRIX_FORK_EPOCH=0
CAPELLA_FORK_EPOCH=0
DENEB_FORK_EPOCH=0
ELECTRA_FORK_EPOCH=0
WITHDRAWAL_TYPE=0x01
WITHDRAWAL_ADDRESS=${DEV_ACCOUNT}
SHADOW_FORK_RPC=
SHADOW_FORK_FILE=
EOF

# Create el_prestate.json for genesis generator
cat > "${TEMP_DIR}/el_prestate.json" <<EOF
{
  "alloc": {
    "${DEV_ACCOUNT}": {
      "balance": "0x200000000000000000000000000000000000000000000000000000000000000"
    }
  }
}
EOF

# Run the genesis generator
docker run --rm \
    -v "${GENESIS_DIR}:/data" \
    -v "${TEMP_DIR}/values.env:/config/values.env" \
    "${GENESIS_GENERATOR_IMAGE}" \
    all

# The generator outputs to /data/metadata/, copy needed files to expected locations
if [[ -f "${GENESIS_DIR}/metadata/genesis.ssz" ]]; then
    cp "${GENESIS_DIR}/metadata/genesis.ssz" "${GENESIS_DIR}/genesis.ssz"
    cp "${GENESIS_DIR}/metadata/config.yaml" "${GENESIS_DIR}/config.yaml"
    cp "${GENESIS_DIR}/metadata/genesis.json" "${GENESIS_DIR}/genesis.json"
    # Copy deposit contract files (required by Lighthouse)
    cp "${GENESIS_DIR}/metadata/deposit_contract"*.txt "${GENESIS_DIR}/"
    # Copy JWT secret if generated
    if [[ -f "${GENESIS_DIR}/jwt/jwtsecret" ]]; then
        cp "${GENESIS_DIR}/jwt/jwtsecret" "${GENESIS_DIR}/jwtsecret"
    fi
fi

# Verify required files were created
if [[ ! -f "${GENESIS_DIR}/genesis.ssz" ]]; then
    log_error "genesis.ssz was not generated"
    exit 1
fi

# Create deploy_block.txt (genesis at block 0)
echo "0" > "${GENESIS_DIR}/deploy_block.txt"

log_success "CL genesis.ssz generated"

echo
echo "=============================================="
log_success "Genesis generation complete!"
echo "=============================================="
echo
echo "Generated files:"
echo "  - EL genesis: ${EL_GENESIS}"
echo "  - CL config:  ${CL_CONFIG}"
echo "  - CL genesis: ${GENESIS_DIR}/genesis.ssz"
echo
echo "Configuration:"
echo "  - Chain ID:        ${CHAIN_ID}"
echo "  - Genesis time:    ${GENESIS_TIME}"
echo "  - Validators:      ${NUM_VALIDATORS}"
echo "  - Slot duration:   ${SECONDS_PER_SLOT}s"
echo "  - Slots per epoch: ${SLOTS_PER_EPOCH}"
echo
echo "Next step: Run ./03-generate-validator-keys.sh"
