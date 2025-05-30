package tencentcloud

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/libdns/libdns"
)

const (
	endpoint = "https://dnspod.tencentcloudapi.com"

	DescribeRecordList = "DescribeRecordList"
	CreateRecord       = "CreateRecord"
	ModifyRecord       = "ModifyRecord"
	DeleteRecord       = "DeleteRecord"
)

func (p *Provider) listRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	domain := strings.TrimSuffix(zone, ".")

	requestData := FindRecordRequest{
		Domain:     domain,
		RecordLine: "默认",
	}

	payload, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	resp, err := p.sendRequest(ctx, DescribeRecordList, string(payload))
	if err != nil {
		return nil, err
	}

	var response Response
	if err = json.Unmarshal(resp, &response); err != nil {
		return nil, err
	}

	list := make([]libdns.Record, 0, len(response.Response.RecordList))
	for _, record := range response.Response.RecordList {
		list = append(list, libdns.Record{
			ID:    strconv.FormatInt(record.RecordId, 10),
			Type:  record.Type,
			Name:  record.Name,
			Value: record.Value,
			TTL:   time.Duration(record.TTL) * time.Second,
		})
	}

	return list, nil
}

func (p *Provider) createRecord(ctx context.Context, zone string, record libdns.Record) error {
	domain := strings.TrimSuffix(zone, ".")

	requestData := CreateModifyRecordRequest{
		Domain:     domain,
		SubDomain:  record.Name,
		RecordType: record.Type,
		RecordLine: "默认",
		Value:      record.Value,
		TTL:        int64(record.TTL.Seconds()),
	}

	payload, err := json.Marshal(requestData)
	if err != nil {
		return err
	}

	resp, err := p.sendRequest(ctx, CreateRecord, string(payload))
	if err != nil {
		return err
	}

	var response Response
	if err := json.Unmarshal(resp, &response); err != nil {
		return err
	}

	if response.Response.RecordId == 0 {
		return ErrNotValid
	}

	return nil
}

func (p *Provider) modifyRecord(ctx context.Context, zone string, record libdns.Record) error {
	domain := strings.TrimSuffix(zone, ".")

	recordId, err := strconv.ParseUint(record.ID, 10, 64)
	if err != nil {
		return err
	}
	requestData := CreateModifyRecordRequest{
		Domain:     domain,
		SubDomain:  record.Name,
		RecordType: record.Type,
		RecordLine: "默认",
		Value:      record.Value,
		TTL:        int64(record.TTL.Seconds()),
		RecordId:   recordId,
	}

	payload, err := json.Marshal(requestData)
	if err != nil {
		return err
	}

	_, err = p.sendRequest(ctx, ModifyRecord, string(payload))
	return err
}

func (p *Provider) deleteRecord(ctx context.Context, zone string, record libdns.Record) error {
	domain := strings.TrimSuffix(zone, ".")

	requestData := DeleteRecordRequest{
		Domain:   domain,
		RecordId: record.ID,
	}

	payload, err := json.Marshal(requestData)
	if err != nil {
		return err
	}

	_, err = p.sendRequest(ctx, DeleteRecord, string(payload))
	return err
}

func (p *Provider) findRecord(ctx context.Context, zone string, record libdns.Record) (uint64, error) {
	domain := strings.TrimSuffix(zone, ".")

	requestData := FindRecordRequest{
		Domain:     domain,
		RecordType: record.Type,
		RecordLine: "默认",
		Subdomain:  record.Name,
		Limit:      3000,
	}

	payload, err := json.Marshal(requestData)
	if err != nil {
		return 0, err
	}

	resp, err := p.sendRequest(ctx, DescribeRecordList, string(payload))
	if err != nil {
		return 0, err
	}

	var response Response
	if err = json.Unmarshal(resp, &response); err != nil {
		return 0, err
	}

	var recordId uint64
	for _, item := range response.Response.RecordList {
		if item.Name == record.Name && item.Type == record.Type {
			if record.Value != "" && item.Value != record.Value {
				continue
			}
			recordId = uint64(item.RecordId)
			break
		}
	}

	if recordId == 0 {
		return 0, ErrRecordNotFound
	}

	return recordId, nil
}

func (p *Provider) sendRequest(ctx context.Context, action string, data string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-TC-Version", "2021-03-23")

	SignRequest(p.SecretId, p.SecretKey, req, action, data)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}
