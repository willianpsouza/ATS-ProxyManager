package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ats-proxy/proxy-manager/backend/internal/domain"
	"github.com/ats-proxy/proxy-manager/backend/internal/repository"
)

type ConfigService struct {
	pool         *pgxpool.Pool
	configs      *repository.ConfigRepo
	domains      *repository.DomainRuleRepo
	ipRanges     *repository.IPRangeRuleRepo
	parents      *repository.ParentProxyRepo
	clientACL    *repository.ClientACLRepo
	configProxy  *repository.ConfigProxyRepo
	audit        *repository.AuditRepo
}

func NewConfigService(
	pool *pgxpool.Pool,
	configs *repository.ConfigRepo,
	domains *repository.DomainRuleRepo,
	ipRanges *repository.IPRangeRuleRepo,
	parents *repository.ParentProxyRepo,
	clientACL *repository.ClientACLRepo,
	configProxy *repository.ConfigProxyRepo,
	audit *repository.AuditRepo,
) *ConfigService {
	return &ConfigService{
		pool:        pool,
		configs:     configs,
		domains:     domains,
		ipRanges:    ipRanges,
		parents:     parents,
		clientACL:   clientACL,
		configProxy: configProxy,
		audit:       audit,
	}
}

type ConfigDetail struct {
	domain.Config
	Domains       []domain.DomainRule    `json:"domains"`
	IPRanges      []domain.IPRangeRule   `json:"ip_ranges"`
	ParentProxies []domain.ParentProxy   `json:"parent_proxies"`
	ClientACL     []domain.ClientACLRule `json:"client_acl"`
	Proxies       []domain.Proxy         `json:"proxies"`
	ModifiedByUser  *UserResponse `json:"modified_by_user,omitempty"`
	ApprovedByUser  *UserResponse `json:"approved_by_user,omitempty"`
}

func (s *ConfigService) GetByID(ctx context.Context, id uuid.UUID) (*ConfigDetail, error) {
	cfg, err := s.configs.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	domains, err := s.domains.ListByConfig(ctx, id)
	if err != nil {
		return nil, err
	}
	ipRanges, err := s.ipRanges.ListByConfig(ctx, id)
	if err != nil {
		return nil, err
	}
	parents, err := s.parents.ListByConfig(ctx, id)
	if err != nil {
		return nil, err
	}
	clientACL, err := s.clientACL.ListByConfig(ctx, id)
	if err != nil {
		return nil, err
	}
	proxies, err := s.configProxy.ListByConfig(ctx, id)
	if err != nil {
		return nil, err
	}

	if domains == nil {
		domains = []domain.DomainRule{}
	}
	if ipRanges == nil {
		ipRanges = []domain.IPRangeRule{}
	}
	if parents == nil {
		parents = []domain.ParentProxy{}
	}
	if clientACL == nil {
		clientACL = []domain.ClientACLRule{}
	}
	if proxies == nil {
		proxies = []domain.Proxy{}
	}

	return &ConfigDetail{
		Config:        *cfg,
		Domains:       domains,
		IPRanges:      ipRanges,
		ParentProxies: parents,
		ClientACL:     clientACL,
		Proxies:       proxies,
	}, nil
}

func (s *ConfigService) Delete(ctx context.Context, id, userID uuid.UUID, ip, ua string) error {
	cfg, err := s.configs.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if cfg.Status == domain.StatusActive {
		return fmt.Errorf("%w: cannot delete an active config", domain.ErrBadRequest)
	}

	if cfg.ApprovedAt != nil {
		return fmt.Errorf("%w: cannot delete a config that was previously activated", domain.ErrBadRequest)
	}

	if err := s.configs.Delete(ctx, id); err != nil {
		return err
	}

	_ = s.audit.Create(ctx, &domain.AuditLog{
		UserID:     &userID,
		Action:     "config.delete",
		EntityType: "config",
		EntityID:   &id,
		IPAddress:  &ip,
		UserAgent:  &ua,
		OldValue:   []byte(fmt.Sprintf(`{"name":%q,"status":%q}`, cfg.Name, cfg.Status)),
	})

	return nil
}

