/**
 * @Author: Joey
 * @Description: Protocol unit tests for serialization/deserialization
 * @Create Date: 2024-01-19
 */

package unitTest

import (
	"testing"

	easyCon "github.com/qiu-tec/easy-con.golang"
)

// TestNewProtocolReqSerialization tests the new protocol request serialization format
func TestNewProtocolReqSerialization(t *testing.T) {
	req := easyCon.PackReq{
		From:    "ModuleA",
		To:      "ModuleB",
		Route:   "testRoute",
		ReqTime: "2024-01-16 10:00:00.000",
		Content: []byte("test content"),
	}
	// Set exported fields from embedded packBase
	req.PType = easyCon.EPTypeReq
	req.Id = 12345

	data, err := req.Raw()
	if err != nil {
		t.Fatalf("Raw() failed: %v", err)
	}

	// 验证格式: [PackType(1 byte)][HeadLen(2 bytes)][Header JSON][Content Bytes]
	if len(data) < 3 {
		t.Fatalf("Data too short: %d", len(data))
	}

	if data[0] != easyCon.PackTypeReq {
		t.Errorf("PackType mismatch: got 0x%02x, want 0x%02x", data[0], easyCon.PackTypeReq)
	}

	headLen := int(data[1])<<8 | int(data[2])
	t.Logf("Header length: %d", headLen)
	t.Logf("Total data length: %d", len(data))

	// Verify header length is reasonable
	if headLen <= 0 || headLen > len(data)-3 {
		t.Errorf("Invalid header length: %d (total: %d)", headLen, len(data))
	}
}

// TestNewProtocolReqRoundTrip tests request serialization and deserialization round-trip
func TestNewProtocolReqRoundTrip(t *testing.T) {
	original := easyCon.PackReq{
		From:    "TestModule",
		To:      "TargetModule",
		Route:   "getUserInfo",
		ReqTime: "2024-01-16 11:00:00.000",
		Content: []byte(`{"name":"test"}`),
	}
	original.PType = easyCon.EPTypeReq
	original.Id = 99999

	data, err := original.Raw()
	if err != nil {
		t.Fatalf("Raw() failed: %v", err)
	}

	decoded, err := easyCon.UnmarshalPack(data)
	if err != nil {
		t.Fatalf("UnmarshalPack() failed: %v", err)
	}

	decodedReq, ok := decoded.(*easyCon.PackReq)
	if !ok {
		t.Fatalf("Not a PackReq, got %T", decoded)
	}

	if decodedReq.GetId() != original.GetId() {
		t.Errorf("Id mismatch: got %d, want %d", decodedReq.GetId(), original.GetId())
	}

	if decodedReq.From != original.From {
		t.Errorf("From mismatch: got %s, want %s", decodedReq.From, original.From)
	}

	if decodedReq.To != original.To {
		t.Errorf("To mismatch: got %s, want %s", decodedReq.To, original.To)
	}

	if string(decodedReq.Content) != string(original.Content) {
		t.Errorf("Content mismatch: got %s, want %s", string(decodedReq.Content), string(original.Content))
	}

	if decodedReq.GetType() != original.GetType() {
		t.Errorf("Type mismatch: got %s, want %s", decodedReq.GetType(), original.GetType())
	}
}

// TestProtocolAutoDetection tests protocol deserialization
func TestProtocolAutoDetection(t *testing.T) {
	req := easyCon.PackReq{
		From:    "A",
		To:      "B",
		Route:   "r",
		ReqTime: "2024-01-16 13:00:00.000",
		Content: []byte("data"),
	}
	req.PType = easyCon.EPTypeReq
	req.Id = 1

	data, err := req.Raw()
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	pack, err := easyCon.UnmarshalPack(data)
	if err != nil {
		t.Errorf("Failed to unmarshal: %v", err)
	}
	if pack.GetType() != easyCon.EPTypeReq {
		t.Errorf("Wrong type: got %s, want %s", pack.GetType(), easyCon.EPTypeReq)
	}
}

// TestNewProtocolRespSerialization tests response serialization with new protocol
func TestNewProtocolRespSerialization(t *testing.T) {
	req := easyCon.PackReq{
		From:    "ModuleA",
		To:      "ModuleB",
		Route:   "testRoute",
		ReqTime: "2024-01-16 15:00:00.000",
		Content: []byte("test content"),
	}
	req.PType = easyCon.EPTypeReq
	req.Id = 12345

	resp := easyCon.PackResp{
		PackReq:  req,
		RespTime: "2024-01-16 15:00:01.000",
		RespCode: easyCon.ERespSuccess,
	}

	data, err := resp.Raw()
	if err != nil {
		t.Fatalf("Raw() failed: %v", err)
	}

	if len(data) < 3 {
		t.Fatalf("Data too short: %d", len(data))
	}

	if data[0] != easyCon.PackTypeResp {
		t.Errorf("PackType mismatch: got 0x%02x, want 0x%02x", data[0], easyCon.PackTypeResp)
	}

	headLen := int(data[1])<<8 | int(data[2])
	t.Logf("Response header length: %d", headLen)
}

