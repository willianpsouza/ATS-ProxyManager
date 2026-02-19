package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
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
	configProxy  *repository.ConfigProxyRepo
	audit        *repository.AuditRepo
}

func NewConfigService(
	pool *pgxpool.Pool,
	configs *repository.ConfigRepo,
	domains *repository.DomainRuleRepo,
	ipRanges *repository.IPRangeRuleRepo,
	parents *repository.ParentProxyRepo,
	configProxy *repository.ConfigProxyRepo,
	audit *repository.AuditRepo,
) *ConfigService {
	return &ConfigService{
		pool:        pool,
		configs:     configs,
		domains:     domains,
		ipRanges:    ipRanges,
		parents:     parents,
		configProxy: configProxy,
		audit:       audit,
	}
}

type ConfigDetail struct {
	domain.Config
	Domains       []domain.DomainRule  `json:"domains"`
	IPRanges      []domain.IPRangeRule `json:"ip_ranges"`
	ParentProxies []domain.ParentProxy `json:"parent_proxies"`
	Proxies       []domain.Proxy       `json:"proxies"`
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
	if proxies == nil {
		proxies = []domain.Proxy{}
	}

	return &ConfigDetail{
		Config:        *cfg,
		Domains:       domains,
		IPRanges:      ipRanges,
		ParentProxies: parents,
		Proxies:       proxies,
	}, nil
}

func (s *ConfigService) List(ctx context.Context, status *domain.ConfigStatus, page, limit int) ([]domain.Config, int, error) {
	offset := (page - 1) * limit
	return s.configs.List(ctx, status, limit, offset)
}

type CreateConfigRequest struct {
	Name          string                    `json:"name"`
	Description   *string                   `json:"description,omitempty"`
	Domains       []DomainRuleInput         `json:"domains"`
	IPRanges      []IPRangeRuleInput        `json:"ip_ranges"`
	ParentProxies []ParentProxyInput        `json:"parent_proxies"`
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

func (s *ConfigService) Create(ctx context.Context, req CreateConfigRequest, userID uuid.UUID, ip, ua string) (*ConfigDetail, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("%w: name is required", domain.ErrBadRequest)
	}

	var result *ConfigDetail

	err := repository.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		txConfigs := repository.NewConfigRepo(tx)
		txDomains := repository.NewDomainRuleRepo(tx)
		txIPRanges := repository.NewIPRangeRuleRepo(tx)
		txParents := repository.NewParentProxyRepo(tx)
		txConfigProxy := repository.NewConfigProxyRepo(tx)
		txAudit := repository.NewAuditRepo(tx)

		cfg := &domain.Config{
			Name:        req.Name,
			Description: req.Description,
			CreatedBy:   &userID,
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
	var result *ConfigDetail

	err := repository.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		txConfigs := repository.NewConfigRepo(tx)
		txDomains := repository.NewDomainRuleRepo(tx)
		txIPRanges := repository.NewIPRangeRuleRepo(tx)
		txParents := repository.NewParentProxyRepo(tx)
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
		txConfigProxy := repository.NewConfigProxyRepo(tx)
		txAudit := repository.NewAuditRepo(tx)

		// Load original config
		original, err := txConfigs.GetByID(ctx, id)
		if err != nil {
			return err
		}

		// Create new config with incremented version
		newCfg := &domain.Config{
			Name:        original.Name,
			Description: original.Description,
			CreatedBy:   &userID,
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

// GenerateConfigHash computes SHA256 of the generated parent.config + sni.yaml content.
func (s *ConfigService) GenerateConfigHash(ctx context.Context, configID uuid.UUID) (string, error) {
	parentConfig, sniYaml, _, err := s.GenerateConfigFiles(ctx, configID)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write([]byte(parentConfig))
	h.Write([]byte(sniYaml))
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

	parentConfig = generateParentConfig(ipRanges, domains, parents)
	sniYaml = generateSNIYaml(domains)
	ipAllowYaml = ""

	return parentConfig, sniYaml, ipAllowYaml, nil
}

func generateParentConfig(ipRanges []domain.IPRangeRule, domainRules []domain.DomainRule, parentProxies []domain.ParentProxy) string {
	var b strings.Builder

	// Sort by priority
	sort.Slice(ipRanges, func(i, j int) bool { return ipRanges[i].Priority < ipRanges[j].Priority })
	sort.Slice(domainRules, func(i, j int) bool { return domainRules[i].Priority < domainRules[j].Priority })
	sort.Slice(parentProxies, func(i, j int) bool { return parentProxies[i].Priority < parentProxies[j].Priority })

	// IP range rules → dest_ip lines
	for _, ir := range ipRanges {
		if ir.Action == domain.ActionDirect {
			ipRange := cidrToRange(ir.CIDR)
			b.WriteString(fmt.Sprintf("dest_ip=%s go_direct=true\n", ipRange))
		}
	}

	// Domain rules → dest_domain lines
	for _, dr := range domainRules {
		if dr.Action == domain.ActionDirect {
			b.WriteString(fmt.Sprintf("dest_domain=%s go_direct=true\n", dr.Domain))
		}
	}

	// Default rule: everything else goes through parent proxies
	if len(parentProxies) > 0 {
		var enabled []domain.ParentProxy
		for _, pp := range parentProxies {
			if pp.Enabled {
				enabled = append(enabled, pp)
			}
		}
		if len(enabled) > 0 {
			var parentList []string
			for _, pp := range enabled {
				parentList = append(parentList, fmt.Sprintf("%s:%d", pp.Address, pp.Port))
			}
			b.WriteString(fmt.Sprintf("dest_domain=. parent=\"%s\" round_robin=strict\n",
				strings.Join(parentList, ";")))
		}
	}

	return b.String()
}

func generateSNIYaml(domainRules []domain.DomainRule) string {
	var b strings.Builder
	b.WriteString("sni:\n")

	sort.Slice(domainRules, func(i, j int) bool { return domainRules[i].Priority < domainRules[j].Priority })

	for _, dr := range domainRules {
		if dr.Action == domain.ActionDirect {
			fqdn := dr.Domain
			if strings.HasPrefix(fqdn, ".") {
				fqdn = "*" + fqdn
			}
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

func jsonVal(key, value string) []byte {
	data, _ := json.Marshal(map[string]string{key: value})
	return data
}
