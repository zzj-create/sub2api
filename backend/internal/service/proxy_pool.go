package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	ProxyPoolStatusActive   = "active"
	ProxyPoolStatusDisabled = "disabled"

	ProxyPoolHealthUnknown   = "unknown"
	ProxyPoolHealthHealthy   = "healthy"
	ProxyPoolHealthUnhealthy = "unhealthy"
)

var (
	ErrProxyPoolNotFound       = infraerrors.NotFound("PROXY_POOL_NOT_FOUND", "proxy pool not found")
	ErrProxyPoolNameExists     = infraerrors.Conflict("PROXY_POOL_NAME_EXISTS", "proxy pool name already exists")
	ErrProxyPoolNoHealthyProxy = infraerrors.Conflict("PROXY_POOL_NO_HEALTHY_PROXY", "proxy pool has no healthy proxy")
	ErrProxyPoolDisabled       = infraerrors.Conflict("PROXY_POOL_DISABLED", "proxy pool is disabled")
	ErrProxyPoolBusy           = infraerrors.Conflict("PROXY_POOL_BUSY", "proxy pool health check is already running")
	ErrProxyPoolBindBusy       = infraerrors.Conflict("PROXY_POOL_BIND_BUSY", "proxy pool account binding is already running")
	ErrProxyPoolGroupBindBusy  = infraerrors.Conflict("PROXY_POOL_GROUP_BIND_BUSY", "proxy pool group binding is already running")
	ErrProxyPoolGroupBound     = infraerrors.Conflict("PROXY_POOL_GROUP_ALREADY_BOUND", "one or more groups are already bound to another proxy pool")
	ErrProxyPoolGroupInvalid   = infraerrors.BadRequest("PROXY_POOL_GROUP_INVALID", "one or more groups are missing or inactive")
	ErrProxyPoolNameRequired   = infraerrors.BadRequest("PROXY_POOL_NAME_REQUIRED", "proxy pool name is required")
	ErrProxyPoolInvalidStatus  = infraerrors.BadRequest("PROXY_POOL_INVALID_STATUS", "invalid proxy pool status")
)

