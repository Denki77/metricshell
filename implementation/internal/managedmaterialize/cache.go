package managedmaterialize

import (
	"encoding/json"
	"errors"
	"math"
	"sort"
	"strconv"
	"sync"

	"github.com/Denki77/metricshell/implementation/internal/managed"
)

type Source interface {
	Read() managed.Snapshot
}

type Encoder func(managed.Snapshot) ([]byte, error)

type Representation struct {
	Generation uint64
	body       []byte
}

func (representation Representation) Bytes() []byte {
	return append([]byte(nil), representation.body...)
}

type Result struct {
	Representation Representation
	Hit            bool
}

type Cache struct {
	mu     sync.Mutex
	source Source
	encode Encoder
	cached *Representation
}

func New(source Source, encoder Encoder) (*Cache, error) {
	if source == nil {
		return nil, errors.New("managed materialization source is required")
	}
	if encoder == nil {
		encoder = Encode
	}
	return &Cache{source: source, encode: encoder}, nil
}

func (cache *Cache) Materialize() (Result, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	snapshot := cache.source.Read()
	if cache.cached != nil && cache.cached.Generation == snapshot.Generation {
		return Result{Representation: *cache.cached, Hit: true}, nil
	}
	body, err := cache.encode(snapshot)
	if err != nil {
		return Result{}, err
	}
	representation := Representation{Generation: snapshot.Generation, body: append([]byte(nil), body...)}
	cache.cached = &representation
	return Result{Representation: representation}, nil
}

func (cache *Cache) Cached() (Representation, bool) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.cached == nil {
		return Representation{}, false
	}
	return *cache.cached, true
}

type document struct {
	SchemaVersion int      `json:"schema_version"`
	Families      []family `json:"families"`
}

type family struct {
	Name   string   `json:"name"`
	Help   string   `json:"help"`
	Type   string   `json:"type"`
	Series []series `json:"series"`
}

type series struct {
	Labels    map[string]string `json:"labels"`
	Value     string            `json:"value,omitempty"`
	Histogram *histogram        `json:"histogram,omitempty"`
}

type histogram struct {
	Count   string   `json:"count"`
	Sum     string   `json:"sum"`
	Buckets []bucket `json:"buckets"`
}

type bucket struct {
	UpperBound string `json:"le"`
	Count      string `json:"count"`
}

func Encode(snapshot managed.Snapshot) ([]byte, error) {
	names := make([]string, 0, len(snapshot.Families))
	for name := range snapshot.Families {
		names = append(names, name)
	}
	sort.Strings(names)
	output := document{SchemaVersion: 1, Families: make([]family, 0, len(names))}
	for _, name := range names {
		input := snapshot.Families[name]
		encoded := family{Name: input.Descriptor.Name, Help: input.Descriptor.Help, Type: string(input.Descriptor.Type), Series: []series{}}
		identities := make([]string, 0, len(input.Series))
		for identity := range input.Series {
			identities = append(identities, identity)
		}
		sort.Strings(identities)
		for _, identity := range identities {
			value := input.Series[identity]
			item := series{Labels: cloneLabels(value.Labels)}
			if input.Descriptor.Type != managed.Histogram {
				item.Value = formatFloat(value.Value)
			} else {
				if len(value.BucketCounts) != len(input.Descriptor.Buckets) {
					return nil, errors.New("managed histogram shape mismatch")
				}
				item.Histogram = &histogram{Count: strconv.FormatUint(value.Count, 10), Sum: formatFloat(value.Sum), Buckets: make([]bucket, len(value.BucketCounts))}
				for index, count := range value.BucketCounts {
					item.Histogram.Buckets[index] = bucket{UpperBound: formatFloat(input.Descriptor.Buckets[index]), Count: strconv.FormatUint(count, 10)}
				}
			}
			encoded.Series = append(encoded.Series, item)
		}
		output.Families = append(output.Families, encoded)
	}
	return json.Marshal(output)
}

func formatFloat(value float64) string {
	if math.IsInf(value, 1) {
		return "+Inf"
	}
	if math.IsInf(value, -1) {
		return "-Inf"
	}
	if math.IsNaN(value) {
		return "NaN"
	}
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func cloneLabels(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for name, value := range input {
		result[name] = value
	}
	return result
}
