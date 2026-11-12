package esx

import (
	"context"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"io"
	"reflect"
	"strings"
	"testing"
)

type ex struct {
	status int
	body   string
	err    error
}

func TestAggregationAndValidation(t *testing.T) {
	s := NewSearcher(ex{status: 200, body: `{"hits":{"total":2,"hits":[]},"aggregations":{"a":{"value":3}}}`})
	r, e := s.Aggregate(context.Background(), AggregationRequest{Index: "i", Filters: []Filter{TermFilter("state", "paid")}, Aggregations: map[string]Aggregation{"a": TermsAggregation("x", 0)}})
	if e != nil || r.Total != 2 || !strings.Contains(string(r.Aggregations), "value") {
		t.Fatal(r, e)
	}
	if _, e = s.Aggregate(context.Background(), AggregationRequest{}); e == nil {
		t.Fatal("empty aggregation")
	}
	if _, e = s.FuzzySearch(context.Background(), FuzzySearchRequest{Keyword: "x"}); e == nil {
		t.Fatal("missing fields")
	}
	if _, e = NewSearcher(nil).Search(context.Background(), SearchRequest{Index: "i", Body: map[string]any{"x": 1}}); e == nil {
		t.Fatal("nil executor")
	}
	if _, e = s.Search(context.Background(), SearchRequest{Body: map[string]any{"x": 1}}); e == nil {
		t.Fatal("missing index")
	}
	if _, e = s.Search(context.Background(), SearchRequest{Index: "i", Body: map[string]any{"x": make(chan int)}}); e == nil {
		t.Fatal("encoding")
	}
	body, e := BuildSearchBody(FuzzySearchRequest{Keyword: "x", Fields: []string{"a"}, Size: 5, Sort: []Sort{SortField("x", "desc")}, Filters: []Filter{TermFilter("x", 1)}, Highlight: HighlightConfig{Fields: []string{"a"}, PreTags: []string{"<b>"}, PostTags: []string{"</b>"}, FragmentSize: 10, NumberOfFragments: 2}})
	if e != nil {
		t.Fatal(e)
	}
	data, _ := io.ReadAll(body)
	if !strings.Contains(string(data), `"fragment_size":10`) {
		t.Fatal(string(data))
	}
	if highlightBody(HighlightConfig{Fields: []string{" "}}) != nil {
		t.Fatal("blank fields")
	}
}

func (e ex) Search(context.Context, []string, io.Reader) (*esapi.Response, error) {
	if e.err != nil {
		return nil, e.err
	}
	return &esapi.Response{StatusCode: e.status, Body: io.NopCloser(strings.NewReader(e.body))}, nil
}
func TestBuildAndSearch(t *testing.T) {
	if _, e := BuildSearchBody(FuzzySearchRequest{}); e == nil {
		t.Fatal()
	}
	b, e := BuildSearchBody(FuzzySearchRequest{Keyword: "x", Fields: []string{"title"}, Highlight: HighlightConfig{Fields: []string{"title", " "}}})
	if e != nil || b == nil {
		t.Fatal()
	}
	if _, e = BuildAggregationBody(AggregationRequest{}); e == nil {
		t.Fatal()
	}
	if _, e = BuildAggregationBody(AggregationRequest{Aggregations: map[string]Aggregation{"a": TermsAggregation("x", 1)}}); e != nil {
		t.Fatal()
	}
	s := NewSearcher(ex{200, `{"hits":{"total":{"value":1},"hits":[{"_id":"1","_index":"i","_score":1,"_source":{"a":1}}]},"aggregations":{}}`, nil})
	r, e := s.FuzzySearch(context.Background(), FuzzySearchRequest{Index: "i", Keyword: "x", Fields: []string{"a"}})
	if e != nil || r.Total != 1 {
		t.Fatal(r, e)
	}
	if _, e = s.Search(context.Background(), SearchRequest{}); e == nil {
		t.Fatal()
	}
	for _, x := range []ex{{500, "bad", nil}, {200, "bad", nil}, {200, "", context.Canceled}} {
		if _, err := NewSearcher(x).Search(context.Background(), SearchRequest{Index: "i", Body: map[string]any{"x": 1}}); err == nil {
			t.Fatal("error response accepted")
		}
	}
}
func TestHelpers(t *testing.T) {
	if SortField("x", "")["x"].(map[string]any)["order"] != "asc" {
		t.Fatal()
	}
	if !reflect.DeepEqual(RangeFilter("x", nil), Filter{"range": map[string]any{"x": map[string]any(nil)}}) {
		t.Fatal("range filter")
	}
	if DateHistogramAggregation("x", "day")["date_histogram"].(map[string]any)["calendar_interval"] != "day" {
		t.Fatal("histogram")
	}
	if RangeAggregation("x", nil)["range"].(map[string]any)["field"] != "x" {
		t.Fatal("range aggregation")
	}
	if totalHits(map[string]any{"value": float64(2)}) != 2 || totalHits("x") != 0 {
		t.Fatal()
	}
	if _, e := NewClient(Config{}); e != nil {
		t.Fatal()
	}
}
