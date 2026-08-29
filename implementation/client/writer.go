package client

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const protocol = "MSP/1"

var publicationIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type ErrorKind string

const (
	InvalidInput     ErrorKind = "invalid_input"
	ShortWrite       ErrorKind = "short_write"
	ClosedConnection ErrorKind = "closed_connection"
	Timeout          ErrorKind = "timeout"
	InvalidResponse  ErrorKind = "invalid_response"
	ResponseMismatch ErrorKind = "response_mismatch"
	Rejected         ErrorKind = "rejected"
)

type Error struct {
	Kind ErrorKind
	Code string
	err  error
}

func (failure *Error) Error() string {
	if failure.Code != "" {
		return fmt.Sprintf("msp client %s: %s", failure.Kind, failure.Code)
	}
	return fmt.Sprintf("msp client %s", failure.Kind)
}

func (failure *Error) Unwrap() error { return failure.err }

func ErrorKindOf(err error) (ErrorKind, bool) {
	var failure *Error
	if !errors.As(err, &failure) {
		return "", false
	}
	return failure.Kind, true
}

type Config struct {
	FrameBytes   int
	Parts        int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func DefaultConfig() Config {
	return Config{FrameBytes: 8 << 10, Parts: 256, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}
}

type Writer struct {
	connection net.Conn
	reader     *bufio.Reader
	config     Config
	gate       chan struct{}
}

func NewWriter(connection net.Conn, configuration Config) (*Writer, error) {
	if connection == nil || configuration.FrameBytes < 64 || configuration.Parts < 1 || configuration.ReadTimeout <= 0 || configuration.WriteTimeout <= 0 {
		return nil, &Error{Kind: InvalidInput}
	}
	gate := make(chan struct{}, 1)
	gate <- struct{}{}
	return &Writer{connection: connection, reader: bufio.NewReaderSize(connection, configuration.FrameBytes), config: configuration, gate: gate}, nil
}

func (writer *Writer) Publish(ctx context.Context, publicationID string, candidate []byte) (uint64, error) {
	if !publicationIDPattern.MatchString(publicationID) || len(candidate) == 0 {
		return 0, &Error{Kind: InvalidInput}
	}
	select {
	case <-ctx.Done():
		return 0, classify(ctx.Err())
	case <-writer.gate:
	}
	defer func() { writer.gate <- struct{}{} }()

	parts, err := split(candidate, publicationID, writer.config.FrameBytes, writer.config.Parts)
	if err != nil {
		return 0, err
	}
	if err := writer.setDeadline(ctx); err != nil {
		return 0, classify(err)
	}
	defer writer.connection.SetDeadline(time.Time{})

	if err := writer.roundTrip(
		fmt.Sprintf("%s SNAPSHOT_BEGIN %s %d %d\n", protocol, publicationID, len(parts), len(candidate)),
		[]string{protocol, "FRAME_ACCEPTED", publicationID, "BEGIN"}, publicationID,
	); err != nil {
		return 0, err
	}
	for index, part := range parts {
		if err := writer.roundTrip(
			fmt.Sprintf("%s SNAPSHOT_PART %s %d %s\n", protocol, publicationID, index, base64.RawURLEncoding.EncodeToString(part)),
			[]string{protocol, "FRAME_ACCEPTED", publicationID, strconv.Itoa(index)}, publicationID,
		); err != nil {
			return 0, err
		}
	}
	if err := writeAll(writer.connection, []byte(fmt.Sprintf("%s SNAPSHOT_COMMIT %s\n", protocol, publicationID))); err != nil {
		return 0, classify(err)
	}
	fields, err := writer.response(publicationID)
	if err != nil {
		return 0, err
	}
	if len(fields) != 4 || fields[0] != protocol || fields[1] != "ACK" || fields[2] != publicationID {
		return 0, responseError(fields, publicationID)
	}
	generation, parseErr := strconv.ParseUint(fields[3], 10, 64)
	if parseErr != nil || fields[3] == "" || (len(fields[3]) > 1 && fields[3][0] == '0') {
		return 0, &Error{Kind: InvalidResponse}
	}
	return generation, nil
}

func (writer *Writer) roundTrip(frame string, expected []string, publicationID string) error {
	if len(frame) > writer.config.FrameBytes {
		return &Error{Kind: InvalidInput}
	}
	if err := writeAll(writer.connection, []byte(frame)); err != nil {
		return classify(err)
	}
	fields, err := writer.response(publicationID)
	if err != nil {
		return err
	}
	if len(fields) != len(expected) {
		return responseError(fields, publicationID)
	}
	for index := range fields {
		if fields[index] != expected[index] {
			return responseError(fields, publicationID)
		}
	}
	return nil
}

func (writer *Writer) response(publicationID string) ([]string, error) {
	line, err := writer.reader.ReadString('\n')
	if err != nil {
		return nil, classify(err)
	}
	if len(line) > writer.config.FrameBytes || !strings.HasSuffix(line, "\n") {
		return nil, &Error{Kind: InvalidResponse}
	}
	fields := strings.Fields(strings.TrimSuffix(line, "\n"))
	if len(fields) == 4 && fields[0] == protocol && fields[1] == "NACK" {
		if fields[2] != publicationID {
			return nil, &Error{Kind: ResponseMismatch}
		}
		return nil, &Error{Kind: Rejected, Code: fields[3]}
	}
	return fields, nil
}

func responseError(fields []string, publicationID string) error {
	if len(fields) > 2 && fields[2] != publicationID {
		return &Error{Kind: ResponseMismatch}
	}
	return &Error{Kind: InvalidResponse}
}

func (writer *Writer) setDeadline(ctx context.Context) error {
	deadline := time.Now().Add(max(writer.config.ReadTimeout, writer.config.WriteTimeout))
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	return writer.connection.SetDeadline(deadline)
}

func split(candidate []byte, publicationID string, frameBytes, maxParts int) ([][]byte, error) {
	parts := make([][]byte, 0, maxParts)
	for offset := 0; offset < len(candidate); {
		index := len(parts)
		if index >= maxParts {
			return nil, &Error{Kind: InvalidInput}
		}
		characters := frameBytes - len("MSP/1 SNAPSHOT_PART ") - len(publicationID) - 1 - len(strconv.Itoa(index)) - 1 - 1
		bytes := characters * 3 / 4
		for bytes > 0 && base64.RawURLEncoding.EncodedLen(bytes) > characters {
			bytes--
		}
		if bytes < 1 {
			return nil, &Error{Kind: InvalidInput}
		}
		end := min(len(candidate), offset+bytes)
		parts = append(parts, candidate[offset:end])
		offset = end
	}
	return parts, nil
}

func writeAll(destination io.Writer, content []byte) error {
	for len(content) > 0 {
		written, err := destination.Write(content)
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

func classify(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return &Error{Kind: Timeout, err: err}
	}
	if errors.Is(err, io.ErrShortWrite) {
		return &Error{Kind: ShortWrite, err: err}
	}
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe) {
		return &Error{Kind: ClosedConnection, err: err}
	}
	if timeoutError, ok := err.(net.Error); ok && timeoutError.Timeout() {
		return &Error{Kind: Timeout, err: err}
	}
	return &Error{Kind: ClosedConnection, err: err}
}
