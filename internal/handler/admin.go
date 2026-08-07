package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"kldns/internal/repository"
	"kldns/internal/runtime"
	services "kldns/internal/service"
	"kldns/pkg/dns"
	"kldns/pkg/utils/auth"
	apperrors "kldns/pkg/utils/errors"
	"kldns/pkg/utils/secrets"
	"kldns/pkg/utils/validation"
)

type AdminListController struct {
	APIController
}

func (c *AdminListController) Users() {
	filter := repository.UserAdminFilter{
		Keyword: strings.TrimSpace(c.GetString("keyword")),
		GroupID: queryInt64(c, "group_id"),
	}
	if raw := strings.TrimSpace(c.GetString("status")); raw != "" {
		if status, err := strconv.Atoi(raw); err == nil {
			filter.Status = &status
		}
	}
	repo := repository.NewAdminRepository(runtime.DB())
	if page, ok := pageQuery(c); ok {
		items, err := repo.ListUsersPage(c.Ctx.Request.Context(), filter, page)
		c.respondList(items, err, "获取用户失败")
		return
	}
	items, err := repo.ListUsers(c.Ctx.Request.Context(), filter)
	c.respondList(items, err, "获取用户失败")
}

func (c *AdminListController) SaveUser() {
	admin, ok := c.CurrentUser()
	if !ok {
		return
	}
	id := c.PathInt64(":id")
	var input repository.UserWrite
	if !c.BindJSON(&input) {
		return
	}
	input.ID = id
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if input.ID <= 0 {
		c.Fail(http.StatusBadRequest, apperrors.CodeInvalidArgument, "用户 ID 不正确")
		return
	}
	if len(input.Username) < 4 {
		c.Fail(http.StatusBadRequest, apperrors.CodeInvalidArgument, "用户名太短")
		return
	}
	if input.Email != "" && !validation.IsValidEmail(input.Email) {
		c.Fail(http.StatusBadRequest, apperrors.CodeInvalidArgument, "邮箱格式不正确")
		return
	}
	if input.GroupID <= 0 {
		c.Fail(http.StatusBadRequest, apperrors.CodeInvalidArgument, "请选择用户组")
		return
	}
	if input.Status < 0 || input.Status > 2 {
		c.Fail(http.StatusBadRequest, apperrors.CodeInvalidArgument, "用户状态不正确")
		return
	}
	if input.ID == 1 && (input.GroupID != 99 || input.Status != 2) {
		c.Fail(http.StatusForbidden, apperrors.CodeForbidden, "初始管理员账号不能修改用户组或状态")
		return
	}
	if input.ID == admin.ID && (input.GroupID != 99 || input.Status == 0) {
		c.Fail(http.StatusForbidden, apperrors.CodeForbidden, "不能取消自己的后台权限或禁用自己")
		return
	}
	var raw struct {
		Password string `json:"password"`
	}
	_ = json.Unmarshal(c.RawBody(), &raw)
	if strings.TrimSpace(raw.Password) != "" {
		if len(raw.Password) < 8 {
			c.Fail(http.StatusBadRequest, apperrors.CodeInvalidArgument, "密码至少 8 位")
			return
		}
		hash, err := auth.HashPassword(raw.Password)
		if err != nil {
			c.Internal("生成密码哈希失败")
			return
		}
		input.PasswordHash = hash
		input.SID = randomSID()
	}
	updated, err := repository.NewAdminRepository(runtime.DB()).UpdateUser(c.Ctx.Request.Context(), input)
	if err != nil {
		c.Fail(http.StatusConflict, apperrors.CodeConflict, "账号或邮箱已被使用")
		return
	}
	if !updated {
		c.Fail(http.StatusNotFound, apperrors.CodeNotFound, "用户不存在")
		return
	}
	c.OK(map[string]any{"updated": true})
}

func (c *AdminListController) Groups() {
	items, err := repository.NewAdminRepository(runtime.DB()).ListGroups(c.Ctx.Request.Context(), repository.GroupFilter{
		Keyword: strings.TrimSpace(c.GetString("keyword")),
	})
	c.respondList(items, err, "获取用户组失败")
}

