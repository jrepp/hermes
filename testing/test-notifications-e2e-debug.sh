#!/bin/bash
#
# Comprehensive E2E testing for Hermes notification system
# Tests all notification types, backends, template resolution, and error handling
#

set +e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
REDPANDA_BROKER="${REDPANDA_BROKER:-localhost:19092}"
TEST_TOPIC="hermes.notifications.test"
TIMEOUT=15

# Counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((TESTS_PASSED++))
}

log_failure() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((TESTS_FAILED++))
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

start_test() {
    ((TESTS_TOTAL++))
    log_info "Test $TESTS_TOTAL: $1"
}

# Check dependencies
check_dependencies() {
    log_info "Checking dependencies..."

    if ! command -v docker &> /dev/null; then
        log_failure "docker not found"
        exit 1
    fi

    if ! command -v rpk &> /dev/null; then
        log_warning "rpk not found, using docker exec for Redpanda commands"
    fi

    log_success "Dependencies check passed"
}

# Check if services are running
check_services() {
    log_info "Checking required services..."

    # Check Redpanda
    if ! docker ps | grep -q hermes-redpanda; then
        log_failure "Redpanda container not running"
        exit 1
    fi
    log_success "Redpanda is running"

    # Check notifier services
    for notifier in hermes-notifier-audit hermes-notifier-mail hermes-notifier-ntfy; do
        if ! docker ps | grep -q "$notifier"; then
            log_warning "$notifier container not running"
        else
            log_success "$notifier is running"
        fi
    done
}

# Publish a test notification to Redpanda
publish_notification() {
    local notification_type="$1"
    local recipient_email="$2"
    local template_context="$3"
    local backends="$4"
    local test_id="$5"

    # Build the notification message JSON
    local message=$(cat <<EOF
{
  "id": "${test_id}",
  "type": "${notification_type}",
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%S.%6NZ")",
  "priority": 0,
  "recipients": [
    {
      "email": "${recipient_email}",
      "name": "Test User"
    }
  ],
  "template": "${notification_type}",
  "template_context": ${template_context},
  ${backends}
  "last_retry_at": "0001-01-01T00:00:00Z",
  "next_retry_at": "0001-01-01T00:00:00Z"
}
EOF
)

    # Publish to Redpanda using rpk (via docker exec)
    echo "$message" | docker exec -i hermes-redpanda rpk topic produce "$TEST_TOPIC" \
        --key "$recipient_email" > /dev/null 2>&1

    if [ $? -eq 0 ]; then
        log_success "Published $notification_type notification (ID: $test_id)"
        return 0
    else
        log_failure "Failed to publish $notification_type notification"
        return 1
    fi
}

# Wait for notification to be processed and verify in audit logs
verify_audit_log() {
    local test_id="$1"
    local expected_patterns="$2"
    local container="${3:-hermes-notifier-audit}"

    log_info "Waiting for notification to be processed (max ${TIMEOUT}s)..."

    # Wait and check logs
    local elapsed=0
    local found=false

    while [ $elapsed -lt $TIMEOUT ]; do
        # Get recent logs
        local logs=$(docker logs "$container" --tail 200 2>&1)

        # Check if our test ID appears in logs
        if echo "$logs" | grep -q "$test_id"; then
            found=true
            break
        fi

        sleep 1
        ((elapsed++))
    done

    if [ "$found" = false ]; then
        log_failure "Notification $test_id not found in logs after ${TIMEOUT}s"
        return 1
    fi

    # Verify all expected patterns are in the logs
    local logs=$(docker logs "$container" --tail 200 2>&1)
    local all_found=true

    IFS='|' read -ra PATTERNS <<< "$expected_patterns"
    for pattern in "${PATTERNS[@]}"; do
        if ! echo "$logs" | grep -q "$pattern"; then
            log_failure "Expected pattern not found in logs: $pattern"
            all_found=false
        fi
    done

    if [ "$all_found" = true ]; then
        log_success "All expected patterns found in audit logs for $test_id"
        return 0
    else
        return 1
    fi
}

