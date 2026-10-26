// Run evals against the different graph types
package main

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/thomas-elliott/go-graph/embed"
	"github.com/thomas-elliott/go-graph/pggraph"
)

var budgets = []int{1000, 2000, 4000}

type question struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Question string   `json:"question"`
	Relevant []string `json:"relevant"`
}

// result is one question's score for one method.
type result struct {
	question  question
	recall    []float64
	firstHit  int
	took      time.Duration
	embedTook time.Duration
}

// query is a question's text, with its embedding when the method needs one.
type query struct {
	text   string
	vector []float32
}

// method ranks node IDs for a question, best first.
type method func(ctx context.Context, q query) ([]string, error)

// embedder turns a question into a vector.
type embedder func(ctx context.Context, text string) ([]float32, error)

func main() {
	path := flag.String("questions", "eval/questions.jsonl", "questions file")
	name := flag.String("method", "text", "retrieval method: text, vector, hybrid or walk")
	var walk walkConfig
	flag.IntVar(&walk.seeds, "seeds", 10, "walk: vector hits the walk starts from")
	flag.Float64Var(&walk.minSimilarity, "min-similarity", 0, "walk: only walk from seeds at least this similar")
	flag.BoolVar(&walk.links, "links", false, "walk: also follow link edges from each seed's destination")
	flag.BoolVar(&walk.fuse, "fuse", false, "walk: fuse the walk's ranking with plain vector search")
	flag.StringVar(&walk.rank, "rank", "similarity", "walk: rank candidates by similarity, decay or pagerank")
	limit := flag.Int("limit", 200, "hits to fetch per question")
	exact := flag.Bool("exact", false, "vector search every row instead of using the index")
	show := flag.String("show", "", "list one question's ranked hits instead of scoring, e.g. nz-039")
	flag.Parse()

	if err := run(context.Background(), *path, *name, *limit, *exact, walk, *show); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, path, name string, limit int, exact bool, walk walkConfig, show string) error {
	questions, err := readQuestions(path)
	if err != nil {
		return err
	}

	store, err := pggraph.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer store.Close()

	search, embedQuery, err := pickMethod(store, name, limit, exact, walk)
	if err != nil {
		return err
	}

	// ask embeds the question before searching, so search time covers only the method's own work.
	ask := func(text string) (query, time.Duration, error) {
		if embedQuery == nil {
			return query{text: text}, 0, nil
		}
		start := time.Now()
		v, err := embedQuery(ctx, text)
		return query{text: text, vector: v}, time.Since(start), err
	}

	tokens := tokenCounter(store)
	if show != "" {
		i := slices.IndexFunc(questions, func(q question) bool { return q.ID == show })
		if i < 0 {
			return fmt.Errorf("no question %q", show)
		}
		in, _, err := ask(questions[i].Question)
		if err != nil {
			return err
		}
		return showHits(ctx, questions[i], in, search, tokens)
	}

	// Warm-up pass, so timings measure a warm cache rather than whatever Docker last evicted.
	for _, q := range questions {
		in, _, err := ask(q.Question)
		if err != nil {
			return fmt.Errorf("%s: %w", q.ID, err)
		}
		if _, err := search(ctx, in); err != nil {
			return fmt.Errorf("%s: %w", q.ID, err)
		}
	}

	var results []result
	for _, q := range questions {
		in, embedTook, err := ask(q.Question)
		if err != nil {
			return fmt.Errorf("%s: %w", q.ID, err)
		}
		start := time.Now()
		ranked, err := search(ctx, in)
		took := time.Since(start)
		if err != nil {
			return fmt.Errorf("%s: %w", q.ID, err)
		}

		r, err := score(ctx, q, ranked, tokens)
		if err != nil {
			return fmt.Errorf("%s: %w", q.ID, err)
		}
		r.took = took
		r.embedTook = embedTook
		results = append(results, r)
		printQuestion(r)
	}

	fmt.Println()
	printSummary(name, results)
	return nil
}

func pickMethod(store *pggraph.Store, name string, limit int, exact bool, walk walkConfig) (method, embedder, error) {
	text := func(ctx context.Context, q query) ([]string, error) {
		hits, err := store.SearchAnyWord(ctx, q.text, limit)
		return hitIDs(hits), err
	}
	if name == "text" {
		return text, nil, nil
	}

	client, err := embed.FromEnv()
	if err != nil {
		return nil, nil, err
	}
	embedQuery := func(ctx context.Context, text string) ([]float32, error) {
		vectors, err := client.Embed(ctx, []string{text})
		if err != nil {
			return nil, err
		}
		return vectors[0], nil
	}
	vectorSearch := store.VectorSearch
	if exact {
		vectorSearch = store.ExactVectorSearch
	}
	vector := func(ctx context.Context, q query) ([]string, error) {
		hits, err := vectorSearch(ctx, client.Model, q.vector, limit)
		return hitIDs(hits), err
	}

	switch name {
	case "vector":
		return vector, embedQuery, nil
	case "hybrid":
		return func(ctx context.Context, q query) ([]string, error) {
			textIDs, err := text(ctx, q)
			if err != nil {
				return nil, err
			}
			vectorIDs, err := vector(ctx, q)
			if err != nil {
				return nil, err
			}
			fused := fuse(textIDs, vectorIDs)
			return fused[:min(limit, len(fused))], nil
		}, embedQuery, nil
	case "walk":
		search, err := walkMethod(store, client.Model, vectorSearch, walk, limit)
		return search, embedQuery, err
	}
	return nil, nil, fmt.Errorf("unknown method %q", name)
}

