package socketingest

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Denki77/metricshell/implementation/internal/ingestion"
)

const protocol = "MSP/1"

var publicationIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type Config struct {
	FrameBytes         int
	Parts              int
	Connections        int
	Transactions       int
	TransactionTimeout time.Duration
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	DecodedBytes       int
	SnapshotBytes      int
}

func DefaultConfig() Config {
	return Config{
		FrameBytes: 8 << 10, Parts: 256, Connections: 8, Transactions: 4,
		TransactionTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second,
		DecodedBytes: 2 << 20, SnapshotBytes: 1 << 20,
	}
}

func EffectiveDecodedCapacity(frameBytes, parts int) int {
	total := 0
	for index := 0; index < parts; index++ {
		payloadCharacters := frameBytes - len("MSP/1 SNAPSHOT_PART ") - 64 - 1 - len(strconv.Itoa(index)) - 1 - 1
		if payloadCharacters > 0 {
			total += payloadCharacters * 3 / 4
		}
	}
	return total
}

func (configuration Config) Validate() error {
	if configuration.FrameBytes < 64 || configuration.Parts < 1 || configuration.Connections < 1 || configuration.Transactions < 1 ||
		configuration.TransactionTimeout <= 0 || configuration.ReadTimeout <= 0 || configuration.WriteTimeout <= 0 ||
		configuration.DecodedBytes < 1 || configuration.SnapshotBytes < 1 || configuration.DecodedBytes < configuration.SnapshotBytes ||
		configuration.FrameBytes > configuration.DecodedBytes || EffectiveDecodedCapacity(configuration.FrameBytes, configuration.Parts) < configuration.SnapshotBytes {
		return errors.New("invalid socket ingestion configuration")
	}
	return nil
}

type Observer interface {
	FrameRejected(ingestion.TransportFailure)
	TransactionExpired()
	ConnectionChanged(int)
}

type noopObserver struct{}

func (noopObserver) FrameRejected(ingestion.TransportFailure) {}
func (noopObserver) TransactionExpired()                      {}
func (noopObserver) ConnectionChanged(int)                    {}

type Protocol struct {
	configuration Config
	publisher     ingestion.Publisher
	observer      Observer
	transactions  chan struct{}
	now           func() time.Time
}

func NewProtocol(configuration Config, publisher ingestion.Publisher, observer Observer) (*Protocol, error) {
	if err := configuration.Validate(); err != nil || publisher == nil {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("publisher is required")
	}
	if observer == nil {
		observer = noopObserver{}
	}
	return &Protocol{configuration: configuration, publisher: publisher, observer: observer, transactions: make(chan struct{}, configuration.Transactions), now: time.Now}, nil
}

type transaction struct {
	id       string
	parts    [][]byte
	next     int
	declared int
	actual   int
	expires  time.Time
	release  sync.Once
}

func (handler *Protocol) ServeConnection(ctx context.Context, connection net.Conn) error {
	reader := bufio.NewReaderSize(connection, handler.configuration.FrameBytes)
	transactions := make(map[string]*transaction)
	defer func() {
		for _, candidate := range transactions {
			handler.release(candidate)
		}
	}()
	for {
		if err := connection.SetReadDeadline(time.Now().Add(handler.configuration.ReadTimeout)); err != nil {
			if connectionClosed(err) {
				return nil
			}
			return err
		}
		line, failure, err := readFrame(reader, handler.configuration.FrameBytes)
		if err != nil {
			if errors.Is(err, io.EOF) || connectionClosed(err) {
				return nil
			}
			if timeoutError, ok := err.(net.Error); ok && timeoutError.Timeout() {
				return nil
			}
			return err
		}
		if failure != ingestion.FailureNone {
			handler.observer.FrameRejected(failure)
			if err := handler.write(connection, nack("invalid", failure)); err != nil {
				return err
			}
			continue
		}
		response, closeConnection := handler.process(ctx, strings.TrimSuffix(line, "\n"), transactions)
		if response != "" {
			if err := handler.write(connection, response); err != nil {
				return err
			}
		}
		if closeConnection {
			return nil
		}
	}
}

func connectionClosed(err error) bool {
	return errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe)
}

func readFrame(reader *bufio.Reader, limit int) (string, ingestion.TransportFailure, error) {
	line, err := reader.ReadSlice('\n')
	if errors.Is(err, bufio.ErrBufferFull) {
		for errors.Is(err, bufio.ErrBufferFull) {
			_, err = reader.ReadSlice('\n')
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return "", ingestion.FailureFrameLimit, err
		}
		return "", ingestion.FailureFrameLimit, nil
	}
	if err != nil {
		return "", ingestion.FailureNone, err
	}
	if len(line) > limit {
		return "", ingestion.FailureFrameLimit, nil
	}
	return string(line), ingestion.FailureNone, nil
}

func (handler *Protocol) process(ctx context.Context, line string, transactions map[string]*transaction) (string, bool) {
	fields := strings.Split(line, " ")
	if len(fields) < 2 || fields[0] != protocol {
		failure := ingestion.FailureMalformed
		if len(fields) > 0 && strings.HasPrefix(fields[0], "MSP/") {
			failure = ingestion.FailureProtocolVersion
		}
		handler.observer.FrameRejected(failure)
		return nack(responseID(fields), failure), false
	}
	switch fields[1] {
	case "SNAPSHOT_BEGIN":
		return handler.begin(fields, transactions), false
	case "SNAPSHOT_PART":
		return handler.part(fields, transactions), false
	case "SNAPSHOT_COMMIT":
		return handler.commit(ctx, fields, transactions), false
	default:
		handler.observer.FrameRejected(ingestion.FailureMalformed)
		return nack(responseID(fields), ingestion.FailureMalformed), false
	}
}