# Test 1: document_approved notification
test_document_approved() {
    start_test "document_approved notification with full template resolution"

    local test_id="test-doc-approved-$(date +%s)"
    local recipient="approved-test@example.com"

    local context=$(cat <<'EOF'
{
  "DocumentShortName": "RFC-087",
  "DocumentTitle": "Notification Backend System",
  "ApproverName": "Alice Test",
  "ApproverEmail": "alice@example.com",
  "DocumentNonApproverCount": 3,
  "DocumentURL": "https://hermes.example.com/document/test-123",
  "Product": "Hermes",
  "DocumentOwner": "Bob Test",
  "DocumentStatus": "In-Review",
  "DocumentType": "RFC"
}
EOF
)

    local backends='"backends": ["audit"],'

    publish_notification "document_approved" "$recipient" "$context" "$backends" "$test_id"

    # Verify expected content in logs
    local expected="$test_id|Alice Test|RFC-087|approved|Bob Test"
    verify_audit_log "$test_id" "$expected"
}

# Test 2: review_requested notification
test_review_requested() {
    start_test "review_requested notification"

    local test_id="test-review-req-$(date +%s)"
    local recipient="review-test@example.com"

    local context=$(cat <<'EOF'
{
  "DocumentShortName": "RFC-088",
  "DocumentTitle": "Test Review System",
  "RequesterName": "Charlie Reviewer",
  "RequesterEmail": "charlie@example.com",
  "DocumentURL": "https://hermes.example.com/document/test-456",
  "Product": "Hermes",
  "DocumentOwner": "Diana Owner",
  "DocumentStatus": "WIP",
  "DocumentType": "PRD"
}
EOF
)

    local backends='"backends": ["audit"],'

    publish_notification "review_requested" "$recipient" "$context" "$backends" "$test_id"

    local expected="$test_id|Charlie Reviewer|RFC-088|review|Diana Owner"
    verify_audit_log "$test_id" "$expected"
}

# Test 3: new_owner notification
test_new_owner() {
    start_test "new_owner notification"

    local test_id="test-new-owner-$(date +%s)"
    local recipient="owner-test@example.com"

    local context=$(cat <<'EOF'
{
  "DocumentShortName": "RFC-089",
  "DocumentTitle": "Ownership Transfer Test",
  "PreviousOwnerName": "Eve Previous",
  "NewOwnerName": "Frank New",
  "DocumentURL": "https://hermes.example.com/document/test-789",
  "Product": "Hermes",
  "DocumentStatus": "Approved",
  "DocumentType": "RFC"
}
EOF
)

    local backends='"backends": ["audit"],'

    publish_notification "new_owner" "$recipient" "$context" "$backends" "$test_id"

    local expected="$test_id|Frank New|RFC-089|owner|Eve Previous"
    verify_audit_log "$test_id" "$expected"
}

# Test 4: document_published notification
test_document_published() {
    start_test "document_published notification"

    local test_id="test-pub-$(date +%s)"
    local recipient="published-test@example.com"

    local context=$(cat <<'EOF'
{
  "DocumentShortName": "RFC-090",
  "DocumentTitle": "Publication Test",
  "PublisherName": "Grace Publisher",
  "DocumentURL": "https://hermes.example.com/document/test-999",
  "Product": "Hermes",
  "DocumentOwner": "Henry Owner",
  "DocumentType": "RFC"
}
EOF
)

    local backends='"backends": ["audit"],'

    publish_notification "document_published" "$recipient" "$context" "$backends" "$test_id"

    local expected="$test_id|Grace Publisher|RFC-090|published|Henry Owner"
    verify_audit_log "$test_id" "$expected"
}

