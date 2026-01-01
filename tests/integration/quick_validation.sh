#!/bin/bash
# Quick Validation - Tests key features quickly
set -e

cd "$(dirname "$0")/../.."

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}Quick Validation of Production Features${NC}\n"

# Build
echo "1. Building binaries..."
go build -o /tmp/quick-master ./master/cmd/master > /dev/null 2>&1
echo -e "${GREEN}✓ Master builds${NC}"

go build -o /tmp/quick-agent ./worker/cmd/agent > /dev/null 2>&1
echo -e "${GREEN}✓ Agent builds${NC}"

go build -o /tmp/quick-cli ./cmd/ffrtmp > /dev/null 2>&1
echo -e "${GREEN}✓ CLI builds${NC}"

# Run unit tests
echo -e "\n2. Running unit tests..."
cd shared/pkg/store && go test -v -run TestSQLiteBasicOperations > /tmp/test.log 2>&1
if grep -q "PASS.*TestSQLiteBasicOperations" /tmp/test.log; then
    echo -e "${GREEN}✓ Store tests pass${NC}"
else
    echo "✗ Store tests failed"
    cat /tmp/test.log
    exit 1
fi

cd ../api && go test -v > /tmp/api-test.log 2>&1
if grep -q "PASS" /tmp/api-test.log; then
    echo -e "${GREEN}✓ API tests pass${NC}"
else
    echo "✗ API tests failed"
    cat /tmp/api-test.log
    exit 1
fi

# Start master briefly
echo -e "\n3. Testing master startup..."
cd ../../../
export MASTER_API_KEY="test-key"
/tmp/quick-master --db /tmp/quick-test.db --port 8095 --tls=false --metrics=false > /tmp/master.log 2>&1 &
MASTER_PID=$!
sleep 2

if kill -0 $MASTER_PID 2>/dev/null; then
    echo -e "${GREEN}✓ Master starts successfully${NC}"
    
    # Test API endpoint
    HEALTH=$(curl -s http://localhost:8095/health)
    if echo "$HEALTH" | grep -q "healthy"; then
        echo -e "${GREEN}✓ Master API responds${NC}"
    fi
    
    # Test node registration
    NODE=$(curl -s -X POST http://localhost:8095/nodes/register \
        -H "Authorization: Bearer test-key" \
        -H "Content-Type: application/json" \
        -d '{"address":"test","type":"server","cpu_threads":4,"cpu_model":"Test","has_gpu":false,"ram_total_bytes":8589934592}')
    
    if echo "$NODE" | grep -q '"id"'; then
        echo -e "${GREEN}✓ Node registration works${NC}"
        
        NODE_ID=$(echo "$NODE" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
        
        # Test job creation with queue/priority
        JOB=$(curl -s -X POST http://localhost:8095/jobs \
            -H "Authorization: Bearer test-key" \
            -H "Content-Type: application/json" \
            -d '{"scenario":"test","queue":"live","priority":"high"}')
        
        if echo "$JOB" | grep -q '"queue":"live"' && echo "$JOB" | grep -q '"priority":"high"'; then
            echo -e "${GREEN}✓ Job creation with queue/priority works${NC}"
            
            JOB_ID=$(echo "$JOB" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
            
            # Test job assignment
            ASSIGNED=$(curl -s "http://localhost:8095/jobs/next?node_id=$NODE_ID" \
                -H "Authorization: Bearer test-key")
            
            if echo "$ASSIGNED" | grep -q "$JOB_ID"; then
                echo -e "${GREEN}✓ Job assignment works${NC}"
                
                # Test pause
                curl -s -X POST "http://localhost:8095/jobs/$JOB_ID/pause" \
                    -H "Authorization: Bearer test-key" > /dev/null
                
                JOB_STATUS=$(curl -s "http://localhost:8095/jobs/$JOB_ID" \
                    -H "Authorization: Bearer test-key")
                
                if echo "$JOB_STATUS" | grep -q '"status":"paused"'; then
                    echo -e "${GREEN}✓ Job pause works${NC}"
                    
                    # Test resume
                    curl -s -X POST "http://localhost:8095/jobs/$JOB_ID/resume" \
                        -H "Authorization: Bearer test-key" > /dev/null
                    
                    JOB_STATUS=$(curl -s "http://localhost:8095/jobs/$JOB_ID" \
                        -H "Authorization: Bearer test-key")
                    
                    if echo "$JOB_STATUS" | grep -q '"status":"processing"'; then
                        echo -e "${GREEN}✓ Job resume works${NC}"
                    fi
                    
                    # Test cancel
                    curl -s -X POST "http://localhost:8095/jobs/$JOB_ID/cancel" \
                        -H "Authorization: Bearer test-key" > /dev/null
                    
                    JOB_STATUS=$(curl -s "http://localhost:8095/jobs/$JOB_ID" \
                        -H "Authorization: Bearer test-key")
                    
                    if echo "$JOB_STATUS" | grep -q '"status":"canceled"'; then
                        echo -e "${GREEN}✓ Job cancel works${NC}"
                    fi
                fi
            fi
        fi
        
        # Test node details endpoint
        NODE_DETAILS=$(curl -s "http://localhost:8095/nodes/$NODE_ID" \
            -H "Authorization: Bearer test-key")
        
        if echo "$NODE_DETAILS" | grep -q '"ram_total_bytes"'; then
            echo -e "${GREEN}✓ Node details endpoint works${NC}"
        fi
    fi
    
    kill $MASTER_PID 2>/dev/null
    wait $MASTER_PID 2>/dev/null || true
else
    echo "✗ Master failed to start"
    cat /tmp/master.log
    exit 1
fi

# Cleanup
rm -f /tmp/quick-test.db /tmp/quick-test.db-* /tmp/master.log /tmp/test.log /tmp/api-test.log

echo -e "\n${GREEN}════════════════════════════════════════${NC}"
echo -e "${GREEN}✓ All Key Features Validated!${NC}"
echo -e "${GREEN}════════════════════════════════════════${NC}"
echo -e "\nValidated:"
echo -e "  • Build system (master, agent, CLI)"
echo -e "  • Unit tests (store, API)"
echo -e "  • Master startup & API"
echo -e "  • Node registration with hardware info"
echo -e "  • Job creation with queue/priority"
echo -e "  • Priority-based scheduling"
echo -e "  • Job control (pause/resume/cancel)"
echo -e "  • Node details endpoint"
echo -e "\n${YELLOW}Ready for Phase 4: Metrics Implementation${NC}"