// fuse merges rankings with reciprocal rank fusion.
func fuse(rankings ...[]string) []string {
	const k = 60
	scores := map[string]float64{}
	for _, ranking := range rankings {
		for i, id := range ranking {
			scores[id] += 1.0 / float64(k+i+1)
		}
	}

	ids := make([]string, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b string) int {
		if c := cmp.Compare(scores[b], scores[a]); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	return ids
}

func hitIDs(hits []pggraph.Hit) []string {
	ids := make([]string, len(hits))
	for i, h := range hits {
		ids[i] = h.ID
	}
	return ids
}

// tokenCounter is a rough characters / 4 estimate, not using an actual tokeniser
func tokenCounter(store *pggraph.Store) func(ctx context.Context, id string) (int, error) {
	cache := map[string]int{}
	return func(ctx context.Context, id string) (int, error) {
		if n, ok := cache[id]; ok {
			return n, nil
		}
		node, found, err := store.Node(ctx, id)
		if err != nil || !found {
			return 0, err
		}
		n := (utf8.RuneCountInString(embed.Text(node)) + 3) / 4
		cache[id] = n
		return n, nil
	}
}

func score(ctx context.Context, q question, ranked []string, tokens func(context.Context, string) (int, error)) (result, error) {
	r := result{question: q, recall: make([]float64, len(budgets))}

	for i, id := range ranked {
		if slices.Contains(q.Relevant, id) {
			r.firstHit = i + 1
			break
		}
	}

	for b, budget := range budgets {
		used, found := 0, 0
		for _, id := range ranked {
			n, err := tokens(ctx, id)
			if err != nil {
				return r, err
			}
			if used >= budget {
				break
			}
			used += n
			if slices.Contains(q.Relevant, id) {
				found++
			}
		}
		r.recall[b] = float64(found) / float64(len(q.Relevant))
	}
	return r, nil
}

// showHits prints the results and ranking from a question, for debugging
func showHits(ctx context.Context, q question, in query, search method, tokens func(context.Context, string) (int, error)) error {
	ranked, err := search(ctx, in)
	if err != nil {
		return err
	}
	fmt.Printf("%s (%s): %s\n", q.ID, q.Type, q.Question)
	fmt.Printf("labelled: %v\n\n", q.Relevant)

	used, b := 0, 0
	for i, id := range ranked {
		for b < len(budgets) && used >= budgets[b] {
			fmt.Printf("     ---- %d token budget\n", budgets[b])
			b++
		}
		if b == len(budgets) {
			break
		}
		n, err := tokens(ctx, id)
		if err != nil {
			return err
		}
		mark := " "
		if slices.Contains(q.Relevant, id) {
			mark = "*"
		}
		fmt.Printf("%4d %s %5d  %s\n", i+1, mark, n, id)
		used += n
	}
	return nil
}

func readQuestions(path string) ([]question, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var questions []question
	lines := bufio.NewScanner(f)
	for lines.Scan() {
		if len(lines.Bytes()) == 0 {
			continue
		}
		var q question
		if err := json.Unmarshal(lines.Bytes(), &q); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		questions = append(questions, q)
	}
	return questions, lines.Err()
}

func printQuestion(r result) {
	first := "-"
	if r.firstHit > 0 {
		first = fmt.Sprint(r.firstHit)
	}
	fmt.Printf("%-8s %-10s recall %s  first hit %4s  search %6s  embed %6s\n",
		r.question.ID, r.question.Type, recallCells(r.recall), first, r.took.Round(time.Millisecond), r.embedTook.Round(time.Millisecond))
}

func printSummary(name string, results []result) {
	fmt.Printf("method: %s   budgets: %v tokens\n", name, budgets)
	fmt.Printf("%-10s %3s  %-17s  %-9s  %-9s  %s\n", "type", "n", "recall", "found", "first hit", "search p50 / p95   embed p50")

	groups := map[string][]result{}
	for _, r := range results {
		groups[r.question.Type] = append(groups[r.question.Type], r)
	}
	for _, typ := range []string{"single", "connected", "broad"} {
		if rs := groups[typ]; len(rs) > 0 {
			printGroup(typ, rs)
		}
	}
	printGroup("all", results)
}

func printGroup(name string, rs []result) {
	recall := make([]float64, len(budgets))
	var firstHits []int
	var times, embedTimes []time.Duration
	for _, r := range rs {
		for b := range budgets {
			recall[b] += r.recall[b] / float64(len(rs))
		}
		if r.firstHit > 0 {
			firstHits = append(firstHits, r.firstHit)
		}
		times = append(times, r.took)
		embedTimes = append(embedTimes, r.embedTook)
	}

	first := "-"
	if len(firstHits) > 0 {
		first = fmt.Sprint(percentile(firstHits, 0.5))
	}
	fmt.Printf("%-10s %3d  %s  %3d of %-3d  %9s  %s / %s   %s\n",
		name, len(rs), recallCells(recall), len(firstHits), len(rs), first,
		percentile(times, 0.5).Round(time.Millisecond), percentile(times, 0.95).Round(time.Millisecond),
		percentile(embedTimes, 0.5).Round(time.Millisecond))
}

func recallCells(recall []float64) string {
	s := ""
	for i, v := range recall {
		if i > 0 {
			s += " "
		}
		s += fmt.Sprintf("%4.0f%%", v*100)
	}
	return s
}

// percentile uses the nearest-rank method: the smallest value with at least p of xs at or below it.
func percentile[T int | time.Duration](xs []T, p float64) T {
	sorted := slices.Clone(xs)
	slices.Sort(sorted)
	return sorted[max(int(math.Ceil(p*float64(len(sorted))))-1, 0)]
}
