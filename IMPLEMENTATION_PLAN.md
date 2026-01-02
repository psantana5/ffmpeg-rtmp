# Enterprise Features Implementation Plan

## Overview
This document outlines the implementation plan for making ffmpeg-rtmp enterprise-ready.

**Total Estimated Time**: 8-12 weeks (1-2 engineers)  
**Phases**: 4 major phases  
**Impact**: Enable serving 100+ customers with HA and RBAC

---

## Phase 1: PostgreSQL Support (Week 1-2)

### Goals
- Replace SQLite with PostgreSQL for scalability
- Support both SQLite (dev) and PostgreSQL (prod)
- Maintain backward compatibility

### Tasks

#### 1.1 Abstract Database Interface
- [ ] Create `store.DatabaseStore` interface
- [ ] Refactor SQLiteStore to implement interface
- [ ] Add configuration for database type selection

#### 1.2 PostgreSQL Implementation
- [ ] Create `PostgreSQLStore` struct
- [ ] Implement all store methods for PostgreSQL
- [ ] Add connection pooling
- [ ] Add prepared statements

#### 1.3 Schema & Migrations
- [ ] Port SQLite schema to PostgreSQL
- [ ] Create migration tool (migrate library)
- [ ] Add version tracking
- [ ] Test migration from SQLite to PostgreSQL

#### 1.4 Testing
- [ ] Port all SQLite tests to PostgreSQL
- [ ] Add integration tests with real PostgreSQL
- [ ] Benchmark: SQLite vs PostgreSQL
- [ ] Load test: 10K jobs, 100 workers

### Deliverables
- `shared/pkg/store/postgres.go`
- `shared/pkg/store/interface.go`
- `shared/pkg/migrations/`
- Docker Compose with PostgreSQL
- Migration guide

---

## Phase 2: Multi-Tenancy (Week 3-4)

### Goals
- Isolate customers/organizations
- Add tenant management API
- Enforce tenant isolation in all queries

### Tasks

#### 2.1 Data Model
- [ ] Add `tenants` table (id, name, created_at, plan, limits)
- [ ] Add `tenant_id` to jobs, nodes, users tables
- [ ] Add `tenant_quotas` table (max_jobs, max_workers, max_cpu)
- [ ] Create indexes on tenant_id

#### 2.2 Tenant Management
- [ ] Create Tenant model
- [ ] Add CreateTenant, GetTenant, ListTenants APIs
- [ ] Add UpdateTenant, DeleteTenant APIs
- [ ] Add tenant API keys (tenant-specific)

#### 2.3 Query Isolation
- [ ] Add tenant_id filter to all queries
- [ ] Create middleware for tenant context
- [ ] Update scheduler to respect tenant boundaries
- [ ] Update CLI to support --tenant flag

#### 2.4 Quotas & Limits
- [ ] Check quotas before job creation
- [ ] Enforce max concurrent jobs per tenant
- [ ] Enforce max workers per tenant
- [ ] Add quota exceeded errors

### Deliverables
- `shared/pkg/models/tenant.go`
- `shared/pkg/tenancy/middleware.go`
- Updated store with tenant filtering
- CLI support for multi-tenancy
- Tenant management APIs

---

## Phase 3: RBAC (Week 5-6)

### Goals
- Add user management
- Implement role-based permissions
- Integrate with tenant system

### Tasks

#### 3.1 User Management
- [ ] Add `users` table (id, email, tenant_id, role, created_at)
- [ ] Add `api_keys` table (key, user_id, permissions, expires_at)
- [ ] Create User model
- [ ] Add CreateUser, GetUser, ListUsers APIs

#### 3.2 Roles & Permissions
- [ ] Define roles: SuperAdmin, TenantAdmin, Operator, Viewer
- [ ] Define permissions: jobs.create, jobs.cancel, jobs.view, etc.
- [ ] Create permission checker middleware
- [ ] Add role inheritance (Admin inherits Operator)

#### 3.3 Authorization
- [ ] Add permission checks to all APIs
- [ ] Update CLI to authenticate as user
- [ ] Add audit logging (who did what)
- [ ] Add API key rotation

#### 3.4 Integration
- [ ] Link users to tenants
- [ ] SuperAdmin can manage all tenants
- [ ] TenantAdmin can only manage own tenant
- [ ] Add user invitation flow

### Deliverables
- `shared/pkg/models/user.go`
- `shared/pkg/auth/rbac.go`
- `shared/pkg/auth/permissions.go`
- User management APIs
- Audit logging system

---

## Phase 4: High Availability (Week 7-8+)

### Goals
- Eliminate single point of failure
- Deploy on Kubernetes
- Add health checks and automatic recovery

### Tasks

#### 4.1 Stateless Master Design
- [ ] Move all state to PostgreSQL
- [ ] Remove local file dependencies
- [ ] Add distributed locking (PostgreSQL advisory locks)
- [ ] Test multiple masters with same database

#### 4.2 Kubernetes Deployment
- [ ] Create Kubernetes manifests
- [ ] Add Deployment for master (3 replicas)
- [ ] Add StatefulSet for PostgreSQL
- [ ] Add Service + LoadBalancer
- [ ] Add ConfigMap for configuration