type ProxyPool struct {
	ID                    int64      `json:"id"`
	Name                  string     `json:"name"`
	Description           *string    `json:"description,omitempty"`
	Status                string     `json:"status"`
	HealthIntervalSeconds int        `json:"health_interval_seconds"`
	FailureThreshold      int        `json:"failure_threshold"`
	AutoRebind            bool       `json:"auto_rebind"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	DeletedAt             *time.Time `json:"deleted_at,omitempty"`
}

func (p *ProxyPool) IsActive() bool {
	return p != nil && p.Status == ProxyPoolStatusActive
}

func (p *ProxyPool) HealthInterval() time.Duration {
	if p == nil || p.HealthIntervalSeconds <= 0 {
		return 5 * time.Minute
	}
	return time.Duration(p.HealthIntervalSeconds) * time.Second
}

func (p *ProxyPool) FailureThresholdValue() int {
	if p == nil || p.FailureThreshold <= 0 {
		return 2
	}
	return p.FailureThreshold
}

type ProxyPoolWithStats struct {
	ProxyPool
	ProxyCount          int64 `json:"proxy_count"`
	HealthyProxyCount   int64 `json:"healthy_proxy_count"`
	UnhealthyProxyCount int64 `json:"unhealthy_proxy_count"`
	BoundAccountCount   int64 `json:"bound_account_count"`
	BoundGroupCount     int64 `json:"bound_group_count"`
}

// ProxyPoolGroup is a group bound to a pool or an option that can be bound.
// BoundPoolID/BoundPoolName are populated by the options endpoint so the UI
// can explain why a group cannot be selected for a second pool.
type ProxyPoolGroup struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Platform      string `json:"platform"`
	Status        string `json:"status"`
	AccountCount  int64  `json:"account_count"`
	BoundPoolID   *int64 `json:"bound_pool_id,omitempty"`
	BoundPoolName string `json:"bound_pool_name,omitempty"`
}

type ProxyPoolGroupBindResult struct {
	BoundGroups    int `json:"bound_groups"`
	SyncedAccounts int `json:"synced_accounts"`
}

type ProxyPoolGroupUnbindResult struct {
	UnboundGroups    int `json:"unbound_groups"`
	DetachedAccounts int `json:"detached_accounts"`
}

type ProxyPoolProxy struct {
	Proxy
	PoolID                int64      `json:"pool_id"`
	PoolHealth            string     `json:"pool_health"`
	PoolCheckedAt         *time.Time `json:"pool_checked_at,omitempty"`
	PoolFailures          int        `json:"pool_failures"`
	GrokQualityStatus     string     `json:"grok_quality_status"`
	GrokQualityCheckedAt  *time.Time `json:"grok_quality_checked_at,omitempty"`
	GrokQualityHTTPStatus *int       `json:"grok_quality_http_status,omitempty"`
	GrokQualityMessage    string     `json:"grok_quality_message,omitempty"`
	AccountCount          int64      `json:"account_count"`
	LatencyMs             *int64     `json:"latency_ms,omitempty"`
	IPAddress             string     `json:"ip_address,omitempty"`
	Country               string     `json:"country,omitempty"`
	CountryCode           string     `json:"country_code,omitempty"`
}

type ProxyPoolHealthSnapshot struct {
	Health                string
	Failures              int
	CheckedAt             time.Time
	GrokQualityStatus     string
	GrokQualityCheckedAt  *time.Time
	GrokQualityHTTPStatus *int
	GrokQualityMessage    string
}

type ProxyPoolRebindLog struct {
	ID            int64     `json:"id"`
	PoolID        int64     `json:"pool_id"`
	FromProxyID   *int64    `json:"from_proxy_id,omitempty"`
	ToProxyID     *int64    `json:"to_proxy_id,omitempty"`
	FromProxyName string    `json:"from_proxy_name,omitempty"`
	ToProxyName   string    `json:"to_proxy_name,omitempty"`
	AccountCount  int       `json:"account_count"`
	Reason        string    `json:"reason"`
	CreatedAt     time.Time `json:"created_at"`
}

type ProxyPoolAccountAssignment struct {
	AccountID int64 `json:"account_id"`
	ProxyID   int64 `json:"proxy_id"`
}

type ProxyPoolBindResult struct {
	Assigned int                          `json:"assigned"`
	Pending  int                          `json:"pending"`
	Failed   int                          `json:"failed"`
	Results  []ProxyPoolAccountAssignment `json:"results"`
}

type CreateProxyPoolInput struct {
	Name                  string
	Description           *string
	Status                string
	HealthIntervalSeconds int
	FailureThreshold      int
	AutoRebind            bool
}

type UpdateProxyPoolInput struct {
	Name                  *string
	Description           *string
	Status                *string
	HealthIntervalSeconds *int
	FailureThreshold      *int
	AutoRebind            *bool
}

type ProxyPoolRepository interface {
	CreatePool(ctx context.Context, pool *ProxyPool) (*ProxyPool, error)
	UpdatePool(ctx context.Context, pool *ProxyPool) error
	DeletePool(ctx context.Context, id int64) error
	GetPoolByID(ctx context.Context, id int64) (*ProxyPool, error)
	ListPools(ctx context.Context) ([]ProxyPool, error)
	ListPoolsWithStats(ctx context.Context) ([]ProxyPoolWithStats, error)
	ListPoolGroups(ctx context.Context, poolID int64) ([]ProxyPoolGroup, error)
	ListPoolGroupOptions(ctx context.Context, poolID int64) ([]ProxyPoolGroup, error)
	BindGroupsToPool(ctx context.Context, poolID int64, groupIDs []int64) (*ProxyPoolGroupBindResult, error)
	UnbindGroupsFromPool(ctx context.Context, poolID int64, groupIDs []int64) (*ProxyPoolGroupUnbindResult, error)
	SyncPoolGroupAccounts(ctx context.Context, poolID int64) (int64, error)

	ListPoolProxies(ctx context.Context, poolID int64) ([]ProxyPoolProxy, error)
	AssignProxiesToPool(ctx context.Context, poolID int64, proxyIDs []int64) (int64, error)
	RemoveProxiesFromPool(ctx context.Context, poolID int64, proxyIDs []int64) (int64, error)
	UpdateProxyPoolHealth(ctx context.Context, poolID, proxyID int64, snapshot ProxyPoolHealthSnapshot) error

	ListPoolUnassignedAccountIDs(ctx context.Context, poolID int64) ([]int64, error)
	ListAccountIDsByProxy(ctx context.Context, poolID, proxyID int64) ([]int64, error)
	CountAccountsByProxyIDs(ctx context.Context, proxyIDs []int64) (map[int64]int64, error)
	BindAccountsToPool(ctx context.Context, poolID int64, assignments []ProxyPoolAccountAssignment) ([]ProxyPoolAccountAssignment, error)
	MarkAccountsPendingInPool(ctx context.Context, poolID int64, accountIDs []int64) ([]int64, error)
	UnbindAccountsFromPool(ctx context.Context, poolID int64, accountIDs []int64) (int64, error)

	RecordRebindLog(ctx context.Context, entry *ProxyPoolRebindLog) error
	ListRebindLogs(ctx context.Context, poolID int64, limit int) ([]ProxyPoolRebindLog, error)
}