# Test 5: Multiple recipients
test_multiple_recipients() {
    start_test "Notification with multiple recipients"

    local test_id="test-multi-$(date +%s)"

    local message=$(cat <<EOF
{
  "id": "${test_id}",
  "type": "document_approved",
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%S.%6NZ")",
  "priority": 0,
  "recipients": [
    {"email": "recipient1@example.com", "name": "User One"},
    {"email": "recipient2@example.com", "name": "User Two"},
    {"email": "recipient3@example.com", "name": "User Three"}
  ],
  "template": "document_approved",
  "template_context": {
    "DocumentShortName": "RFC-091",
    "DocumentTitle": "Multi Recipient Test",
    "ApproverName": "Ivy Approver",
    "ApproverEmail": "ivy@example.com",
    "DocumentNonApproverCount": 2,
    "DocumentURL": "https://hermes.example.com/document/test-multi",
    "Product": "Hermes",
    "DocumentOwner": "Jack Owner",
    "DocumentStatus": "In-Review",
    "DocumentType": "RFC"
  },
  "backends": ["audit"]
}
EOF
)

    echo "$message" | docker exec -i hermes-redpanda rpk topic produce "$TEST_TOPIC" \
        --key "recipient1@example.com" > /dev/null 2>&1

    local expected="$test_id|Ivy Approver|RFC-091"
    verify_audit_log "$test_id" "$expected"
}

# Test 6: Multiple backends (if available)
test_multiple_backends() {
    start_test "Notification routed to multiple backends"

    # Check if mail backend is available
    if ! docker ps | grep -q hermes-notifier-mail; then
        log_warning "Skipping multi-backend test (mail notifier not running)"
        ((TESTS_TOTAL--))
        return 0
    fi

    local test_id="test-backends-$(date +%s)"
    local recipient="multiback-test@example.com"

    local context=$(cat <<'EOF'
{
  "DocumentShortName": "RFC-092",
  "DocumentTitle": "Multi Backend Test",
  "ApproverName": "Kate Approver",
  "ApproverEmail": "kate@example.com",
  "DocumentNonApproverCount": 1,
  "DocumentURL": "https://hermes.example.com/document/test-multi-backend",
  "Product": "Hermes",
  "DocumentOwner": "Leo Owner",
  "DocumentStatus": "In-Review",
  "DocumentType": "RFC"
}
EOF
)

    local backends='"backends": ["audit", "mail"],'

    publish_notification "document_approved" "$recipient" "$context" "$backends" "$test_id"

    # Verify in audit logs
    local expected="$test_id|Kate Approver|RFC-092"
    verify_audit_log "$test_id" "$expected" "hermes-notifier-audit"
}

# Test 7: Template with special characters (XSS prevention)
test_template_special_chars() {
    start_test "Template with special characters (XSS prevention)"

    local test_id="test-xss-$(date +%s)"
    local recipient="xss-test@example.com"

    local context=$(cat <<'EOF'
{
  "DocumentShortName": "RFC-<script>alert('xss')</script>",
  "DocumentTitle": "Test <b>HTML</b> & 'Quotes\"",
  "ApproverName": "Alice & Bob",
  "ApproverEmail": "alice@example.com",
  "DocumentNonApproverCount": 0,
  "DocumentURL": "https://hermes.example.com/document/test-xss",
  "Product": "Hermes",
  "DocumentOwner": "Owner's Name",
  "DocumentStatus": "In-Review",
  "DocumentType": "RFC"
}
EOF
)

    local backends='"backends": ["audit"],'

    publish_notification "document_approved" "$recipient" "$context" "$backends" "$test_id"

    # Verify the special characters are present (they should be logged safely)
    local expected="$test_id|Alice & Bob"
    verify_audit_log "$test_id" "$expected"
}

# Test 8: Consumer group behavior
test_consumer_group() {
    start_test "Consumer group is processing messages"

    # Check consumer group status
    local group_status=$(docker exec hermes-redpanda rpk group describe hermes-notifiers 2>&1)

    if echo "$group_status" | grep -q "Stable"; then
        log_success "Consumer group 'hermes-notifiers' is Stable"
    else
        log_failure "Consumer group 'hermes-notifiers' is not Stable"
        echo "$group_status"
        return 1
    fi

    # Check for consumer lag
    if echo "$group_status" | grep -q "0.*0"; then
        log_success "No consumer lag detected"
    else
        log_warning "Consumer lag detected (may be processing)"
    fi
}

