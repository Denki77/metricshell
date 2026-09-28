package managedprotocol

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/Denki77/metricshell/implementation/internal/managed"
)

const Version = 1

type Code string

const (
	CodeEmptyFrame         Code = "empty_frame"
	CodePartialFrame       Code = "partial_frame"
	CodeFrameTooLarge      Code = "frame_too_large"
	CodeMultipleFrames     Code = "multiple_frames"
	CodeMalformedJSON      Code = "malformed_json"
	CodeMissingVersion     Code = "missing_version"
	CodeInvalidVersion     Code = "invalid_version"
	CodeUnsupportedVersion Code = "unsupported_version"
	CodeInvalidRequest     Code = "invalid_request"
)

type ProtocolError struct{ Code Code }

func (err *ProtocolError) Error() string { return "managed protocol rejection: " + string(err.Code) }

func ErrorCode(err error) (Code, bool) {
	var protocolError *ProtocolError
	if !errors.As(err, &protocolError) {
		return "", false
	}
	return protocolError.Code, true
}

func reject(code Code) error { return &ProtocolError{Code: code} }

type Submitter interface {
	Submit(context.Context, managed.Mutation) managed.Result
}

type RejectionObserver interface {
	ProtocolRejected(Code)
}

type Response struct {
	Version    int             `json:"version"`
	Outcome    managed.Outcome `json:"outcome"`
	Generation uint64          `json:"generation,omitempty"`
	Commit     uint64          `json:"commit,omitempty"`
	Reason     string          `json:"reason,omitempty"`
}