func (s *ConfigService) List(ctx context.Context, status *domain.ConfigStatus, page, limit int) ([]domain.Config, int, error) {
	offset := (page - 1) * limit
	return s.configs.List(ctx, status, limit, offset)
}

type CreateConfigRequest struct {
	Name          string                    `json:"name"`
	Description   *string                   `json:"description,omitempty"`
	DefaultAction domain.RuleAction         `json:"default_action"`
	Domains       []DomainRuleInput         `json:"domains"`
	IPRanges      []IPRangeRuleInput        `json:"ip_ranges"`
	ParentProxies []ParentProxyInput        `json:"parent_proxies"`
	ClientACL     []ClientACLInput          `json:"client_acl"`
	ProxyIDs      []uuid.UUID               `json:"proxy_ids"`
}

type DomainRuleInput struct {
	Domain   string            `json:"domain"`
	Action   domain.RuleAction `json:"action"`
	Priority int               `json:"priority"`
}

type IPRangeRuleInput struct {
	CIDR     string            `json:"cidr"`
	Action   domain.RuleAction `json:"action"`
	Priority int               `json:"priority"`
}

type ParentProxyInput struct {
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Priority int    `json:"priority"`
	Enabled  bool   `json:"enabled"`
}

type ClientACLInput struct {
	CIDR     string           `json:"cidr"`
	Action   domain.ACLAction `json:"action"`
	Priority int              `json:"priority"`
}

var domainPattern = regexp.MustCompile(`^(\*\.)?[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)+$`)

func validateRules(req CreateConfigRequest) error {
	var errs []string

	// Validate default_action
	if req.DefaultAction == "" {
		req.DefaultAction = domain.ActionDirect
	}
	if !req.DefaultAction.IsValid() {
		errs = append(errs, fmt.Sprintf("default_action: '%s' is not valid, must be 'direct' or 'parent'", req.DefaultAction))
	}

	// Validate domain rules
	for i, d := range req.Domains {
		if d.Domain == "" {
			errs = append(errs, fmt.Sprintf("domains[%d]: domain cannot be empty", i))
			continue
		}
		if d.Domain == "*." || d.Domain == "*" {
			errs = append(errs, fmt.Sprintf("domains[%d]: '%s' total wildcard is not allowed", i, d.Domain))
			continue
		}
		if !domainPattern.MatchString(d.Domain) {
			errs = append(errs, fmt.Sprintf("domains[%d]: '%s' is not a valid domain (use *.example.com or host.example.com)", i, d.Domain))
		}
		if !d.Action.IsValid() {
			errs = append(errs, fmt.Sprintf("domains[%d]: action '%s' is not valid", i, d.Action))
		}
	}

	// Validate IP range rules
	for i, ir := range req.IPRanges {
		if ir.CIDR == "" {
			errs = append(errs, fmt.Sprintf("ip_ranges[%d]: CIDR cannot be empty", i))
			continue
		}
		if err := validateCIDR(ir.CIDR, fmt.Sprintf("ip_ranges[%d]", i)); err != "" {
			errs = append(errs, err)
		}
		if !ir.Action.IsValid() {
			errs = append(errs, fmt.Sprintf("ip_ranges[%d]: action '%s' is not valid", i, ir.Action))
		}
	}

	// Validate client ACL rules
	for i, acl := range req.ClientACL {
		if acl.CIDR == "" {
			errs = append(errs, fmt.Sprintf("client_acl[%d]: CIDR cannot be empty", i))
			continue
		}
		// Allow IPv6 in client ACL (e.g. ::1)
		ip := net.ParseIP(acl.CIDR)
		if ip != nil {
			// Valid bare IP — check for 0.0.0.0
			if acl.CIDR == "0.0.0.0" {
				errs = append(errs, fmt.Sprintf("client_acl[%d]: 0.0.0.0 is not allowed", i))
			}
		} else {
			_, ipnet, err := net.ParseCIDR(acl.CIDR)
			if err != nil {
				errs = append(errs, fmt.Sprintf("client_acl[%d]: '%s' is not a valid CIDR or IP address", i, acl.CIDR))
			} else if ipnet.IP.To4() != nil && ipnet.IP.Equal(net.IPv4zero) {
				errs = append(errs, fmt.Sprintf("client_acl[%d]: 0.0.0.0/%d is not allowed", i, maskSize(ipnet)))
			}
		}
		if !acl.Action.IsValid() {
			errs = append(errs, fmt.Sprintf("client_acl[%d]: action '%s' is not valid", i, acl.Action))
		}
	}

	// Validate parent proxies
	for i, pp := range req.ParentProxies {
		if pp.Address == "" {
			errs = append(errs, fmt.Sprintf("parent_proxies[%d]: address cannot be empty", i))
		} else if net.ParseIP(pp.Address) == nil {
			errs = append(errs, fmt.Sprintf("parent_proxies[%d]: '%s' is not a valid IP address", i, pp.Address))
		}
		if pp.Port < 1024 || pp.Port > 65535 {
			errs = append(errs, fmt.Sprintf("parent_proxies[%d]: port %d is out of range (1024-65535)", i, pp.Port))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("%w: %s", domain.ErrBadRequest, strings.Join(errs, "; "))
	}
	return nil
}

func validateCIDR(cidr, prefix string) string {
	ip := net.ParseIP(cidr)
	if ip != nil {
		// Bare IP is valid
		if ip.To4() != nil && ip.Equal(net.IPv4zero) {
			return fmt.Sprintf("%s: 0.0.0.0 is not allowed", prefix)
		}
		return ""
	}

	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Sprintf("%s: '%s' is not a valid CIDR or IP address", prefix, cidr)
	}
	if ipnet.IP.To4() != nil && ipnet.IP.Equal(net.IPv4zero) {
		return fmt.Sprintf("%s: 0.0.0.0/%d is not allowed", prefix, maskSize(ipnet))
	}
	return ""
}