// TestNewProtocolRespRoundTrip tests response round-trip serialization/deserialization
func TestNewProtocolRespRoundTrip(t *testing.T) {
	packReq := easyCon.PackReq{
		From:    "TestModule",
		To:      "TargetModule",
		Route:   "testRoute",
		ReqTime: "2024-01-16 16:00:00.000",
		Content: []byte(`{"result":"success"}`),
	}
	packReq.PType = easyCon.EPTypeReq
	packReq.Id = 54321

	original := easyCon.PackResp{
		PackReq:  packReq,
		RespTime: "2024-01-16 16:00:01.000",
		RespCode: easyCon.ERespSuccess,
	}
	original.PType = easyCon.EPTypeResp

	data, err := original.Raw()
	if err != nil {
		t.Fatalf("Raw() failed: %v", err)
	}

	decoded, err := easyCon.UnmarshalPack(data)
	if err != nil {
		t.Fatalf("UnmarshalPack() failed: %v", err)
	}

	decodedResp, ok := decoded.(*easyCon.PackResp)
	if !ok {
		t.Fatalf("Not a PackResp, got %T", decoded)
	}

	if decodedResp.GetId() != original.GetId() {
		t.Errorf("Id mismatch: got %d, want %d", decodedResp.GetId(), original.GetId())
	}

	if decodedResp.RespCode != original.RespCode {
		t.Errorf("RespCode mismatch: got %d, want %d", decodedResp.RespCode, original.RespCode)
	}

	if decodedResp.From != original.From {
		t.Errorf("From mismatch: got %s, want %s", decodedResp.From, original.From)
	}
}

// TestNewProtocolNoticeSerialization tests notice serialization with new protocol
func TestNewProtocolNoticeSerialization(t *testing.T) {
	notice := easyCon.PackNotice{
		From:    "ModuleA",
		Route:   "testNotice",
		Retain:  false,
		Content: []byte("notice content"),
	}
	notice.PType = easyCon.EPTypeNotice
	notice.Id = 99999

	data, err := notice.Raw()
	if err != nil {
		t.Fatalf("Raw() failed: %v", err)
	}

	if len(data) < 3 {
		t.Fatalf("Data too short: %d", len(data))
	}

	if data[0] != easyCon.PackTypeNotice {
		t.Errorf("PackType mismatch: got 0x%02x, want 0x%02x", data[0], easyCon.PackTypeNotice)
	}
}

// TestNewProtocolNoticeRoundTrip tests notice round-trip serialization/deserialization
func TestNewProtocolNoticeRoundTrip(t *testing.T) {
	original := easyCon.PackNotice{
		From:    "TestModule",
		Route:   "noticeRoute",
		Retain:  true,
		Content: []byte(`{"msg":"notification"}`),
	}
	original.PType = easyCon.EPTypeNotice
	original.Id = 88888

	data, err := original.Raw()
	if err != nil {
		t.Fatalf("Raw() failed: %v", err)
	}

	decoded, err := easyCon.UnmarshalPack(data)
	if err != nil {
		t.Fatalf("UnmarshalPack() failed: %v", err)
	}

	decodedNotice, ok := decoded.(*easyCon.PackNotice)
	if !ok {
		t.Fatalf("Not a PackNotice, got %T", decoded)
	}

	if decodedNotice.GetId() != original.GetId() {
		t.Errorf("Id mismatch: got %d, want %d", decodedNotice.GetId(), original.GetId())
	}

	if decodedNotice.From != original.From {
		t.Errorf("From mismatch: got %s, want %s", decodedNotice.From, original.From)
	}

	if decodedNotice.Route != original.Route {
		t.Errorf("Route mismatch: got %s, want %s", decodedNotice.Route, original.Route)
	}

	if decodedNotice.Retain != original.Retain {
		t.Errorf("Retain mismatch: got %v, want %v", decodedNotice.Retain, original.Retain)
	}
}

// TestNewProtocolLogSerialization tests log serialization with new protocol
func TestNewProtocolLogSerialization(t *testing.T) {
	log := easyCon.PackLog{
		From:    "ModuleA",
		Level:   easyCon.ELogLevelError,
		LogTime: "2024-01-16 17:00:00.000",
		Content: "log content message",
	}
	log.PType = easyCon.EPTypeLog
	log.Id = 77777

	data, err := log.Raw()
	if err != nil {
		t.Fatalf("Raw() failed: %v", err)
	}

	if len(data) < 3 {
		t.Fatalf("Data too short: %d", len(data))
	}

	if data[0] != easyCon.PackTypeLog {
		t.Errorf("PackType mismatch: got 0x%02x, want 0x%02x", data[0], easyCon.PackTypeLog)
	}
}

