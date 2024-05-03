package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/redis/go-redis/v9"
)

type Response struct {
	FoundIn string `json:"found_in"`
	Data    string `json:"data"`
	PodIP   string `json:"pod_ip"`
	PodName string `json:"pod_name"`
}

type ElasticHits struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
	} `json:"hits"`
}

type RapidHits struct {
	TitleResults struct {
		Results []json.RawMessage `json:"results"`
	} `json:"titleResults"`
}

type SearchHandler struct {
	rdsClient *redis.Client
	esClient  *elasticsearch.Client
}

func New(rdsClient *redis.Client, esClient *elasticsearch.Client) *SearchHandler {
	return &SearchHandler{
		rdsClient: rdsClient,
		esClient:  esClient,
	}
}

func (sh *SearchHandler) SearchMovie(w http.ResponseWriter, r *http.Request) {
	qValues, err := url.ParseQuery(r.URL.RawQuery)
	movieName := qValues.Get("query") // user should send a query parameter with name "query"
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	podName := os.Getenv("MY_POD_NAME")
	podIP := os.Getenv("MY_POD_IP")

	ctx, cancelFunc := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelFunc()

	// searching in redis
	data, err := sh.rdsClient.Get(ctx, movieName).Result()
	if err == nil {
		resp := Response{
			FoundIn: "Redis",
			Data:    data,
			PodIP:   podIP,
			PodName: podName,
		}
		if err = sh.sendJsonResp(resp, w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	if err != redis.Nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// searching in elastic
	esQuery := fmt.Sprintf(`{
		"query": {
			"match": {
				"Series_Title": "%s"
			}
		}
	}`, movieName)

	esRes, err := sh.esClient.Search(
		sh.esClient.Search.WithContext(context.Background()),
		sh.esClient.Search.WithIndex("movies"),
		sh.esClient.Search.WithBody(strings.NewReader(esQuery)),
		sh.esClient.Search.WithTrackTotalHits(true),
		sh.esClient.Search.WithPretty(),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer esRes.Body.Close()

	var elHits ElasticHits // to know how many results are retrieved
	esResBody, err := io.ReadAll(esRes.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.Unmarshal(esResBody, &elHits); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// found at least one result in the elastic
	if elHits.Hits.Total.Value > 0 {

		sh.rdsClient.Set(ctx, movieName, string(esResBody), 0)
		resp := Response{
			FoundIn: "Elastic Search Index",
			Data:    string(esResBody),
			PodIP:   podIP,
			PodName: podName,
		}
		if err = sh.sendJsonResp(resp, w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	// searching by imdb api
	rapidHits, err := sh.reqToRapidAPI(movieName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(rapidHits.TitleResults.Results) == 0 {
		http.Error(w, "Movie not found", http.StatusNotFound)
		return
	}

	jsonRapidHits, err := json.MarshalIndent(rapidHits, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sh.rdsClient.Set(ctx, movieName, string(jsonRapidHits), 0)
	resp := Response{
		FoundIn: "IMDB Rapid API",
		Data:    string(jsonRapidHits),
		PodIP:   podIP,
		PodName: podName,
	}
	if err = sh.sendJsonResp(resp, w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}

func (sh *SearchHandler) sendJsonResp(resp Response, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(resp)
}

func (sh *SearchHandler) reqToRapidAPI(query string) (RapidHits, error) {
	url := fmt.Sprintf("https://imdb146.p.rapidapi.com/v1/find/?query=%s", query)
	req, _ := http.NewRequest("GET", url, nil)

	req.Header.Add("X-RapidAPI-Key", "3acee9ab9amshc1a9e394e2bcbccp1b469fjsn92fa96f0bc42")
	req.Header.Add("X-RapidAPI-Host", "imdb146.p.rapidapi.com")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return RapidHits{}, err
	}

	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)

	var hits RapidHits
	if err := json.Unmarshal(body, &hits); err != nil {
		return hits, err
	}

	return hits, nil
}
