package services

import (
	"context"
	"strings"

	"kldns/models"
	"kldns/pkg/dns"
	apperrors "kldns/pkg/errors"
)

type enqueueDNSWriteJobFunc func(context.Context, models.DNSWriteJob) error

func createRemoteRecordThenLocal(
	ctx context.Context,
	resolver ProviderResolver,
	enqueue enqueueDNSWriteJobFunc,
	domain models.Domain,
	record models.Record,
	source string,
	applyLocal func(models.Record) error,
) (SubmitRecordResult, *apperrors.AppError) {
	provider, err := resolver.Resolve(ctx, domain)
	if err != nil {
		return SubmitRecordResult{}, dnsProviderError("域名配置错误", err)
	}
	zone := dns.Zone{ID: domain.RemoteZoneID, Domain: domain.Domain}
	remoteRecord, err := provider.CreateRecord(ctx, zone, dns.RecordInput{
		Name: record.Name, Type: record.Type, Value: record.Value, LineID: record.LineID,
	})
	if err != nil {
		return SubmitRecordResult{}, dnsProviderError("添加记录失败", err)
	}
	record.RecordID = remoteRecord.RemoteID
	if remoteRecord.Line != "" {
		record.Line = remoteRecord.Line
	}
	if err := applyLocal(record); err == nil {
		return SubmitRecordResult{Mode: "direct"}, nil
	} else {
		if deleteErr := deleteRemoteRecordValue(ctx, provider, zone, remoteRecord.RemoteID, record); deleteErr != nil {
			enqueueRecordRepair(ctx, enqueue, models.DNSWriteJob{
				UID: record.UID, Source: source, ProviderKey: domain.ProviderKey, Domain: domain.Domain,
				RecordName: record.Name, RecordType: record.Type, ValueDigest: digest(record.Value),
				RemoteRecordID: remoteRecord.RemoteID, Operation: "compensate_delete", Status: "pending",
				LastError: deleteErr.Error(), Payload: mustJSON(map[string]any{"record": record, "delete_error": deleteErr.Error()}),
			})
		}
		return SubmitRecordResult{}, apperrors.Wrap(apperrors.CodeInternal, "本地保存失败，已触发远端补偿流程", err)
	}
}

func updateRemoteRecordThenLocal(
	ctx context.Context,
	resolver ProviderResolver,
	enqueue enqueueDNSWriteJobFunc,
	domain models.Domain,
	oldRecord models.Record,
	next models.Record,
	source string,
	applyLocal func(models.Record) error,
) (SubmitRecordResult, *apperrors.AppError) {
	provider, err := resolver.Resolve(ctx, domain)
	if err != nil {
		return SubmitRecordResult{}, dnsProviderError("域名配置错误", err)
	}
	zone := dns.Zone{ID: domain.RemoteZoneID, Domain: domain.Domain}
	if sameRemoteRecord(oldRecord, next) {
		next.RecordID = oldRecord.RecordID
		if err := applyLocal(next); err == nil {
			return SubmitRecordResult{Mode: "direct"}, nil
		} else {
			return SubmitRecordResult{}, apperrors.Wrap(apperrors.CodeInternal, "本地保存失败", err)
		}
	}
	remoteRecord, err := updateRemoteRecordValue(ctx, provider, zone, oldRecord, next)
	if err != nil {
		if current, getErr := provider.GetRecord(ctx, zone, oldRecord.RecordID); getErr == nil && remoteMatchesRecord(current, next) {
			next.RecordID = oldRecord.RecordID
			if current.RemoteID != "" {
				next.RecordID = current.RemoteID
			}
			if current.Line != "" {
				next.Line = current.Line
			}
			if err := applyLocal(next); err == nil {
				return SubmitRecordResult{Mode: "direct"}, nil
			} else {
				return SubmitRecordResult{}, apperrors.Wrap(apperrors.CodeInternal, "本地保存失败", err)
			}
		}
		return SubmitRecordResult{}, dnsProviderError("更新记录失败", err)
	}
	next.RecordID = oldRecord.RecordID
	if remoteRecord.RemoteID != "" {
		next.RecordID = remoteRecord.RemoteID
	}
	if remoteRecord.Line != "" {
		next.Line = remoteRecord.Line
	}
	if err := applyLocal(next); err == nil {
		return SubmitRecordResult{Mode: "direct"}, nil
	} else {
		if _, restoreErr := updateRemoteRecordValue(ctx, provider, zone, next, oldRecord); restoreErr != nil {
			enqueueRecordRepair(ctx, enqueue, models.DNSWriteJob{
				UID: oldRecord.UID, Source: source, ProviderKey: domain.ProviderKey, Domain: domain.Domain,
				RecordName: oldRecord.Name, RecordType: oldRecord.Type, ValueDigest: digest(oldRecord.Value),
				RemoteRecordID: next.RecordID, Operation: "restore_update", Status: "pending",
				LastError: restoreErr.Error(), Payload: mustJSON(map[string]any{"old": oldRecord, "next": next}),
			})
		}
		return SubmitRecordResult{}, apperrors.Wrap(apperrors.CodeInternal, "本地保存失败，已触发远端恢复流程", err)
	}
}