func (c *AdminListController) SaveGroup() {
	var input struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	if !c.BindJSON(&input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		c.Fail(http.StatusBadRequest, apperrors.CodeInvalidArgument, "请输入用户组名称")
		return
	}
	id, err := repository.NewAdminRepository(runtime.DB()).UpsertGroup(c.Ctx.Request.Context(), input.ID, input.Name)
	if err != nil {
		c.Internal("保存用户组失败")
		return
	}
	c.OK(map[string]any{"id": id})
}

func (c *AdminListController) DeleteGroup() {
	id := c.PathInt64(":id")
	deleted, err := repository.NewAdminRepository(runtime.DB()).DeleteGroup(c.Ctx.Request.Context(), id)
	if err != nil {
		c.Internal("删除用户组失败")
		return
	}
	if !deleted {
		c.Fail(http.StatusNotFound, apperrors.CodeNotFound, "用户组不存在或不能删除")
		return
	}
	c.OK(map[string]any{"deleted": true})
}

func (c *AdminListController) Domains() {
	filter := repository.DomainAdminFilter{
		ProviderKey: strings.TrimSpace(c.GetString("provider")),
		Keyword:     strings.TrimSpace(c.GetString("keyword")),
	}
	repo := repository.NewAdminRepository(runtime.DB())
	if page, ok := pageQuery(c); ok {
		items, err := repo.ListDomainsPage(c.Ctx.Request.Context(), filter, page)
		c.respondList(items, err, "获取主域失败")
		return
	}
	items, err := repo.ListDomains(c.Ctx.Request.Context(), filter)
	c.respondList(items, err, "获取主域失败")
}

func (c *AdminListController) SaveDomain() {
	var input repository.DomainWrite
	if !c.BindJSON(&input) {
		return
	}
	if id := c.PathInt64(":id"); id > 0 {
		input.ID = id
	}
	input.RemoteZoneID = strings.TrimSpace(input.RemoteZoneID)
	input.Domain = strings.ToLower(strings.TrimSpace(input.Domain))
	if input.ProviderConfigID <= 0 || input.RemoteZoneID == "" || input.Domain == "" {
		c.Fail(http.StatusBadRequest, apperrors.CodeInvalidArgument, "请完整填写主域信息")
		return
	}
	repo := repository.NewAdminRepository(runtime.DB())
	configRepo := repo.ProviderConfigs()
	providerConfig, err := configRepo.Get(c.Ctx.Request.Context(), input.ProviderConfigID)
	if err != nil {
		c.Fail(http.StatusBadRequest, apperrors.CodeInvalidArgument, "请选择有效的平台配置")
		return
	}
	if strings.TrimSpace(providerConfig.ConfigCiphertext) == "" {
		c.Fail(http.StatusBadRequest, apperrors.CodeInvalidArgument, "所选平台配置尚未填写密钥，请先完善配置")
		return
	}
	input.ProviderKey = providerConfig.ProviderKey
	if _, ok := dns.New(input.ProviderKey); !ok {
		c.Fail(http.StatusBadRequest, apperrors.CodeInvalidArgument, "暂不支持此 DNS 平台")
		return
	}
	if input.ID > 0 {
		if _, err := repository.NewRecordRepository(runtime.DB()).GetDomain(c.Ctx.Request.Context(), input.ID); err != nil {
			c.Fail(http.StatusNotFound, apperrors.CodeNotFound, "主域不存在")
			return
		}
	}
	if conflict, found, err := repo.FindDomainConflict(c.Ctx.Request.Context(), input); err != nil {
		c.Internal("检查主域重复失败")
		return
	} else if found {
		if conflict.Domain == input.Domain {
			c.Fail(http.StatusConflict, apperrors.CodeConflict, "主域已存在，请编辑已有主域或选择其他域名")
			return
		}
		c.Fail(http.StatusConflict, apperrors.CodeConflict, "该 DNS 平台域名已添加，请编辑已有主域")
		return
	}
	id, err := repo.UpsertDomain(c.Ctx.Request.Context(), input)
	if err != nil {
		c.Fail(http.StatusConflict, apperrors.CodeDatabaseConflict, "保存主域失败")
		return
	}
	c.OK(map[string]any{"id": id})
}

