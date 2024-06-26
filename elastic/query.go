package elastic

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	common_utils "github.com/kholiqdev/go-common/utils"
	"github.com/pkg/errors"
)

var (
	ErrMultiMatchSearchPrefix = errors.New("MultiMatchSearchPrefix response error")
)

type MultiMatch struct {
	Fields []string `json:"fields"`
	Query  string   `json:"query"`
	Type   string   `json:"type"`
}

type MultiMatchQuery struct {
	MultiMatch MultiMatch `json:"multi_match"`
}

type MultiMatchSearchQuery struct {
	Query MultiMatchQuery `json:"query"`
	Sort  []any           `json:"sort"`
}

type Bool struct {
	Must []any `json:"must"`
}

func SearchMultiMatchPrefix[T any](ctx context.Context, transport esapi.Transport, request SearchMatchPrefixRequest) (*SearchListResponse[T], error) {
	searchQuery := make(map[string]any, 10)
	matchPrefix := make(map[string]any, 10)
	for _, field := range request.Fields {
		matchPrefix[field] = request.Term
	}

	matchSearchQuery := MultiMatchSearchQuery{
		Sort: []any{"_score", request.SortMap},
		Query: MultiMatchQuery{
			MultiMatch: MultiMatch{
				Fields: request.Fields,
				Query:  request.Term,
				Type:   "phrase_prefix",
			}}}

	if request.SortMap != nil {
		searchQuery["sort"] = []any{"_score", request.SortMap}
	}

	queryBytes, err := common_utils.Marshal(&matchSearchQuery)
	if err != nil {
		return nil, err
	}

	searchRequest := esapi.SearchRequest{
		Index:  request.Index,
		Body:   bytes.NewReader(queryBytes),
		Size:   IntPointer(request.Size),
		From:   IntPointer(request.From),
		Pretty: true,
	}

	if request.Sort != nil {
		searchRequest.Sort = request.Sort
	}

	response, err := searchRequest.Do(ctx, transport)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.IsError() {
		return nil, errors.Wrapf(ErrMultiMatchSearchPrefix, "err: %s", response.String())
	}

	hits := EsHits[T]{}
	err = common_utils.NewDecoder(response.Body).Decode(&hits)
	if err != nil {
		return nil, err
	}

	responseList := make([]T, len(hits.Hits.Hits))
	for i, source := range hits.Hits.Hits {
		responseList[i] = source.Source
	}

	return &SearchListResponse[T]{
		List:  responseList,
		Total: hits.Hits.Total.Value,
	}, nil
}

func SearchMatchPhrasePrefix[T any](ctx context.Context, transport esapi.Transport, request SearchMatchPrefixRequest) (*SearchListResponse[T], error) {
	searchQuery := make(map[string]any, 10)
	matchPrefix := make(map[string]any, 10)
	for _, field := range request.Fields {
		matchPrefix[field] = request.Term
	}

	searchQuery["query"] = map[string]any{
		"bool": map[string]any{
			"must": map[string]any{
				"match_phrase_prefix": matchPrefix,
			}},
	}

	if request.SortMap != nil {
		searchQuery["sort"] = []any{"_score", request.SortMap}
	}

	queryBytes, err := common_utils.Marshal(searchQuery)
	if err != nil {
		return nil, err
	}

	searchRequest := esapi.SearchRequest{
		Index:  request.Index,
		Body:   bytes.NewReader(queryBytes),
		Size:   IntPointer(request.Size),
		From:   IntPointer(request.From),
		Sort:   request.Sort,
		Pretty: true,
	}

	response, err := searchRequest.Do(ctx, transport)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	hits := EsHits[T]{}
	err = json.NewDecoder(response.Body).Decode(&hits)
	if err != nil {
		return nil, err
	}

	responseList := make([]T, len(hits.Hits.Hits))
	for i, source := range hits.Hits.Hits {
		responseList[i] = source.Source
	}

	return &SearchListResponse[T]{
		List:  responseList,
		Total: hits.Hits.Total.Value,
	}, nil
}