func sameRemoteRecord(oldRecord models.Record, next models.Record) bool {
	return strings.EqualFold(strings.TrimSpace(oldRecord.Name), strings.TrimSpace(next.Name)) &&
		strings.EqualFold(strings.TrimSpace(oldRecord.Type), strings.TrimSpace(next.Type)) &&
		strings.TrimSpace(oldRecord.Value) == strings.TrimSpace(next.Value) &&
		equivalentLineID(oldRecord.LineID) == equivalentLineID(next.LineID)
}

func remoteMatchesRecord(remote dns.Record, record models.Record) bool {
	return strings.EqualFold(strings.TrimSpace(remote.Name), strings.TrimSpace(record.Name)) &&
		strings.EqualFold(strings.TrimSpace(remote.Type), strings.TrimSpace(record.Type)) &&
		strings.TrimSpace(remote.Value) == strings.TrimSpace(record.Value) &&
		equivalentLineID(remote.LineID) == equivalentLineID(record.LineID)
}

func equivalentLineID(lineID string) string {
	lineID = strings.ToLower(strings.TrimSpace(lineID))
	if lineID == "" || lineID == "0" || lineID == "default" || lineID == "默认" {
		return "default"
	}
	return lineID
}

func deleteRemoteRecordThenLocal(
	ctx context.Context,
	resolver ProviderResolver,
	enqueue enqueueDNSWriteJobFunc,
	domain models.Domain,
	record models.Record,
	source string,
	applyLocal func() error,
) (SubmitRecordResult, *apperrors.AppError) {
	provider, err := resolver.Resolve(ctx, domain)
	if err != nil {
		return SubmitRecordResult{}, dnsProviderError("域名配置错误", err)
	}
	zone := dns.Zone{ID: domain.RemoteZoneID, Domain: domain.Domain}
	// Empty remote ID means nothing to delete on the platform; continue with local cleanup.
	if remoteID := strings.TrimSpace(record.RecordID); remoteID != "" {
		if err := deleteRemoteRecordValue(ctx, provider, zone, remoteID, record); err != nil && !dns.IsNotFound(err) {
			return SubmitRecordResult{}, dnsProviderError("删除记录失败", err)
		}
	}
	if err := applyLocal(); err == nil {
		return SubmitRecordResult{Mode: "direct"}, nil
	} else {
		enqueueRecordRepair(ctx, enqueue, models.DNSWriteJob{
			UID: record.UID, Source: source, ProviderKey: domain.ProviderKey, Domain: domain.Domain,
			RecordName: record.Name, RecordType: record.Type, ValueDigest: digest(record.Value),
			RemoteRecordID: record.RecordID, Operation: "restore_deleted_record", Status: "pending",
			LastError: err.Error(), Payload: mustJSON(map[string]any{"record": record}),
		})
		return SubmitRecordResult{}, apperrors.Wrap(apperrors.CodeInternal, "本地删除失败，已记录待修复任务", err)
	}
}

func updateRemoteRecordValue(ctx context.Context, provider dns.Provider, zone dns.Zone, oldRecord models.Record, next models.Record) (dns.Record, error) {
	oldInput := dns.RecordInput{Name: oldRecord.Name, Type: oldRecord.Type, Value: oldRecord.Value, LineID: oldRecord.LineID}
	nextInput := dns.RecordInput{Name: next.Name, Type: next.Type, Value: next.Value, LineID: next.LineID}
	if manager, ok := provider.(dns.RecordValueManager); ok {
		return manager.UpdateRecordValue(ctx, zone, oldRecord.RecordID, oldInput, nextInput)
	}
	return provider.UpdateRecord(ctx, zone, oldRecord.RecordID, nextInput)
}

func deleteRemoteRecordValue(ctx context.Context, provider dns.Provider, zone dns.Zone, remoteID string, record models.Record) error {
	if manager, ok := provider.(dns.RecordValueManager); ok {
		return manager.DeleteRecordValue(ctx, zone, remoteID, dns.RecordInput{
			Name: record.Name, Type: record.Type, Value: record.Value, LineID: record.LineID,
		})
	}
	return provider.DeleteRecord(ctx, zone, remoteID)
}

func enqueueRecordRepair(ctx context.Context, enqueue enqueueDNSWriteJobFunc, job models.DNSWriteJob) {
	if enqueue == nil {
		return
	}
	_ = enqueue(ctx, job)
}
