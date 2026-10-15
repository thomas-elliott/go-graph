package embed

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestEmbed(t *testing.T) {
	var sent request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer key" {
			t.Errorf("Authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
			t.Error(err)
		}
		w.Write([]byte(`{"data":[{"index":1,"embedding":[3,4]},{"index":0,"embedding":[1,2]}]}`))
	}))
	defer server.Close()

	c := &Client{URL: server.URL, Model: "m", Dims: 2, APIKey: "key"}
	got, err := c.Embed(t.Context(), []string{"first", "second"})
	if err != nil {
		t.Fatal(err)
	}
	if want := [][]float32{{1, 2}, {3, 4}}; !reflect.DeepEqual(got, want) {
		t.Errorf("Embed = %v, want %v", got, want)
	}
	if sent.Model != "m" || sent.Dimensions != 2 {
		t.Errorf("sent model %q with %d dimensions, want m with 2", sent.Model, sent.Dimensions)
	}

	c.Dims = 3
	if _, err := c.Embed(t.Context(), []string{"first", "second"}); err == nil {
		t.Error("wrong size: want an error")
	}
}
