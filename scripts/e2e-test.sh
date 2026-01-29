#!/bin/bash
# E2E Test for key-seeker ZFS unlock
# Run this on a Linux system with ZFS installed

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
TEST_PASSPHRASE="test-passphrase"
TEST_POOL="testpool"
TEST_DATASET="$TEST_POOL/encrypted"
TEST_IMG="/tmp/zfs-test.img"

echo "=== Key-Seeker E2E Test ==="
echo "Project: $PROJECT_DIR"
echo ""

# Build key-seeker
echo "[1/7] Building key-seeker..."
cd "$PROJECT_DIR"
go build -o bin/key-seeker ./cmd/key-seeker
echo "      Built: bin/key-seeker"

# Create test ZFS pool
echo "[2/7] Creating test ZFS pool..."
sudo truncate -s 1G "$TEST_IMG"
sudo zpool create "$TEST_POOL" "$TEST_IMG"
echo "      Created pool: $TEST_POOL"

# Create encrypted dataset
echo "[3/7] Creating encrypted dataset..."
echo "$TEST_PASSPHRASE" | sudo zfs create \
    -o encryption=on \
    -o keyformat=passphrase \
    -o keylocation=prompt \
    "$TEST_DATASET"
echo "      Created dataset: $TEST_DATASET"

# Lock the dataset
echo "[4/7] Locking dataset..."
sudo zfs unmount "$TEST_DATASET"
sudo zfs unload-key "$TEST_DATASET"

# Verify locked
echo "[5/7] Verifying locked state..."
STATUS=$(sudo zfs get -H -o value keystatus "$TEST_DATASET")
if [ "$STATUS" != "unavailable" ]; then
    echo "      ERROR: Expected 'unavailable', got '$STATUS'"
    exit 1
fi
echo "      Status: $STATUS (correct)"

# Test unlock with key-seeker
echo "[6/7] Testing key-seeker unlock..."
echo "$TEST_PASSPHRASE" | sudo "$PROJECT_DIR/bin/key-seeker" --dataset "$TEST_DATASET"

# Verify unlocked
STATUS=$(sudo zfs get -H -o value keystatus "$TEST_DATASET")
if [ "$STATUS" != "available" ]; then
    echo "      ERROR: Expected 'available', got '$STATUS'"
    exit 1
fi
echo "      Status: $STATUS (correct)"

# Cleanup
echo "[7/7] Cleaning up..."
sudo zpool destroy "$TEST_POOL"
rm -f "$TEST_IMG"
echo "      Cleaned up"

echo ""
echo "=== E2E Test PASSED ==="