func (handler *Protocol) begin(fields []string, transactions map[string]*transaction) string {
	if len(fields) != 5 || !publicationIDPattern.MatchString(fields[2]) {
		return handler.reject(responseID(fields), ingestion.FailureMalformed)
	}
	id := fields[2]
	parts, partsOK := canonicalUint(fields[3])
	decoded, decodedOK := canonicalUint(fields[4])
	if !partsOK || parts < 1 || parts > handler.configuration.Parts {
		return handler.reject(id, ingestion.FailurePartLimit)
	}
	if !decodedOK || decoded < 1 || decoded > handler.configuration.DecodedBytes {
		return handler.reject(id, ingestion.FailureTransactionInvalid)
	}
	if _, exists := transactions[id]; exists {
		return handler.reject(id, ingestion.FailureTransactionInvalid)
	}
	select {
	case handler.transactions <- struct{}{}:
	default:
		return nack(id, ingestion.Busy)
	}
	transactions[id] = &transaction{id: id, parts: make([][]byte, parts), declared: decoded, expires: handler.now().Add(handler.configuration.TransactionTimeout)}
	return fmt.Sprintf("%s FRAME_ACCEPTED %s BEGIN\n", protocol, id)
}

func (handler *Protocol) part(fields []string, transactions map[string]*transaction) string {
	if len(fields) != 5 || !publicationIDPattern.MatchString(fields[2]) {
		return handler.reject(responseID(fields), ingestion.FailureMalformed)
	}
	id := fields[2]
	candidate, exists := transactions[id]
	if !exists {
		return handler.reject(id, ingestion.FailureTransactionInvalid)
	}
	if !handler.now().Before(candidate.expires) {
		delete(transactions, id)
		handler.release(candidate)
		handler.observer.TransactionExpired()
		return handler.reject(id, ingestion.FailureTransactionExpired)
	}
	index, ok := canonicalUint(fields[3])
	if !ok || index >= len(candidate.parts) {
		return handler.reject(id, ingestion.FailurePartLimit)
	}
	if index < candidate.next {
		return handler.reject(id, ingestion.FailureDuplicatePart)
	}
	if index > candidate.next {
		return handler.reject(id, ingestion.FailureMissingPart)
	}
	if strings.Contains(fields[4], "=") {
		return handler.reject(id, ingestion.FailureMalformed)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(fields[4])
	if err != nil || len(decoded) == 0 || candidate.actual+len(decoded) > candidate.declared || candidate.actual+len(decoded) > handler.configuration.DecodedBytes {
		return handler.reject(id, ingestion.FailureTransactionInvalid)
	}
	candidate.parts[index] = decoded
	candidate.actual += len(decoded)
	candidate.next++
	return fmt.Sprintf("%s FRAME_ACCEPTED %s %d\n", protocol, id, index)
}

func (handler *Protocol) commit(ctx context.Context, fields []string, transactions map[string]*transaction) string {
	if len(fields) != 3 || !publicationIDPattern.MatchString(fields[2]) {
		return handler.reject(responseID(fields), ingestion.FailureMalformed)
	}
	id := fields[2]
	candidate, exists := transactions[id]
	if !exists {
		return handler.reject(id, ingestion.FailureTransactionInvalid)
	}
	delete(transactions, id)
	defer handler.release(candidate)
	if !handler.now().Before(candidate.expires) {
		handler.observer.TransactionExpired()
		return handler.reject(id, ingestion.FailureTransactionExpired)
	}
	if candidate.next != len(candidate.parts) {
		return handler.reject(id, ingestion.FailureMissingPart)
	}
	if candidate.actual != candidate.declared {
		return handler.reject(id, ingestion.FailureTransactionInvalid)
	}
	content := make([]byte, 0, candidate.actual)
	for _, part := range candidate.parts {
		content = append(content, part...)
	}
	publicationContext, cancel := context.WithDeadline(ctx, candidate.expires)
	defer cancel()
	result := handler.publisher.Publish(publicationContext, ingestion.Unix, content)
	switch result.Outcome {
	case ingestion.Accepted:
		return fmt.Sprintf("%s ACK %s %d\n", protocol, id, result.Generation)
	case ingestion.Rejected:
		return nack(id, result.Reason)
	case ingestion.Busy:
		return nack(id, ingestion.Busy)
	case ingestion.Timeout:
		return nack(id, ingestion.Timeout)
	default:
		return nack(id, snapshotInternal)
	}
}

const snapshotInternal = "internal"

func (handler *Protocol) reject(id string, failure ingestion.TransportFailure) string {
	handler.observer.FrameRejected(failure)
	return nack(id, failure)
}

func (handler *Protocol) release(candidate *transaction) {
	candidate.release.Do(func() { <-handler.transactions })
}

func (handler *Protocol) write(connection net.Conn, frame string) error {
	if err := connection.SetWriteDeadline(time.Now().Add(handler.configuration.WriteTimeout)); err != nil {
		return err
	}
	content := []byte(frame)
	for len(content) > 0 {
		written, err := connection.Write(content)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		content = content[written:]
	}
	return nil
}

func canonicalUint(value string) (int, bool) {
	if value == "" || (len(value) > 1 && value[0] == '0') {
		return 0, false
	}
	parsed, err := strconv.ParseUint(value, 10, 31)
	return int(parsed), err == nil
}

func responseID(fields []string) string {
	if len(fields) > 2 && publicationIDPattern.MatchString(fields[2]) {
		return fields[2]
	}
	return "invalid"
}

func nack(id string, code any) string {
	return fmt.Sprintf("%s NACK %s %s\n", protocol, id, code)
}