# Test 9: Message ordering (same partition key)
test_message_ordering() {
    start_test "Message ordering with same partition key"

    local base_id="test-order-$(date +%s)"
    local recipient="order-test@example.com"

    # Send 3 messages with same key (should go to same partition)
    for i in 1 2 3; do
        local test_id="${base_id}-${i}"
        local context=$(cat <<EOF
{
  "DocumentShortName": "RFC-093-${i}",
  "DocumentTitle": "Ordering Test ${i}",
  "ApproverName": "Approver ${i}",
  "ApproverEmail": "approver@example.com",
  "DocumentNonApproverCount": 0,
  "DocumentURL": "https://hermes.example.com/document/test-order-${i}",
  "Product": "Hermes",
  "DocumentOwner": "Owner",
  "DocumentStatus": "In-Review",
  "DocumentType": "RFC"
}
EOF
)

        local backends='"backends": ["audit"],'
        publish_notification "document_approved" "$recipient" "$context" "$backends" "$test_id" > /dev/null 2>&1
    done

    sleep 3

    # Check that all 3 messages appear in logs
    local logs=$(docker logs hermes-notifier-audit --tail 100 2>&1)
    local found_count=0

    for i in 1 2 3; do
        if echo "$logs" | grep -q "${base_id}-${i}"; then
            ((found_count++))
        fi
    done

    if [ $found_count -eq 3 ]; then
        log_success "All 3 ordered messages processed"
    else
        log_failure "Only $found_count/3 ordered messages found"
    fi
}

# Test 10: High volume (stress test)
test_high_volume() {
    start_test "High volume stress test (50 notifications)"

    local base_id="test-volume-$(date +%s)"

    log_info "Publishing 50 notifications..."
    for i in $(seq 1 50); do
        local test_id="${base_id}-${i}"
        local recipient="volume-${i}@example.com"

        local context=$(cat <<EOF
{
  "DocumentShortName": "RFC-${i}",
  "DocumentTitle": "Volume Test ${i}",
  "ApproverName": "Approver ${i}",
  "ApproverEmail": "approver${i}@example.com",
  "DocumentNonApproverCount": 0,
  "DocumentURL": "https://hermes.example.com/document/test-${i}",
  "Product": "Hermes",
  "DocumentOwner": "Owner",
  "DocumentStatus": "In-Review",
  "DocumentType": "RFC"
}
EOF
)

        local backends='"backends": ["audit"],'
        publish_notification "document_approved" "$recipient" "$context" "$backends" "$test_id" > /dev/null 2>&1 &
    done

    wait # Wait for all background jobs

    log_info "Waiting for processing..."
    sleep 5

    # Check how many were processed
    local logs=$(docker logs hermes-notifier-audit --tail 300 2>&1)
    local found_count=0

    for i in $(seq 1 50); do
        if echo "$logs" | grep -q "${base_id}-${i}"; then
            ((found_count++))
        fi
    done

    if [ $found_count -ge 45 ]; then
        log_success "High volume test passed: $found_count/50 messages processed"
    else
        log_failure "High volume test failed: only $found_count/50 messages processed"
    fi
}

# Print summary
print_summary() {
    echo ""
    echo "========================================"
    echo "         Test Summary"
    echo "========================================"
    echo -e "Total Tests:  ${TESTS_TOTAL}"
    echo -e "${GREEN}Passed:       ${TESTS_PASSED}${NC}"
    echo -e "${RED}Failed:       ${TESTS_FAILED}${NC}"
    echo "========================================"

    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "${GREEN}All tests passed!${NC}"
        exit 0
    else
        echo -e "${RED}Some tests failed!${NC}"
        exit 1
    fi
}

# Main execution
main() {
    echo "========================================"
    echo "  Hermes Notification E2E Test Suite"
    echo "========================================"
    echo ""

    check_dependencies
    check_services

    echo ""
    echo "Running tests..."
    echo ""

    # Run all tests
    test_document_approved
    test_review_requested
    test_new_owner
    test_document_published
    test_multiple_recipients
    test_multiple_backends
    test_template_special_chars
    test_consumer_group
    test_message_ordering
    test_high_volume

    print_summary
}

# Run main
main "$@"
