package otelutil

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
)

func KvListToMap(kvs []*commonpb.KeyValue) map[string]string {
	m := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		m[kv.GetKey()] = AnyValueToString(kv.GetValue())
	}
	return m
}

func AnyValueToString(v *commonpb.AnyValue) string {
	if v == nil {
		return ""
	}

	switch val := v.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return val.StringValue
	case *commonpb.AnyValue_BoolValue:
		return fmt.Sprintf("%t", val.BoolValue)
	case *commonpb.AnyValue_IntValue:
		return fmt.Sprintf("%d", val.IntValue)
	case *commonpb.AnyValue_DoubleValue:
		return fmt.Sprintf("%g", val.DoubleValue)
	case *commonpb.AnyValue_BytesValue:
		return hex.EncodeToString(val.BytesValue)
	case *commonpb.AnyValue_ArrayValue, *commonpb.AnyValue_KvlistValue:
		// Nested structures get JSON-serialized into the string map.
		b, err := json.Marshal(anyValueToInterface(v))
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	default:
		return ""
	}
}

func anyValueToInterface(v *commonpb.AnyValue) any {
	if v == nil {
		return nil
	}

	switch val := v.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return val.StringValue
	case *commonpb.AnyValue_BoolValue:
		return val.BoolValue
	case *commonpb.AnyValue_IntValue:
		return val.IntValue
	case *commonpb.AnyValue_DoubleValue:
		return val.DoubleValue
	case *commonpb.AnyValue_BytesValue:
		return hex.EncodeToString(val.BytesValue)
	case *commonpb.AnyValue_ArrayValue:
		if val.ArrayValue == nil {
			return nil
		}
		arr := make([]any, len(val.ArrayValue.GetValues()))
		for i, v := range val.ArrayValue.GetValues() {
			arr[i] = anyValueToInterface(v)
		}
		return arr
	case *commonpb.AnyValue_KvlistValue:
		if val.KvlistValue == nil {
			return nil
		}
		m := make(map[string]any, len(val.KvlistValue.GetValues()))
		for _, kv := range val.KvlistValue.GetValues() {
			m[kv.GetKey()] = anyValueToInterface(kv.GetValue())
		}
		return m
	default:
		return nil
	}
}
