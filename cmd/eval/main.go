// Run evals against the different graph types
package main

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"flag"
	"fmt"
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
	question question
	recall   []float64
	firstHit int
	took     time.Duration
}

// method ranks node IDs for a question, best first.
type method func(ctx context.Context, query string) ([]string, error)

func main() {
	path := flag.String("questions", "eval/questions.jsonl", "questions file")
	name := flag.String("method", "text", "retrieval method: text, vector or hybrid")
	limit := flag.Int("limit", 200, "hits to fetch per question")
	show := flag.String("show", "", "list one question's ranked hits instead of scoring, e.g. nz-039")
	flag.Parse()

	if err := run(context.Background(), *path, *name, *limit, *show); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, path, name string, limit int, show string) error {
	questions, err := readQuestions(path)
	if err != nil {
		return err
	}

	store, err := pggraph.Open(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer store.Close()

	search, err := pickMethod(store, name, limit)
	if err != nil {
		return err
	}

	tokens := tokenCounter(store)
	if show != "" {
		i := slices.IndexFunc(questions, func(q question) bool { return q.ID == show })
		if i < 0 {
			return fmt.Errorf("no question %q", show)
		}
		return showHits(ctx, questions[i], search, tokens)
	}

	var results []result
	for _, q := range questions {
		start := time.Now()
		ranked, err := search(ctx, q.Question)
		took := time.Since(start)
		if err != nil {
			return fmt.Errorf("%s: %w", q.ID, err)
		}

		r, err := score(ctx, q, ranked, tokens)
		if err != nil {
			return fmt.Errorf("%s: %w", q.ID, err)
		}
		r.took = took
		results = append(results, r)
		printQuestion(r)
	}

	fmt.Println()
	printSummary(name, results)
	return nil
}

func pickMethod(store *pggraph.Store, name string, limit int) (method, error) {
	text := func(ctx context.Context, query string) ([]string, error) {
		hits, err := store.SearchAnyWord(ctx, query, limit)
		return hitIDs(hits), err
	}
	if name == "text" {
		return text, nil
	}

	client, err := embed.FromEnv()
	if err != nil {
		return nil, err
	}
	vector := func(ctx context.Context, query string) ([]string, error) {
		vectors, err := client.Embed(ctx, []string{query})
		if err != nil {
			return nil, err
		}
		hits, err := store.VectorSearch(ctx, client.Model, vectors[0], limit)
		return hitIDs(hits), err
	}

	switch name {
	case "vector":
		return vector, nil
	case "hybrid":
		return func(ctx context.Context, query string) ([]string, error) {
			textIDs, err := text(ctx, query)
			if err != nil {
				return nil, err
			}
			vectorIDs, err := vector(ctx, query)
			if err != nil {
				return nil, err
			}
			fused := fuse(textIDs, vectorIDs)
			return fused[:min(limit, len(fused))], nil
		}, nil
	}
	return nil, fmt.Errorf("unknown method %q", name)
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
func showHits(ctx context.Context, q question, search method, tokens func(context.Context, string) (int, error)) error {
	ranked, err := search(ctx, q.Question)
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
	fmt.Printf("%-8s %-10s recall %s  first hit %4s  %6s\n",
		r.question.ID, r.question.Type, recallCells(r.recall), first, r.took.Round(time.Millisecond))
}

func printSummary(name string, results []result) {
	fmt.Printf("method: %s   budgets: %v tokens\n", name, budgets)
	fmt.Printf("%-10s %3s  %-17s  %-9s  %-9s  %s\n", "type", "n", "recall", "found", "first hit", "search time")

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
	var times []time.Duration
	for _, r := range rs {
		for b := range budgets {
			recall[b] += r.recall[b] / float64(len(rs))
		}
		if r.firstHit > 0 {
			firstHits = append(firstHits, r.firstHit)
		}
		times = append(times, r.took)
	}

	first := "-"
	if len(firstHits) > 0 {
		first = fmt.Sprint(median(firstHits))
	}
	fmt.Printf("%-10s %3d  %s  %3d of %-3d  %9s  %s\n",
		name, len(rs), recallCells(recall), len(firstHits), len(rs), first, median(times).Round(time.Millisecond))
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

func median[T int | time.Duration](xs []T) T {
	sorted := slices.Clone(xs)
	slices.Sort(sorted)
	return sorted[len(sorted)/2]
}