func maskSize(ipnet *net.IPNet) int {
	ones, _ := ipnet.Mask.Size()
	return ones
}

func (s *ConfigService) Create(ctx context.Context, req CreateConfigRequest, userID uuid.UUID, ip, ua string) (*ConfigDetail, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("%w: name is required", domain.ErrBadRequest)
	}

	// Default default_action to "direct" if empty
	if req.DefaultAction == "" {
		req.DefaultAction = domain.ActionDirect
	}

	if err := validateRules(req); err != nil {
		return nil, err
	}

	var result *ConfigDetail

	err := repository.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		txConfigs := repository.NewConfigRepo(tx)
		txDomains := repository.NewDomainRuleRepo(tx)
		txIPRanges := repository.NewIPRangeRuleRepo(tx)
		txParents := repository.NewParentProxyRepo(tx)
		txClientACL := repository.NewClientACLRepo(tx)
		txConfigProxy := repository.NewConfigProxyRepo(tx)
		txAudit := repository.NewAuditRepo(tx)

		cfg := &domain.Config{
			Name:          req.Name,
			Description:   req.Description,
			DefaultAction: req.DefaultAction,
			CreatedBy:     &userID,
		}
		if err := txConfigs.Create(ctx, cfg); err != nil {
			return err
		}

		domains := make([]domain.DomainRule, 0, len(req.Domains))
		for _, d := range req.Domains {
			dr := domain.DomainRule{ConfigID: cfg.ID, Domain: d.Domain, Action: d.Action, Priority: d.Priority}
			if err := txDomains.Create(ctx, &dr); err != nil {
				return err
			}
			domains = append(domains, dr)
		}

		ipRanges := make([]domain.IPRangeRule, 0, len(req.IPRanges))
		for _, ir := range req.IPRanges {
			rule := domain.IPRangeRule{ConfigID: cfg.ID, CIDR: ir.CIDR, Action: ir.Action, Priority: ir.Priority}
			if err := txIPRanges.Create(ctx, &rule); err != nil {
				return err
			}
			ipRanges = append(ipRanges, rule)
		}

		parents := make([]domain.ParentProxy, 0, len(req.ParentProxies))
		for _, pp := range req.ParentProxies {
			proxy := domain.ParentProxy{ConfigID: cfg.ID, Address: pp.Address, Port: pp.Port, Priority: pp.Priority, Enabled: pp.Enabled}
			if err := txParents.Create(ctx, &proxy); err != nil {
				return err
			}
			parents = append(parents, proxy)
		}

		// Client ACL rules — insert defaults if none provided
		aclInputs := req.ClientACL
		if len(aclInputs) == 0 {
			aclInputs = []ClientACLInput{
				{CIDR: "127.0.0.1", Action: domain.ACLAllow, Priority: 10},
				{CIDR: "::1", Action: domain.ACLAllow, Priority: 20},
				{CIDR: "10.0.0.0/8", Action: domain.ACLAllow, Priority: 30},
			}
		}
		clientACL := make([]domain.ClientACLRule, 0, len(aclInputs))
		for _, acl := range aclInputs {
			rule := domain.ClientACLRule{ConfigID: cfg.ID, CIDR: acl.CIDR, Action: acl.Action, Priority: acl.Priority}
			if err := txClientACL.Create(ctx, &rule); err != nil {
				return err
			}
			clientACL = append(clientACL, rule)
		}

		for _, pid := range req.ProxyIDs {
			if err := txConfigProxy.Assign(ctx, cfg.ID, pid, userID); err != nil {
				return err
			}
		}

		_ = txAudit.Create(ctx, &domain.AuditLog{
			UserID:     &userID,
			Action:     "config.create",
			EntityType: "config",
			EntityID:   &cfg.ID,
			IPAddress:  &ip,
			UserAgent:  &ua,
		})

		result = &ConfigDetail{
			Config:        *cfg,
			Domains:       domains,
			IPRanges:      ipRanges,
			ParentProxies: parents,
			ClientACL:     clientACL,
			Proxies:       []domain.Proxy{},
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *ConfigService) Update(ctx context.Context, id uuid.UUID, req CreateConfigRequest, userID uuid.UUID, ip, ua string) (*ConfigDetail, error) {
	// Default default_action to "direct" if empty
	if req.DefaultAction == "" {
		req.DefaultAction = domain.ActionDirect
	}

	if err := validateRules(req); err != nil {
		return nil, err
	}

	var result *ConfigDetail

	err := repository.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		txConfigs := repository.NewConfigRepo(tx)
		txDomains := repository.NewDomainRuleRepo(tx)
		txIPRanges := repository.NewIPRangeRuleRepo(tx)
		txParents := repository.NewParentProxyRepo(tx)
		txClientACL := repository.NewClientACLRepo(tx)
		txConfigProxy := repository.NewConfigProxyRepo(tx)
		txAudit := repository.NewAuditRepo(tx)

		cfg, err := txConfigs.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if cfg.Status != domain.StatusDraft {
			return fmt.Errorf("%w: can only edit configs in draft status", domain.ErrInvalidStatus)
		}

		cfg.Name = req.Name
		cfg.Description = req.Description
		cfg.DefaultAction = req.DefaultAction
		cfg.ModifiedBy = &userID
		if err := txConfigs.Update(ctx, cfg); err != nil {
			return err
		}

		// Replace all rules
		if err := txDomains.DeleteByConfig(ctx, id); err != nil {
			return err
		}
		if err := txIPRanges.DeleteByConfig(ctx, id); err != nil {
			return err
		}
		if err := txParents.DeleteByConfig(ctx, id); err != nil {
			return err
		}
		if err := txClientACL.DeleteByConfig(ctx, id); err != nil {
			return err
		}
		if err := txConfigProxy.DeleteByConfig(ctx, id); err != nil {
			return err
		}

		domains := make([]domain.DomainRule, 0, len(req.Domains))
		for _, d := range req.Domains {
			dr := domain.DomainRule{ConfigID: id, Domain: d.Domain, Action: d.Action, Priority: d.Priority}
			if err := txDomains.Create(ctx, &dr); err != nil {
				return err
			}
			domains = append(domains, dr)
		}

		ipRanges := make([]domain.IPRangeRule, 0, len(req.IPRanges))
		for _, ir := range req.IPRanges {
			rule := domain.IPRangeRule{ConfigID: id, CIDR: ir.CIDR, Action: ir.Action, Priority: ir.Priority}
			if err := txIPRanges.Create(ctx, &rule); err != nil {
				return err
			}
			ipRanges = append(ipRanges, rule)
		}

		parents := make([]domain.ParentProxy, 0, len(req.ParentProxies))
		for _, pp := range req.ParentProxies {
			proxy := domain.ParentProxy{ConfigID: id, Address: pp.Address, Port: pp.Port, Priority: pp.Priority, Enabled: pp.Enabled}
			if err := txParents.Create(ctx, &proxy); err != nil {
				return err
			}
			parents = append(parents, proxy)
		}

		clientACL := make([]domain.ClientACLRule, 0, len(req.ClientACL))
		for _, acl := range req.ClientACL {
			rule := domain.ClientACLRule{ConfigID: id, CIDR: acl.CIDR, Action: acl.Action, Priority: acl.Priority}
			if err := txClientACL.Create(ctx, &rule); err != nil {
				return err
			}
			clientACL = append(clientACL, rule)
		}

		for _, pid := range req.ProxyIDs {
			if err := txConfigProxy.Assign(ctx, id, pid, userID); err != nil {
				return err
			}
		}

		_ = txAudit.Create(ctx, &domain.AuditLog{
			UserID:     &userID,
			Action:     "config.update",
			EntityType: "config",
			EntityID:   &id,
			IPAddress:  &ip,
			UserAgent:  &ua,
		})

		updated, err := txConfigs.GetByID(ctx, id)
		if err != nil {
			return err
		}

		result = &ConfigDetail{
			Config:        *updated,
			Domains:       domains,
			IPRanges:      ipRanges,
			ParentProxies: parents,
			ClientACL:     clientACL,
			Proxies:       []domain.Proxy{},
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *ConfigService) Clone(ctx context.Context, id, userID uuid.UUID, ip, ua string) (*ConfigDetail, error) {
	var result *ConfigDetail

	err := repository.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		txConfigs := repository.NewConfigRepo(tx)
		txDomains := repository.NewDomainRuleRepo(tx)
		txIPRanges := repository.NewIPRangeRuleRepo(tx)
		txParents := repository.NewParentProxyRepo(tx)
		txClientACL := repository.NewClientACLRepo(tx)
		txConfigProxy := repository.NewConfigProxyRepo(tx)
		txAudit := repository.NewAuditRepo(tx)

		// Load original config
		original, err := txConfigs.GetByID(ctx, id)
		if err != nil {
			return err
		}

		// Create new config with incremented version
		newCfg := &domain.Config{
			Name:          original.Name,
			Description:   original.Description,
			DefaultAction: original.DefaultAction,
			CreatedBy:     &userID,
		}
		if err := txConfigs.CreateWithVersion(ctx, newCfg, original.Version+1); err != nil {
			return err
		}

		// Copy domain rules
		origDomains, err := txDomains.ListByConfig(ctx, id)
		if err != nil {
			return err
		}
		domains := make([]domain.DomainRule, 0, len(origDomains))
		for _, d := range origDomains {
			dr := domain.DomainRule{ConfigID: newCfg.ID, Domain: d.Domain, Action: d.Action, Priority: d.Priority}
			if err := txDomains.Create(ctx, &dr); err != nil {
				return err
			}
			domains = append(domains, dr)
		}

		// Copy IP range rules
		origIPRanges, err := txIPRanges.ListByConfig(ctx, id)
		if err != nil {
			return err
		}
		ipRanges := make([]domain.IPRangeRule, 0, len(origIPRanges))
		for _, ir := range origIPRanges {
			rule := domain.IPRangeRule{ConfigID: newCfg.ID, CIDR: ir.CIDR, Action: ir.Action, Priority: ir.Priority}
			if err := txIPRanges.Create(ctx, &rule); err != nil {
				return err
			}
			ipRanges = append(ipRanges, rule)
		}

		// Copy parent proxies
		origParents, err := txParents.ListByConfig(ctx, id)
		if err != nil {
			return err
		}
		parents := make([]domain.ParentProxy, 0, len(origParents))
		for _, pp := range origParents {
			proxy := domain.ParentProxy{ConfigID: newCfg.ID, Address: pp.Address, Port: pp.Port, Priority: pp.Priority, Enabled: pp.Enabled}
			if err := txParents.Create(ctx, &proxy); err != nil {
				return err
			}
			parents = append(parents, proxy)
		}

		// Copy client ACL rules
		origACL, err := txClientACL.ListByConfig(ctx, id)
		if err != nil {
			return err
		}
		clientACL := make([]domain.ClientACLRule, 0, len(origACL))
		for _, acl := range origACL {
			rule := domain.ClientACLRule{ConfigID: newCfg.ID, CIDR: acl.CIDR, Action: acl.Action, Priority: acl.Priority}
			if err := txClientACL.Create(ctx, &rule); err != nil {
				return err
			}
			clientACL = append(clientACL, rule)
		}

		// Copy config_proxies assignments
		origProxies, err := txConfigProxy.ListByConfig(ctx, id)
		if err != nil {
			return err
		}
		for _, p := range origProxies {
			if err := txConfigProxy.Assign(ctx, newCfg.ID, p.ID, userID); err != nil {
				return err
			}
		}

		_ = txAudit.Create(ctx, &domain.AuditLog{
			UserID:     &userID,
			Action:     "config.clone",
			EntityType: "config",
			EntityID:   &newCfg.ID,
			OldValue:   jsonVal("source_id", id.String()),
			IPAddress:  &ip,
			UserAgent:  &ua,
		})

		result = &ConfigDetail{
			Config:        *newCfg,
			Domains:       domains,
			IPRanges:      ipRanges,
			ParentProxies: parents,
			ClientACL:     clientACL,
			Proxies:       origProxies,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *ConfigService) Submit(ctx context.Context, id, userID uuid.UUID, ip, ua string) (*domain.Config, error) {
	if err := s.configs.Submit(ctx, id, userID); err != nil {
		return nil, err
	}

	_ = s.audit.Create(ctx, &domain.AuditLog{
		UserID:     &userID,
		Action:     "config.submit",
		EntityType: "config",
		EntityID:   &id,
		OldValue:   jsonVal("status", "draft"),
		NewValue:   jsonVal("status", "pending_approval"),
		IPAddress:  &ip,
		UserAgent:  &ua,
	})

	return s.configs.GetByID(ctx, id)
}

func (s *ConfigService) Approve(ctx context.Context, id, userID uuid.UUID, ip, ua string) (*domain.Config, error) {
	cfg, err := s.configs.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if cfg.Status != domain.StatusPendingApproval {
		return nil, fmt.Errorf("%w: config is not pending approval", domain.ErrInvalidStatus)
	}
	if cfg.SubmittedBy == nil || *cfg.SubmittedBy != userID {
		return nil, fmt.Errorf("%w: approval must be done by the same user who submitted", domain.ErrForbidden)
	}

	// Generate config hash
	hash, err := s.GenerateConfigHash(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("generate config hash: %w", err)
	}

	// Deactivate other active configs
	if err := s.configs.DeactivateOthers(ctx, id); err != nil {
		return nil, fmt.Errorf("deactivate others: %w", err)
	}

	if err := s.configs.Approve(ctx, id, userID, hash); err != nil {
		return nil, err
	}

	_ = s.audit.Create(ctx, &domain.AuditLog{
		UserID:     &userID,
		Action:     "config.approve",
		EntityType: "config",
		EntityID:   &id,
		OldValue:   jsonVal("status", "pending_approval"),
		NewValue:   jsonVal("status", "active"),
		IPAddress:  &ip,
		UserAgent:  &ua,
	})

	return s.configs.GetByID(ctx, id)
}

func (s *ConfigService) Reject(ctx context.Context, id, userID uuid.UUID, reason, ip, ua string) (*domain.Config, error) {
	if err := s.configs.Reject(ctx, id); err != nil {
		return nil, err
	}

	_ = s.audit.Create(ctx, &domain.AuditLog{
		UserID:     &userID,
		Action:     "config.reject",
		EntityType: "config",
		EntityID:   &id,
		OldValue:   jsonVal("status", "pending_approval"),
		NewValue:   jsonVal("status", "draft"),
		IPAddress:  &ip,
		UserAgent:  &ua,
	})

	return s.configs.GetByID(ctx, id)
}

// GenerateConfigHash computes SHA256 of the generated parent.config + sni.yaml + ip_allow.yaml content.
func (s *ConfigService) GenerateConfigHash(ctx context.Context, configID uuid.UUID) (string, error) {
	parentConfig, sniYaml, ipAllowYaml, err := s.GenerateConfigFiles(ctx, configID)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write([]byte(parentConfig))
	h.Write([]byte(sniYaml))
	h.Write([]byte(ipAllowYaml))
	return hex.EncodeToString(h.Sum(nil)), nil
}

// GenerateConfigFiles generates parent.config, sni.yaml, and ip_allow.yaml from DB rules.
func (s *ConfigService) GenerateConfigFiles(ctx context.Context, configID uuid.UUID) (parentConfig, sniYaml, ipAllowYaml string, err error) {
	domains, err := s.domains.ListByConfig(ctx, configID)
	if err != nil {
		return "", "", "", err
	}
	ipRanges, err := s.ipRanges.ListByConfig(ctx, configID)
	if err != nil {
		return "", "", "", err
	}
	parents, err := s.parents.ListByConfig(ctx, configID)
	if err != nil {
		return "", "", "", err
	}

	clientACL, err := s.clientACL.ListByConfig(ctx, configID)
	if err != nil {
		return "", "", "", err
	}

	cfg, cfgErr := s.configs.GetByID(ctx, configID)
	if cfgErr != nil {
		return "", "", "", cfgErr
	}

	parentConfig = generateParentConfig(ipRanges, domains, parents, cfg.DefaultAction)
	sniYaml = generateSNIYaml(domains)
	ipAllowYaml = generateIPAllowYaml(clientACL)

	return parentConfig, sniYaml, ipAllowYaml, nil
}

// domainToATS converts user-facing domain format to ATS parent.config format.
// *.example.com → .example.com (ATS uses leading dot for wildcard)
// example.com → example.com (exact match, no change)
func domainToATS(domain string) string {
	if strings.HasPrefix(domain, "*.") {
		return domain[1:] // *.example.com → .example.com
	}
	return domain
}

func generateParentConfig(ipRanges []domain.IPRangeRule, domainRules []domain.DomainRule, parentProxies []domain.ParentProxy, defaultAction domain.RuleAction) string {
	var b strings.Builder

	// Sort by priority
	sort.Slice(ipRanges, func(i, j int) bool { return ipRanges[i].Priority < ipRanges[j].Priority })
	sort.Slice(domainRules, func(i, j int) bool { return domainRules[i].Priority < domainRules[j].Priority })
	sort.Slice(parentProxies, func(i, j int) bool { return parentProxies[i].Priority < parentProxies[j].Priority })

	// Build parent list (needed for parent rules)
	var enabled []domain.ParentProxy
	for _, pp := range parentProxies {
		if pp.Enabled {
			enabled = append(enabled, pp)
		}
	}
	var parentStr string
	if len(enabled) > 0 {
		var parentList []string
		for _, pp := range enabled {
			parentList = append(parentList, fmt.Sprintf("%s:%d", pp.Address, pp.Port))
		}
		parentStr = strings.Join(parentList, ";")
	}

	// --- Infrastructure rules (always present) ---
	b.WriteString("# Localhost\n")
	b.WriteString("dest_ip=127.0.0.0-127.255.255.255 go_direct=true\n")
	b.WriteString("# Link-local\n")
	b.WriteString("dest_ip=169.254.0.0-169.254.255.255 go_direct=true\n")
	b.WriteString("# Kubernetes\n")
	b.WriteString("dest_domain=.svc.cluster.local go_direct=true\n")
	b.WriteString("dest_domain=.cluster.local go_direct=true\n")
	b.WriteString("dest_domain=localhost go_direct=true\n")
	b.WriteString("\n")

	// --- User-defined IP range rules → dest_ip lines ---
	for _, ir := range ipRanges {
		ipRange := cidrToRange(ir.CIDR)
		if ir.Action == domain.ActionDirect {
			b.WriteString(fmt.Sprintf("dest_ip=%s go_direct=true\n", ipRange))
		} else if ir.Action == domain.ActionParent && parentStr != "" {
			b.WriteString(fmt.Sprintf("dest_ip=%s parent=\"%s\" round_robin=strict go_direct=false\n", ipRange, parentStr))
		}
	}

	// --- User-defined domain rules → dest_domain lines ---
	for _, dr := range domainRules {
		atsDomain := domainToATS(dr.Domain)
		if dr.Action == domain.ActionDirect {
			b.WriteString(fmt.Sprintf("dest_domain=%s go_direct=true\n", atsDomain))
		} else if dr.Action == domain.ActionParent && parentStr != "" {
			b.WriteString(fmt.Sprintf("dest_domain=%s parent=\"%s\" round_robin=strict go_direct=false\n", atsDomain, parentStr))
		}
	}

	// --- Default rule based on default_action ---
	if defaultAction == domain.ActionParent && parentStr != "" {
		b.WriteString(fmt.Sprintf("dest_domain=. parent=\"%s\" round_robin=strict go_direct=false\n", parentStr))
	} else {
		b.WriteString("dest_domain=. go_direct=true\n")
	}

	return b.String()
}

// domainToSNI converts domain to sni.yaml fqdn format.
// *.example.com stays *.example.com
// .example.com → *.example.com
// example.com stays example.com
func domainToSNI(domain string) string {
	if strings.HasPrefix(domain, ".") {
		return "*" + domain
	}
	return domain
}

func generateSNIYaml(domainRules []domain.DomainRule) string {
	var b strings.Builder
	b.WriteString("sni:\n")

	sort.Slice(domainRules, func(i, j int) bool { return domainRules[i].Priority < domainRules[j].Priority })

	for _, dr := range domainRules {
		if dr.Action == domain.ActionDirect {
			fqdn := domainToSNI(dr.Domain)
			b.WriteString(fmt.Sprintf("  - fqdn: '%s'\n    tunnel_route: direct\n", fqdn))
		}
	}

	return b.String()
}

// cidrToRange converts CIDR notation to IP range (e.g., 10.0.0.0/8 → 10.0.0.0-10.255.255.255)
func cidrToRange(cidr string) string {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return cidr
	}

	start := ipnet.IP.To4()
	if start == nil {
		return cidr
	}

	mask := ipnet.Mask
	end := make(net.IP, len(start))
	for i := range start {
		end[i] = start[i] | ^mask[i]
	}

	return fmt.Sprintf("%s-%s", start.String(), end.String())
}

func generateIPAllowYaml(rules []domain.ClientACLRule) string {
	var b strings.Builder
	b.WriteString("ip_allow:\n")

	sort.Slice(rules, func(i, j int) bool { return rules[i].Priority < rules[j].Priority })

	for _, r := range rules {
		atsAction := "set_allow"
		if r.Action == domain.ACLDeny {
			atsAction = "set_deny"
		}
		b.WriteString(fmt.Sprintf("  - apply: in\n    ip_addrs: %s\n    action: %s\n    methods: ALL\n", r.CIDR, atsAction))
	}

	// Always append deny-all for both IPv4 and IPv6
	b.WriteString("  - apply: in\n    ip_addrs: 0/0\n    action: set_deny\n    methods: ALL\n")
	b.WriteString("  - apply: in\n    ip_addrs: ::/0\n    action: set_deny\n    methods: ALL\n")

	return b.String()
}

func jsonVal(key, value string) []byte {
	data, _ := json.Marshal(map[string]string{key: value})
	return data
}