// TestNewProtocolLogRoundTrip tests log round-trip serialization/deserialization
func TestNewProtocolLogRoundTrip(t *testing.T) {
	original := easyCon.PackLog{
		From:    "TestModule",
		Level:   easyCon.ELogLevelWarning,
		LogTime: "2024-01-16 18:00:00.000",
		Content: "warning message",
	}
	original.PType = easyCon.EPTypeLog
	original.Id = 66666

	data, err := original.Raw()
	if err != nil {
		t.Fatalf("Raw() failed: %v", err)
	}

	decoded, err := easyCon.UnmarshalPack(data)
	if err != nil {
		t.Fatalf("UnmarshalPack() failed: %v", err)
	}

	decodedLog, ok := decoded.(*easyCon.PackLog)
	if !ok {
		t.Fatalf("Not a PackLog, got %T", decoded)
	}

	if decodedLog.GetId() != original.GetId() {
		t.Errorf("Id mismatch: got %d, want %d", decodedLog.GetId(), original.GetId())
	}

	if decodedLog.From != original.From {
		t.Errorf("From mismatch: got %s, want %s", decodedLog.From, original.From)
	}

	if decodedLog.Level != original.Level {
		t.Errorf("Level mismatch: got %s, want %s", decodedLog.Level, original.Level)
	}

	if decodedLog.Content != original.Content {
		t.Errorf("Content mismatch: got %s, want %s", decodedLog.Content, original.Content)
	}
}

// TestEmptyContent tests serialization with empty content
func TestEmptyContent(t *testing.T) {
	req := easyCon.PackReq{
		From:    "ModuleA",
		To:      "ModuleB",
		Route:   "testRoute",
		ReqTime: "2024-01-16 19:00:00.000",
		Content: []byte{},
	}
	req.PType = easyCon.EPTypeReq
	req.Id = 55555

	data, err := req.Raw()
	if err != nil {
		t.Fatalf("Raw() failed: %v", err)
	}

	decoded, err := easyCon.UnmarshalPack(data)
	if err != nil {
		t.Fatalf("UnmarshalPack() failed: %v", err)
	}

	decodedReq, ok := decoded.(*easyCon.PackReq)
	if !ok {
		t.Fatalf("Not a PackReq, got %T", decoded)
	}

	if len(decodedReq.Content) != 0 {
		t.Errorf("Content should be empty, got %d bytes", len(decodedReq.Content))
	}
}

// TestNilContent tests serialization with nil content
func TestNilContent(t *testing.T) {
	req := easyCon.PackReq{
		From:    "ModuleA",
		To:      "ModuleB",
		Route:   "testRoute",
		ReqTime: "2024-01-16 20:00:00.000",
		Content: nil,
	}
	req.PType = easyCon.EPTypeReq
	req.Id = 44444

	data, err := req.Raw()
	if err != nil {
		t.Fatalf("Raw() failed: %v", err)
	}

	decoded, err := easyCon.UnmarshalPack(data)
	if err != nil {
		t.Fatalf("UnmarshalPack() failed: %v", err)
	}

	decodedReq, ok := decoded.(*easyCon.PackReq)
	if !ok {
		t.Fatalf("Not a PackReq, got %T", decoded)
	}

	if len(decodedReq.Content) != 0 {
		t.Errorf("Content should be empty, got %d bytes", len(decodedReq.Content))
	}
}

// TestLargeContent tests serialization with large content
func TestLargeContent(t *testing.T) {
	largeContent := make([]byte, 10000)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	req := easyCon.PackReq{
		From:    "ModuleA",
		To:      "ModuleB",
		Route:   "testRoute",
		ReqTime: "2024-01-16 21:00:00.000",
		Content: largeContent,
	}
	req.PType = easyCon.EPTypeReq
	req.Id = 33333

	data, err := req.Raw()
	if err != nil {
		t.Fatalf("Raw() failed: %v", err)
	}

	decoded, err := easyCon.UnmarshalPack(data)
	if err != nil {
		t.Fatalf("UnmarshalPack() failed: %v", err)
	}

	decodedReq, ok := decoded.(*easyCon.PackReq)
	if !ok {
		t.Fatalf("Not a PackReq, got %T", decoded)
	}

	if len(decodedReq.Content) != len(largeContent) {
		t.Errorf("Content length mismatch: got %d, want %d", len(decodedReq.Content), len(largeContent))
	}

	// Verify content integrity
	for i := range largeContent {
		if decodedReq.Content[i] != largeContent[i] {
			t.Errorf("Content mismatch at byte %d: got %d, want %d", i, decodedReq.Content[i], largeContent[i])
			break
		}
	}
}