#### 4.3 Health & Readiness
- [ ] Add /health endpoint (liveness probe)
- [ ] Add /ready endpoint (readiness probe)
- [ ] Check database connectivity
- [ ] Check worker connectivity

#### 4.4 Leader Election (Optional)
- [ ] Add leader election (if needed for background jobs)
- [ ] Use PostgreSQL advisory locks
- [ ] Ensure only one master runs scheduler
- [ ] Test failover

### Deliverables
- `deployment/k8s/master-deployment.yaml`
- `deployment/k8s/postgres-statefulset.yaml`
- `deployment/k8s/service.yaml`
- Health check endpoints
- HA deployment guide

---

## Quick Wins (Parallel Work)

These can be done in parallel with above phases:

### API Versioning (1 week)
- [ ] Add `/v1/` prefix to all routes
- [ ] Create API version middleware
- [ ] Document versioning policy
- [ ] Add deprecation headers

### OpenAPI Spec (1 week)
- [ ] Generate OpenAPI 3.0 spec
- [ ] Add Swagger UI endpoint
- [ ] Document all endpoints
- [ ] Add request/response examples

### Resource Limits (1 week)
- [ ] Add max_cpu, max_memory to job parameters
- [ ] Enforce limits in worker
- [ ] Add timeout for resource acquisition
- [ ] Report resource usage in metrics

---

## Testing Strategy

### Unit Tests
- Test all new store methods
- Test tenant isolation
- Test permission checking
- Test quota enforcement

### Integration Tests
- Test full flow with PostgreSQL
- Test multi-tenant job submission
- Test RBAC enforcement
- Test Kubernetes deployment

### Load Tests
- 10K jobs with 100 workers
- 1000 jobs/second submission rate
- 10 tenants with 1000 jobs each
- Master failover during load

---

## Migration Path

### For Existing Users

#### Step 1: Upgrade to PostgreSQL
```bash
# Backup SQLite
cp master.db master.db.backup

# Run migration tool
./bin/migrate --from sqlite://master.db --to postgres://localhost/ffmpeg

# Verify data
./bin/ffrtmp jobs status
```

#### Step 2: Create Default Tenant
```bash
# Existing installation becomes default tenant
./bin/ffrtmp tenants create --name "default" --migrate-existing

# All existing jobs/workers assigned to default tenant
```

#### Step 3: Enable RBAC
```bash
# Create admin user
./bin/ffrtmp users create --email admin@company.com --role admin

# Generate API key
./bin/ffrtmp users create-key --user admin@company.com
```

#### Step 4: Deploy on Kubernetes
```bash
# Apply Kubernetes manifests
kubectl apply -f deployment/k8s/

# Verify deployment
kubectl get pods -n ffmpeg-rtmp
```

---

## Success Criteria

### Phase 1 Complete When:
- [ ] PostgreSQL store passes all tests
- [ ] Can handle 10K jobs without slowdown
- [ ] Migration tool works on sample data
- [ ] Documentation updated

### Phase 2 Complete When:
- [ ] Can create and manage tenants
- [ ] Jobs isolated by tenant
- [ ] Quotas enforced
- [ ] CLI supports multi-tenancy

### Phase 3 Complete When:
- [ ] Can create users with roles
- [ ] Permissions enforced on all APIs
- [ ] Audit log captures all actions
- [ ] API key management works

### Phase 4 Complete When:
- [ ] 3 masters running on Kubernetes
- [ ] Can kill 1 master without downtime
- [ ] Health checks working
- [ ] Load balancer distributes traffic

---

## Risk Mitigation

### Risk: Breaking Changes
**Mitigation**: 
- Maintain backward compatibility
- Add feature flags for new features
- Provide migration guide
- Keep SQLite support for development

### Risk: Performance Regression
**Mitigation**:
- Benchmark before and after
- Use connection pooling
- Add database indexes
- Load test before release

### Risk: Security Vulnerabilities
**Mitigation**:
- Code review all auth changes
- Penetration testing
- Follow OWASP guidelines
- Regular security audits

### Risk: Complexity
**Mitigation**:
- Start simple (basic RBAC, simple HA)
- Iterate based on feedback
- Document everything
- Provide examples

---

## Rollout Strategy

### Week 1-2: PostgreSQL
Deploy to staging, test with real workloads

### Week 3-4: Multi-Tenancy
Beta test with 2-3 friendly customers

### Week 5-6: RBAC
Security audit, penetration testing

### Week 7-8: HA on K8s
Deploy to production with monitoring

### Week 9+: Iterate
Fix bugs, add features based on feedback

---

## Resources Needed

### Development
- 1-2 backend engineers (Go)
- Access to PostgreSQL instance
- Kubernetes cluster for testing
- Load testing infrastructure

### Tools
- PostgreSQL 14+
- Kubernetes 1.24+
- Helm (optional)
- migrate tool
- k6 or Gatling for load testing

### Documentation
- API documentation
- Migration guides
- Deployment runbooks
- Security guidelines

---

## Next Steps

**Ready to start?** 

Let's begin with Phase 1: PostgreSQL implementation. I'll:
1. Create the store interface
2. Implement PostgreSQL store
3. Add migration tool
4. Update tests

**Estimated time for Phase 1**: 2 weeks with 1 engineer

Should I proceed with the implementation? 🚀