func ParseFrame(frame []byte, maximum int) (managed.Mutation, error) {
	if maximum < 1 {
		return managed.Mutation{}, reject(CodeFrameTooLarge)
	}
	newline := bytes.IndexByte(frame, '\n')
	if newline < 0 {
		if len(frame) > maximum {
			return managed.Mutation{}, reject(CodeFrameTooLarge)
		}
		return managed.Mutation{}, reject(CodePartialFrame)
	}
	if newline == 0 {
		return managed.Mutation{}, reject(CodeEmptyFrame)
	}
	if newline > maximum {
		return managed.Mutation{}, reject(CodeFrameTooLarge)
	}
	if newline != len(frame)-1 {
		return managed.Mutation{}, reject(CodeMultipleFrames)
	}
	payload := frame[:newline]
	if err := validateUniqueJSON(payload); err != nil {
		return managed.Mutation{}, reject(CodeMalformedJSON)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(payload, &object); err != nil || object == nil {
		return managed.Mutation{}, reject(CodeMalformedJSON)
	}
	versionRaw, exists := object["version"]
	if !exists {
		return managed.Mutation{}, reject(CodeMissingVersion)
	}
	var version int
	if err := json.Unmarshal(versionRaw, &version); err != nil {
		return managed.Mutation{}, reject(CodeInvalidVersion)
	}
	if version != Version {
		return managed.Mutation{}, reject(CodeUnsupportedVersion)
	}
	var operation string
	if err := json.Unmarshal(object["op"], &operation); err != nil || operation == "" {
		return managed.Mutation{}, reject(CodeInvalidRequest)
	}
	switch managed.OperationKind(operation) {
	case "declare":
		return parseDeclaration(object)
	case managed.CounterInitialize, managed.CounterAdd, managed.GaugeSet, managed.HistogramObserve:
		return parseOperation(object, managed.OperationKind(operation))
	default:
		return managed.Mutation{}, reject(CodeInvalidRequest)
	}
}

func Handle(ctx context.Context, submitter Submitter, frame []byte, maximum int) Response {
	return HandleObserved(ctx, submitter, frame, maximum, nil)
}

func HandleObserved(ctx context.Context, submitter Submitter, frame []byte, maximum int, observer RejectionObserver) Response {
	mutation, err := ParseFrame(frame, maximum)
	if err != nil {
		code, _ := ErrorCode(err)
		if observer != nil {
			observer.ProtocolRejected(code)
		}
		return Response{Version: Version, Outcome: "protocol", Reason: string(code)}
	}
	result := submitter.Submit(ctx, mutation)
	return Response{
		Version: Version, Outcome: result.Outcome, Generation: result.Generation,
		Commit: result.Commit, Reason: string(result.Reason),
	}
}

func EncodeResponse(response Response) ([]byte, error) {
	content, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func ReadFrame(reader io.Reader, maximum int) ([]byte, error) {
	if maximum < 1 {
		return nil, reject(CodeFrameTooLarge)
	}
	buffer := bufio.NewReaderSize(reader, maximum+1)
	content, err := buffer.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) || len(content) > maximum+1 {
		return nil, reject(CodeFrameTooLarge)
	}
	if errors.Is(err, io.EOF) {
		return content, reject(CodePartialFrame)
	}
	if err != nil {
		return nil, err
	}
	if buffer.Buffered() != 0 {
		return content, reject(CodeMultipleFrames)
	}
	return content, nil
}

func parseDeclaration(object map[string]json.RawMessage) (managed.Mutation, error) {
	if !exactFields(object, "version", "op", "name", "help", "type", "label_names", "buckets") {
		return managed.Mutation{}, reject(CodeInvalidRequest)
	}
	var name, help string
	var metricType managed.MetricType
	var labels []string
	var bucketsRaw []json.RawMessage
	if json.Unmarshal(object["name"], &name) != nil || json.Unmarshal(object["help"], &help) != nil ||
		json.Unmarshal(object["type"], &metricType) != nil || json.Unmarshal(object["label_names"], &labels) != nil || labels == nil ||
		json.Unmarshal(object["buckets"], &bucketsRaw) != nil || bucketsRaw == nil {
		return managed.Mutation{}, reject(CodeInvalidRequest)
	}
	buckets := make([]float64, 0, len(bucketsRaw))
	for _, raw := range bucketsRaw {
		value, err := parseNumber(raw)
		if err != nil {
			return managed.Mutation{}, reject(CodeInvalidRequest)
		}
		buckets = append(buckets, value)
	}
	descriptor := managed.Descriptor{Name: name, Help: help, Type: metricType, Labels: labels, Buckets: buckets}
	return managed.Mutation{Descriptor: &descriptor}, nil
}

func parseOperation(object map[string]json.RawMessage, kind managed.OperationKind) (managed.Mutation, error) {
	if !exactFields(object, "version", "op", "name", "labels", "value") {
		return managed.Mutation{}, reject(CodeInvalidRequest)
	}
	var name string
	var labels map[string]string
	if json.Unmarshal(object["name"], &name) != nil || json.Unmarshal(object["labels"], &labels) != nil || labels == nil {
		return managed.Mutation{}, reject(CodeInvalidRequest)
	}
	value, err := parseNumber(object["value"])
	if err != nil {
		return managed.Mutation{}, reject(CodeInvalidRequest)
	}
	return managed.Mutation{Operations: []managed.Operation{{Kind: kind, Name: name, Labels: labels, Value: value}}}, nil
}

func parseNumber(raw json.RawMessage) (float64, error) {
	var text string
	if len(raw) > 0 && raw[0] == '"' {
		if err := json.Unmarshal(raw, &text); err != nil {
			return 0, err
		}
	} else {
		text = string(raw)
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func exactFields(object map[string]json.RawMessage, fields ...string) bool {
	if len(object) != len(fields) {
		return false
	}
	for _, field := range fields {
		if _, exists := object[field]; !exists {
			return false
		}
	}
	return true
}

func validateUniqueJSON(content []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	if err := scanValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing JSON value")
	}
	return nil
}

func scanValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, composite := token.(json.Delim)
	if !composite {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return fmt.Errorf("duplicate JSON member")
			}
			seen[name] = true
			if err := scanValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanValue(decoder); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("invalid JSON delimiter")
	}
	_, err = decoder.Token()
	return err
}
