package snapshot

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

func Parse(content []byte, limits Limits) (ValidatedSnapshot, error) {
	if len(content) == 0 {
		return ValidatedSnapshot{}, Reject(ReasonEmptyPayload)
	}
	if len(content) > limits.DecodedBytes {
		return ValidatedSnapshot{}, Reject(ReasonPayloadLimit)
	}
	if !utf8.Valid(content) {
		return ValidatedSnapshot{}, Reject(ReasonMalformed)
	}
	if err := validateJSONStructure(content); err != nil {
		var duplicate duplicateKeyError
		if errors.As(err, &duplicate) && duplicate.labels {
			return ValidatedSnapshot{}, Reject(ReasonDuplicateSeries)
		}
		return ValidatedSnapshot{}, Reject(ReasonMalformed)
	}

	top, err := decodeExactObject(content, "schema_version", "families")
	if err != nil {
		return ValidatedSnapshot{}, Reject(ReasonMalformed)
	}
	var schemaVersion int
	if err := json.Unmarshal(top["schema_version"], &schemaVersion); err != nil || schemaVersion != SchemaVersion {
		return ValidatedSnapshot{}, Reject(ReasonSchemaVersion)
	}
	var encodedFamilies []json.RawMessage
	if err := json.Unmarshal(top["families"], &encodedFamilies); err != nil || encodedFamilies == nil {
		return ValidatedSnapshot{}, Reject(ReasonMalformed)
	}
	families := make([]InputFamily, 0, len(encodedFamilies))
	for _, encodedFamily := range encodedFamilies {
		family, err := decodeFamily(encodedFamily)
		if err != nil {
			return ValidatedSnapshot{}, err
		}
		families = append(families, family)
	}
	return Validate(NewCandidate(schemaVersion, families, len(content)), limits)
}

func decodeFamily(content []byte) (InputFamily, error) {
	object, err := decodeExactObject(content, "name", "help", "type", "series")
	if err != nil {
		return InputFamily{}, Reject(ReasonMalformed)
	}
	var family InputFamily
	if json.Unmarshal(object["name"], &family.Name) != nil || json.Unmarshal(object["help"], &family.Help) != nil || json.Unmarshal(object["type"], &family.Type) != nil {
		return InputFamily{}, Reject(ReasonMalformed)
	}
	if family.Type != Counter && family.Type != Gauge && family.Type != Histogram {
		return InputFamily{}, Reject(ReasonPolicy)
	}
	var encodedSeries []json.RawMessage
	if err := json.Unmarshal(object["series"], &encodedSeries); err != nil || encodedSeries == nil {
		return InputFamily{}, Reject(ReasonMalformed)
	}
	family.Series = make([]InputSeries, 0, len(encodedSeries))
	for _, encoded := range encodedSeries {
		series, err := decodeSeries(encoded, family.Type)
		if err != nil {
			return InputFamily{}, err
		}
		family.Series = append(family.Series, series)
	}
	return family, nil
}

func decodeSeries(content []byte, metricType MetricType) (InputSeries, error) {
	required := []string{"labels", "value"}
	if metricType == Histogram {
		required = []string{"labels", "histogram"}
	}
	object, err := decodeExactObject(content, required...)
	if err != nil {
		return InputSeries{}, Reject(ReasonMalformed)
	}
	labels, err := decodeLabels(object["labels"])
	if err != nil {
		return InputSeries{}, err
	}
	series := InputSeries{Labels: labels}
	if metricType != Histogram {
		if err := json.Unmarshal(object["value"], &series.Value); err != nil {
			return InputSeries{}, Reject(ReasonMalformed)
		}
		return series, nil
	}
	histogram, err := decodeHistogram(object["histogram"])
	if err != nil {
		return InputSeries{}, err
	}
	series.Histogram = &histogram
	return series, nil
}

func decodeLabels(content []byte) ([]InputLabel, error) {
	var labels map[string]string
	if err := json.Unmarshal(content, &labels); err != nil || labels == nil {
		return nil, Reject(ReasonMalformed)
	}
	result := make([]InputLabel, 0, len(labels))
	for name, value := range labels {
		result = append(result, InputLabel{Name: name, Value: value})
	}
	return result, nil
}

func decodeHistogram(content []byte) (InputHistogram, error) {
	object, err := decodeExactObject(content, "count", "sum", "buckets")
	if err != nil {
		return InputHistogram{}, Reject(ReasonMalformed)
	}
	var histogram InputHistogram
	if json.Unmarshal(object["count"], &histogram.Count) != nil || json.Unmarshal(object["sum"], &histogram.Sum) != nil {
		return InputHistogram{}, Reject(ReasonMalformed)
	}
	var encodedBuckets []json.RawMessage
	if err := json.Unmarshal(object["buckets"], &encodedBuckets); err != nil || encodedBuckets == nil {
		return InputHistogram{}, Reject(ReasonMalformed)
	}
	histogram.Buckets = make([]InputBucket, 0, len(encodedBuckets))
	for _, encoded := range encodedBuckets {
		object, err := decodeExactObject(encoded, "le", "count")
		if err != nil {
			return InputHistogram{}, Reject(ReasonMalformed)
		}
		var bucket InputBucket
		if json.Unmarshal(object["le"], &bucket.UpperBound) != nil || json.Unmarshal(object["count"], &bucket.Count) != nil {
			return InputHistogram{}, Reject(ReasonMalformed)
		}
		histogram.Buckets = append(histogram.Buckets, bucket)
	}
	return histogram, nil
}

func decodeExactObject(content []byte, required ...string) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(content, &object); err != nil || object == nil || len(object) != len(required) {
		return nil, errors.New("object shape")
	}
	for _, name := range required {
		if _, exists := object[name]; !exists {
			return nil, errors.New("object field")
		}
	}
	return object, nil
}

type duplicateKeyError struct{ labels bool }

func (duplicate duplicateKeyError) Error() string { return "duplicate JSON member" }

func validateJSONStructure(content []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	if err := scanJSONValue(decoder, ""); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder, parentKey string) error {
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
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("non-string object key")
			}
			if _, exists := seen[key]; exists {
				return duplicateKeyError{labels: parentKey == "labels"}
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder, key); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder, parentKey); err != nil {
				return err
			}
		}
	default:
		return errors.New("unexpected delimiter")
	}
	closing, err := decoder.Token()
	if err != nil {
		return err
	}
	if closeDelimiter, ok := closing.(json.Delim); !ok || (delimiter == '{' && closeDelimiter != '}') || (delimiter == '[' && closeDelimiter != ']') {
		return errors.New("mismatched delimiter")
	}
	return nil
}