func (c *AdminListController) DeleteDomain() {
	admin, ok := c.CurrentUser()
	if !ok {
		return
	}
	id := c.PathInt64(":id")
	var input struct {
		DeleteMode string `json:"delete_mode"`
	}
	if len(c.RawBody()) > 0 {
		if !c.BindJSON(&input) {
			return
		}
	}
	svc := services.AdminDeletionService{
		Repo:     repository.NewRecordRepository(runtime.DB()),
		Resolver: providerResolver(),
	}
	result, appErr := svc.DeleteDomain(c.Ctx.Request.Context(), admin.ID, id, input.DeleteMode, "admin")
	if appErr != nil {
		c.FailApp(appErr)
		return
	}
	c.OK(result)
}

func (c *AdminListController) SyncDomainRecords() {
	admin, ok := c.CurrentUser()
	if !ok {
		return
	}
	id := c.PathInt64(":id")
	svc := services.AdminDomainSyncService{
		Repo:     repository.NewRecordRepository(runtime.DB()),
		Resolver: providerResolver(),
	}
	result, appErr := svc.SyncRecords(c.Ctx.Request.Context(), admin.ID, id)
	if appErr != nil {
		c.FailApp(appErr)
		return
	}
	c.OK(result)
}

func hasProviderConfigValue(config map[string]string) bool {
	for _, value := range config {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func normalizeProviderConfig(fields []dns.ConfigField, config map[string]string) map[string]string {
	out := make(map[string]string, len(fields))
	for _, field := range fields {
		out[field.Name] = strings.TrimSpace(config[field.Name])
	}
	return out
}

func (c *AdminListController) Providers() {
	repo := repository.NewAdminRepository(runtime.DB())
	stored, err := repo.ProviderConfigs().StoredProviders(c.Ctx.Request.Context())
	if err != nil {
		c.Internal("获取 DNS 平台失败")
		return
	}
	var items []repository.ProviderSummary
	for _, key := range dns.RegisteredKeys() {
		provider, ok := dns.New(key)
		if !ok {
			continue
		}
		items = append(items, repository.ProviderSummary{
			Key: key, Label: provider.Label(), Fields: provider.ConfigFields(), Stored: stored[key],
		})
	}
	c.OK(items)
}

func (c *AdminListController) ProviderConfigs() {
	filter := repository.ProviderConfigFilter{
		ProviderKey: strings.TrimSpace(c.GetString("provider")),
		Keyword:     strings.TrimSpace(c.GetString("keyword")),
	}
	repo := repository.NewAdminRepository(runtime.DB()).ProviderConfigs()
	if page, ok := pageQuery(c); ok {
		items, err := repo.ListPage(c.Ctx.Request.Context(), filter, page)
		c.respondList(items, err, "获取平台配置失败")
		return
	}
	items, err := repo.List(c.Ctx.Request.Context(), filter)
	c.respondList(items, err, "获取平台配置失败")
}

func (c *AdminListController) SaveProviderConfig() {
	var input repository.ProviderConfigWrite
	if !c.BindJSON(&input) {
		return
	}
	if id := c.PathInt64(":id"); id > 0 {
		input.ID = id
	}
	input.ProviderKey = strings.TrimSpace(input.ProviderKey)
	input.Name = strings.TrimSpace(input.Name)
	if input.ProviderKey == "" || input.Name == "" {
		c.Fail(http.StatusBadRequest, apperrors.CodeInvalidArgument, "请填写平台配置名称")
		return
	}
	provider, ok := dns.New(input.ProviderKey)
	if !ok {
		c.Fail(http.StatusBadRequest, apperrors.CodeInvalidArgument, "暂不支持此 DNS 平台")
		return
	}
	repo := repository.NewAdminRepository(runtime.DB()).ProviderConfigs()
	var existing repository.ProviderConfigRecord
	if input.ID > 0 {
		var err error
		existing, err = repo.Get(c.Ctx.Request.Context(), input.ID)
		if err != nil {
			c.Fail(http.StatusNotFound, apperrors.CodeNotFound, "平台配置不存在")
			return
		}
		// Platform type is immutable so linked domains keep a stable provider_key.
		input.ProviderKey = existing.ProviderKey
	}
	if conflict, found, err := repo.FindByName(c.Ctx.Request.Context(), input.ProviderKey, input.Name, input.ID); err != nil {
		c.Internal("检查平台配置名称失败")
		return
	} else if found {
		_ = conflict
		c.Fail(http.StatusConflict, apperrors.CodeConflict, "同一平台下配置名称已存在")
		return
	}
	ciphertext, keepExisting, appErr := c.prepareProviderConfigCiphertext(input, existing, provider)
	if appErr != nil {
		c.FailApp(appErr)
		return
	}
	input.ConfigCiphertext = ciphertext
	input.KeepExistingCiphertext = keepExisting
	id, err := repo.Upsert(c.Ctx.Request.Context(), input)
	if err != nil {
		c.Fail(http.StatusConflict, apperrors.CodeDatabaseConflict, "保存平台配置失败")
		return
	}
	c.OK(map[string]any{"id": id})
}

func (c *AdminListController) DeleteProviderConfig() {
	id := c.PathInt64(":id")
	repo := repository.NewAdminRepository(runtime.DB()).ProviderConfigs()
	deleted, err := repo.Delete(c.Ctx.Request.Context(), id)
	if err == repository.ErrProviderConfigInUse {
		c.Fail(http.StatusConflict, apperrors.CodeConflict, "仍有主域使用此配置，请先更换或删除相关主域")
		return
	}
	if err != nil {
		c.Internal("删除平台配置失败")
		return
	}
	if !deleted {
		c.Fail(http.StatusNotFound, apperrors.CodeNotFound, "平台配置不存在")
		return
	}
	c.OK(map[string]any{"deleted": true})
}

func (c *AdminListController) prepareProviderConfigCiphertext(input repository.ProviderConfigWrite, existing repository.ProviderConfigRecord, provider dns.Provider) (string, bool, *apperrors.AppError) {
	fields := provider.ConfigFields()
	config := normalizeProviderConfig(fields, input.Config)

	// Editing: blank fields keep previously stored values so users can leave secrets empty
	// or only update optional settings (e.g. ProxyURL).
	if input.ID > 0 && existing.ProviderKey == input.ProviderKey && strings.TrimSpace(existing.ConfigCiphertext) != "" {
		stored, err := decryptProviderConfigMap(existing.ConfigCiphertext)
		if err != nil {
			return "", false, err
		}
		merged := mergeProviderConfig(fields, stored, config)
		if !providerConfigChanged(fields, stored, merged) {
			return "", true, nil
		}
		config = merged
	}

	for _, field := range fields {
		if field.Required && strings.TrimSpace(config[field.Name]) == "" {
			return "", false, apperrors.New(apperrors.CodeInvalidArgument, "请完整填写 DNS 配置信息")
		}
	}
	if err := provider.Configure(config); err != nil {
		return "", false, apperrors.New(apperrors.CodeInvalidArgument, "DNS 配置格式不正确")
	}
	if err := provider.Check(c.Ctx.Request.Context()); err != nil {
		return "", false, apperrors.New(apperrors.CodeInvalidArgument, "请检查 DNS 配置是否正确")
	}
	data, err := json.Marshal(config)
	if err != nil {
		return "", false, apperrors.New(apperrors.CodeInternal, "序列化 DNS 配置失败")
	}
	ciphertext, err := secrets.Encrypt(appSecret(), string(data))
	if err != nil {
		return "", false, apperrors.New(apperrors.CodeInternal, "加密 DNS 配置失败")
	}
	return ciphertext, false, nil
}

func decryptProviderConfigMap(ciphertext string) (map[string]string, *apperrors.AppError) {
	raw, err := secrets.Decrypt(appSecret(), ciphertext)
	if err != nil {
		return nil, apperrors.New(apperrors.CodeInternal, "解密 DNS 配置失败")
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, apperrors.New(apperrors.CodeInternal, "解析 DNS 配置失败")
	}
	if decoded == nil {
		decoded = map[string]string{}
	}
	return decoded, nil
}

func mergeProviderConfig(fields []dns.ConfigField, stored map[string]string, input map[string]string) map[string]string {
	out := make(map[string]string, len(fields))
	for _, field := range fields {
		next := strings.TrimSpace(input[field.Name])
		if next == "" {
			next = strings.TrimSpace(stored[field.Name])
		}
		out[field.Name] = next
	}
	return out
}

func providerConfigChanged(fields []dns.ConfigField, stored map[string]string, next map[string]string) bool {
	for _, field := range fields {
		if strings.TrimSpace(stored[field.Name]) != strings.TrimSpace(next[field.Name]) {
			return true
		}
	}
	return false
}

func (c *AdminListController) ProviderZones() {
	var input struct {
		Key              string            `json:"key"`
		Config           map[string]string `json:"config"`
		ProviderConfigID int64             `json:"provider_config_id"`
	}
	if !c.BindJSON(&input) {
		return
	}
	input.Key = strings.TrimSpace(input.Key)
	provider, ok := dns.New(input.Key)
	if !ok {
		c.Fail(http.StatusBadRequest, apperrors.CodeInvalidArgument, "暂不支持此 DNS 平台")
		return
	}
	config, appErr := c.providerConfigForZoneList(input.Key, input.Config, input.ProviderConfigID, provider)
	if appErr != nil {
		c.FailApp(appErr)
		return
	}
	if err := provider.Configure(config); err != nil {
		c.Fail(http.StatusBadRequest, apperrors.CodeInvalidArgument, "DNS 配置格式不正确")
		return
	}
	zones, err := provider.ListZones(c.Ctx.Request.Context())
	if err != nil {
		c.Fail(http.StatusBadRequest, apperrors.CodeDNSProviderFailed, dnsProviderFailureMessage("获取平台域名失败，请检查 DNS 配置", err))
		return
	}
	c.OK(zones)
}

func dnsProviderFailureMessage(message string, err error) string {
	detail := strings.TrimSpace(err.Error())
	if detail == "" {
		return message
	}
	if len(detail) > 240 {
		detail = detail[:240] + "..."
	}
	return message + "：" + detail
}

func (c *AdminListController) providerConfigForZoneList(key string, config map[string]string, providerConfigID int64, provider dns.Provider) (map[string]string, *apperrors.AppError) {
	if hasProviderConfigValue(config) {
		normalized := normalizeProviderConfig(provider.ConfigFields(), config)
		for _, field := range provider.ConfigFields() {
			if field.Required && strings.TrimSpace(normalized[field.Name]) == "" {
				return nil, apperrors.New(apperrors.CodeInvalidArgument, "请完整填写 DNS 配置信息")
			}
		}
		return normalized, nil
	}
	if providerConfigID <= 0 {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "请先选择或填写 DNS 配置信息")
	}
	existing, err := repository.NewAdminRepository(runtime.DB()).ProviderConfigs().Get(c.Ctx.Request.Context(), providerConfigID)
	if err != nil {
		return nil, apperrors.New(apperrors.CodeNotFound, "平台配置不存在")
	}
	if existing.ProviderKey != key || strings.TrimSpace(existing.ConfigCiphertext) == "" {
		return nil, apperrors.New(apperrors.CodeInvalidArgument, "请先完善 DNS 配置信息")
	}
	raw, err := secrets.Decrypt(appSecret(), existing.ConfigCiphertext)
	if err != nil {
		return nil, apperrors.New(apperrors.CodeInternal, "解密 DNS 配置失败")
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, apperrors.New(apperrors.CodeInternal, "解析 DNS 配置失败")
	}
	return normalizeProviderConfig(provider.ConfigFields(), decoded), nil
}

func (c *AdminListController) Records() {
	filter := repository.RecordAdminFilter{
		DID:         queryInt64(c, "did"),
		SubdomainID: queryInt64(c, "subdomain_id"),
		UID:         queryInt64(c, "uid"),
		Type:        strings.TrimSpace(c.GetString("type")),
		Keyword:     strings.TrimSpace(c.GetString("keyword")),
	}
	repo := repository.NewAdminRepository(runtime.DB())
	if page, ok := pageQuery(c); ok {
		items, err := repo.ListAllRecordsPage(c.Ctx.Request.Context(), filter, page)
		c.respondList(items, err, "获取记录失败")
		return
	}
	items, err := repo.ListAllRecords(c.Ctx.Request.Context(), filter)
	c.respondList(items, err, "获取记录失败")
}

func (c *AdminListController) Subdomains() {
	filter := repository.SubdomainAdminFilter{
		DID:     queryInt64(c, "did"),
		Keyword: strings.TrimSpace(c.GetString("keyword")),
	}
	if raw := strings.TrimSpace(c.GetString("status")); raw != "" {
		if status, err := strconv.Atoi(raw); err == nil {
			filter.Status = &status
		}
	}
	repo := repository.NewAdminRepository(runtime.DB())
	if page, ok := pageQuery(c); ok {
		items, err := repo.ListAllSubdomainsPage(c.Ctx.Request.Context(), filter, page)
		c.respondList(items, err, "获取二级域名失败")
		return
	}
	items, err := repo.ListAllSubdomains(c.Ctx.Request.Context(), filter)
	c.respondList(items, err, "获取二级域名失败")
}

func (c *AdminListController) ApproveSubdomain() {
	admin, ok := c.CurrentUser()
	if !ok {
		return
	}
	svc := services.AdminSubdomainReviewService{
		Repo: repository.NewRecordRepository(runtime.DB()),
	}
	result, appErr := svc.Approve(c.Ctx.Request.Context(), admin.ID, c.PathInt64(":id"), "admin")
	if appErr != nil {
		c.FailApp(appErr)
		return
	}
	c.OK(result)
}

func (c *AdminListController) RejectSubdomain() {
	admin, ok := c.CurrentUser()
	if !ok {
		return
	}
	var input struct {
		Reason string `json:"reason"`
	}
	if !c.BindJSON(&input) {
		return
	}
	svc := services.AdminSubdomainReviewService{
		Repo: repository.NewRecordRepository(runtime.DB()),
	}
	result, appErr := svc.Reject(c.Ctx.Request.Context(), admin.ID, c.PathInt64(":id"), input.Reason, "admin")
	if appErr != nil {
		c.FailApp(appErr)
		return
	}
	c.OK(result)
}

func (c *AdminListController) DeleteSubdomain() {
	admin, ok := c.CurrentUser()
	if !ok {
		return
	}
	id := c.PathInt64(":id")
	svc := services.AdminDeletionService{
		Repo:     repository.NewRecordRepository(runtime.DB()),
		Resolver: providerResolver(),
	}
	result, appErr := svc.DeleteSubdomain(c.Ctx.Request.Context(), admin.ID, id, "admin")
	if appErr != nil {
		c.FailApp(appErr)
		return
	}
	c.OK(result)
}

func (c *AdminListController) DeleteUser() {
	admin, ok := c.CurrentUser()
	if !ok {
		return
	}
	id := c.PathInt64(":id")
	var input struct {
		ConfirmUsername string `json:"confirm_username"`
	}
	if !c.BindJSON(&input) {
		return
	}
	svc := services.AdminDeletionService{
		Repo:     repository.NewRecordRepository(runtime.DB()),
		Resolver: providerResolver(),
	}
	result, appErr := svc.DeleteUser(c.Ctx.Request.Context(), admin.ID, id, input.ConfirmUsername, "admin")
	if appErr != nil {
		c.FailApp(appErr)
		return
	}
	c.OK(result)
}

func (c *AdminListController) Points() {
	filter := repository.PointRecordAdminFilter{
		UID:     queryInt64(c, "uid"),
		AdminID: queryInt64(c, "admin_uid"),
		Action:  strings.TrimSpace(c.GetString("action")),
		Change:  strings.TrimSpace(c.GetString("change")),
		Keyword: strings.TrimSpace(c.GetString("keyword")),
	}
	repo := repository.NewPointsRepository(runtime.DB())
	if page, ok := pageQuery(c); ok {
		items, err := repo.ListAdminPointRecordsPage(c.Ctx.Request.Context(), filter, page)
		c.respondList(items, err, "获取积分明细失败")
		return
	}
	items, err := repo.ListAdminPointRecords(c.Ctx.Request.Context(), filter)
	c.respondList(items, err, "获取积分明细失败")
}

func (c *AdminListController) AdjustUserPoints() {
	admin, ok := c.CurrentUser()
	if !ok {
		return
	}
	var input struct {
		Mode   string `json:"mode"`
		Points int64  `json:"points"`
		Remark string `json:"remark"`
	}
	if !c.BindJSON(&input) {
		return
	}
	svc := services.AdminPointsService{
		Repo: repository.NewPointsRepository(runtime.DB()),
	}
	result, appErr := svc.AdjustUserPoints(c.Ctx.Request.Context(), services.AdjustUserPointsInput{
		AdminID: admin.ID, UserID: c.PathInt64(":id"), Mode: input.Mode, Points: input.Points,
		Remark: input.Remark, Source: "admin",
	})
	if appErr != nil {
		c.FailApp(appErr)
		return
	}
	c.OK(result)
}

func (c *AdminListController) SaveRecord() {
	admin, ok := c.CurrentUser()
	if !ok {
		return
	}
	var input struct {
		UID    int64  `json:"uid"`
		DID    int64  `json:"did"`
		Name   string `json:"name"`
		Type   string `json:"type"`
		Value  string `json:"value"`
		LineID string `json:"line_id"`
	}
	if !c.BindJSON(&input) {
		return
	}
	id := c.PathInt64(":id")
	svc := services.AdminRecordService{
		Repo:     repository.NewRecordRepository(runtime.DB()),
		Resolver: providerResolver(),
		Reserved: settingsReserveNames(c.Ctx.Request.Context()),
	}
	var result services.SubmitRecordResult
	var appErr *apperrors.AppError
	if id > 0 {
		result, appErr = svc.Update(c.Ctx.Request.Context(), services.AdminRecordInput{
			AdminID: admin.ID, ID: id, Name: input.Name, Type: input.Type, Value: input.Value, LineID: input.LineID, Source: "admin",
		})
	} else {
		result, appErr = svc.Create(c.Ctx.Request.Context(), services.AdminRecordInput{
			AdminID: admin.ID, UID: input.UID, DID: input.DID, Name: input.Name, Type: input.Type, Value: input.Value, LineID: input.LineID, Source: "admin",
		})
	}
	if appErr != nil {
		c.FailApp(appErr)
		return
	}
	c.OK(result)
}

func (c *AdminListController) DeleteRecord() {
	admin, ok := c.CurrentUser()
	if !ok {
		return
	}
	id := c.PathInt64(":id")
	svc := services.AdminRecordService{
		Repo:     repository.NewRecordRepository(runtime.DB()),
		Resolver: providerResolver(),
		Reserved: settingsReserveNames(c.Ctx.Request.Context()),
	}
	result, appErr := svc.Delete(c.Ctx.Request.Context(), admin.ID, id, "admin")
	if appErr != nil {
		c.FailApp(appErr)
		return
	}
	c.OK(result)
}

func (c *AdminListController) Logs() {
	filter := repository.LogFilter{
		Source:  strings.TrimSpace(c.GetString("source")),
		Action:  strings.TrimSpace(c.GetString("action")),
		Keyword: strings.TrimSpace(c.GetString("keyword")),
	}
	repo := repository.NewAdminRepository(runtime.DB())
	if page, ok := pageQuery(c); ok {
		items, err := repo.ListLogsPage(c.Ctx.Request.Context(), filter, page)
		c.respondList(items, err, "获取日志失败")
		return
	}
	items, err := repo.ListLogs(c.Ctx.Request.Context(), filter)
	c.respondList(items, err, "获取日志失败")
}

func (c *AdminListController) Settings() {
	items, err := repository.NewAdminRepository(runtime.DB()).ListSettings(c.Ctx.Request.Context(), appSecret())
	c.respondList(items, err, "获取设置失败")
}

func (c *AdminListController) SaveSettings() {
	var input map[string]string
	if !c.BindJSON(&input) {
		return
	}
	if err := repository.NewAdminRepository(runtime.DB()).UpsertSettings(c.Ctx.Request.Context(), input, appSecret()); err != nil {
		c.Internal("保存设置失败")
		return
	}
	c.OK(map[string]any{"saved": true})
}

func (c *AdminListController) respondList(items any, err error, message string) {
	if err != nil {
		c.Internal(message)
		return
	}
	c.OK(items)
}

func appSecret() string {
	return runtime.SecretKey()
}

func queryInt64(c interface {
	GetString(string, ...string) string
}, key string) int64 {
	value, _ := strconv.ParseInt(strings.TrimSpace(c.GetString(key)), 10, 64)
	return value
}

func pageQuery(c interface {
	GetString(string, ...string) string
}) (repository.PageQuery, bool) {
	rawPage := strings.TrimSpace(c.GetString("page"))
	rawPageSize := strings.TrimSpace(c.GetString("page_size"))
	if rawPage == "" && rawPageSize == "" {
		return repository.PageQuery{}, false
	}
	page, _ := strconv.Atoi(rawPage)
	pageSize, _ := strconv.Atoi(rawPageSize)
	return repository.PageQuery{Page: page, PageSize: pageSize}.Normalize(), true
}
