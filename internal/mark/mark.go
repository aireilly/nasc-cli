// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Aidan Reilly and the nasc contributors

package mark

import (
	"sort"
	"sync"

	"github.com/aireilly/nasc-cli/internal/model"
	"github.com/aireilly/nasc-cli/internal/schema"
)

// Result is the set of new field values for one document, keyed by the
// frontmatter key each value is written under.
type Result struct {
	Path    string
	Updates map[string]model.Value
}

// Failure records a source that could not produce a value for a document.
type Failure struct {
	Path   string
	Source string
	Err    error
}

// Skip records a field nasc chose not to write, and why.
type Skip struct {
	Path   string
	Key    string
	Reason string
}

// Options configures one mark run.
type Options struct {
	Root       string
	Sources    []string
	Force      bool
	Jobs       int
	OwnerBlame bool
	LLM        LLMConfig
	// Protected, when set, returns frontmatter keys nasc must not write for a
	// document path, mapped to a reason (a site generator owns the key). A
	// field whose schema entry names its key explicitly is written anyway.
	Protected func(path string) map[string]string
}

// Report is everything a mark run found.
type Report struct {
	Results  []Result
	Failures []Failure
	Skipped  []Skip
	// Unmapped counts documents per directory that a parent-dir field could
	// not place, because the directory name is not in the field's enum and no
	// map entry covers it.
	Unmapped map[string]int
}

func cloneFields(m map[string]model.Value) map[string]model.Value {
	out := make(map[string]model.Value, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

type docOutcome struct {
	result   *Result
	failures []Failure
	skipped  []Skip
	unmapped string
}

// Plan runs the requested sources over docs and reports one Result per doc
// that gained or refreshed a field. Sources run in order for each doc: they
// fill absent keys, the git source refreshes lastUpdated, and force overwrites
// present keys. Later sources see earlier sources' additions. Documents are
// processed concurrently, opts.Jobs at a time; the report keeps input order.
func Plan(docs []model.Doc, s *schema.Schema, opts Options) Report {
	outcomes := make([]docOutcome, len(docs))
	jobs := opts.Jobs
	if jobs < 1 {
		jobs = 1
	}
	work := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < jobs; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				outcomes[i] = planOne(docs[i], s, opts)
			}
		}()
	}
	for i := range docs {
		work <- i
	}
	close(work)
	wg.Wait()

	rep := Report{Unmapped: map[string]int{}}
	for _, o := range outcomes {
		if o.result != nil {
			rep.Results = append(rep.Results, *o.result)
		}
		rep.Failures = append(rep.Failures, o.failures...)
		rep.Skipped = append(rep.Skipped, o.skipped...)
		if o.unmapped != "" {
			rep.Unmapped[o.unmapped]++
		}
	}
	return rep
}

func planOne(d model.Doc, s *schema.Schema, opts Options) docOutcome {
	var out docOutcome
	var protected map[string]string
	if opts.Protected != nil {
		protected = opts.Protected(d.Path)
	}
	merged := map[string]model.Value{}
	view := d
	view.Fields = cloneFields(d.Fields)
	for _, source := range opts.Sources {
		var up map[string]model.Value
		var err error
		switch source {
		case "file":
			up, out.unmapped = FileSource(view, s, opts.Force)
		case "git":
			up = GitSource(view, s, opts.Root, opts.Force, opts.OwnerBlame)
		case "llm":
			up, err = LLMSource(view, s, opts.LLM, opts.Force)
		}
		if err != nil {
			out.failures = append(out.failures, Failure{Path: d.Path, Source: source, Err: err})
		}
		for _, name := range sortedNames(up) {
			v := up[name]
			key := s.Key(name)
			if reason, ok := protected[key]; ok && !explicitKey(s, name) {
				out.skipped = append(out.skipped, Skip{Path: d.Path, Key: key, Reason: reason})
				continue
			}
			// Skip refreshes that would not change the value, so a re-run
			// over an unchanged corpus stays a no-op (idempotent).
			if cur, ok := d.Field(key); ok && equalValue(cur, v) {
				continue
			}
			merged[key] = v
			view.Fields[key] = v // so the next source sees it as present
		}
	}
	if len(merged) > 0 {
		out.result = &Result{Path: d.Path, Updates: merged}
	}
	return out
}

func explicitKey(s *schema.Schema, name string) bool {
	if s == nil {
		return false
	}
	f, ok := s.Fields[name]
	return ok && f.Key != ""
}

func sortedNames(m map[string]model.Value) []string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// equalValue reports whether two values are the same for refresh purposes.
// Kind plus canonical string form covers strings, dates, and lists.
func equalValue(a, b model.Value) bool {
	return a.Kind == b.Kind && a.String() == b.String()
}
