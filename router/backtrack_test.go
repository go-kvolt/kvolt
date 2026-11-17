package router

import (
	"math/rand"
	"strings"
	"testing"
)

// A static route that shares a prefix with a request must not hide the param route:
// with /user/new and /user/:id registered, /user/next is a valid :id.
func TestFind_FallsBackFromStaticToParam(t *testing.T) {
	r := New()
	for _, p := range []string{
		"/user/new", "/user/:id", "/user/:id/posts",
		"/a/:x/c", "/a/b/d",
		"/static/js/app.js", "/static/:kind/main.css",
		"/files/special", "/files/*path",
	} {
		r.AddRoute("GET", p, p)
	}

	cases := []struct {
		path    string
		pattern string // "" means no match
		key     string
		value   string
	}{
		{"/user/new", "/user/new", "", ""},
		{"/user/next", "/user/:id", "id", "next"},
		{"/user/n", "/user/:id", "id", "n"},
		{"/user/newer", "/user/:id", "id", "newer"},
		{"/user/42", "/user/:id", "id", "42"},
		{"/user/new/posts", "/user/:id/posts", "id", "new"},
		{"/user/42/posts", "/user/:id/posts", "id", "42"},
		{"/a/b/c", "/a/:x/c", "x", "b"},
		{"/a/b/d", "/a/b/d", "", ""},
		{"/a/z/c", "/a/:x/c", "x", "z"},
		{"/a/z/d", "", "", ""},
		{"/static/js/app.js", "/static/js/app.js", "", ""},
		{"/static/js/main.css", "/static/:kind/main.css", "kind", "js"},
		{"/files/special", "/files/special", "", ""},
		{"/files/spec", "/files/*path", "path", "spec"},
		{"/files/special/more", "/files/*path", "path", "special/more"},
		{"/user/new/other", "", "", ""},
	}
	for _, tc := range cases {
		res := r.FindEx("GET", tc.path, nil)
		if tc.pattern == "" {
			if res.Found {
				t.Errorf("%s: matched %q, want no match", tc.path, res.Pattern)
			}
			if len(res.Params) != 0 {
				t.Errorf("%s: miss left params %v", tc.path, res.Params)
			}
			continue
		}
		if !res.Found || res.Pattern != tc.pattern || res.Handler != tc.pattern {
			t.Errorf("%s: found=%v pattern=%q, want %q", tc.path, res.Found, res.Pattern, tc.pattern)
			continue
		}
		if tc.key == "" {
			if len(res.Params) != 0 {
				t.Errorf("%s: unexpected params %v", tc.path, res.Params)
			}
		} else if len(res.Params) != 1 || res.Params.Get(tc.key) != tc.value {
			t.Errorf("%s: params = %v, want %s=%s", tc.path, res.Params, tc.key, tc.value)
		}
	}
}

// A failed branch must not leak its params into the result of the branch that matches.
func TestFind_BacktrackDiscardsParamsOfFailedBranch(t *testing.T) {
	r := New()
	r.AddRoute("GET", "/:a/x/:b/end", "deep")
	r.AddRoute("GET", "/:a/*rest", "wide")

	res := r.FindEx("GET", "/1/x/2/nope", make(Params, 0, 8))
	if !res.Found || res.Pattern != "/:a/*rest" {
		t.Fatalf("got found=%v pattern=%q, want /:a/*rest", res.Found, res.Pattern)
	}
	if len(res.Params) != 2 || res.Params.Get("a") != "1" || res.Params.Get("rest") != "x/2/nope" {
		t.Fatalf("params = %v, want a=1 rest=x/2/nope", res.Params)
	}
}

// --- differential test against a straightforward reference matcher ---

type refRoute struct {
	pattern string
	segs    []string
}

// refMatch walks segment by segment, preferring static, then param, then catch-all,
// and backtracks when a choice does not lead to a full match.
func refMatch(routes []refRoute, pathSegs []string) (string, Params, bool) {
	var walk func(cands []refRoute, i int, ps Params) (string, Params, bool)
	walk = func(cands []refRoute, i int, ps Params) (string, Params, bool) {
		if i == len(pathSegs) {
			for _, c := range cands {
				if len(c.segs) == i {
					return c.pattern, ps, true
				}
			}
			return "", nil, false
		}
		seg := pathSegs[i]
		var static, param, catchAll []refRoute
		for _, c := range cands {
			if len(c.segs) <= i {
				continue
			}
			switch s := c.segs[i]; {
			case s[0] == ':':
				param = append(param, c)
			case s[0] == '*':
				catchAll = append(catchAll, c)
			case s == seg:
				static = append(static, c)
			}
		}
		if len(static) > 0 {
			if p, out, ok := walk(static, i+1, ps); ok {
				return p, out, true
			}
		}
		if len(param) > 0 {
			next := append(append(Params{}, ps...), Param{Key: param[0].segs[i][1:], Value: seg})
			if p, out, ok := walk(param, i+1, next); ok {
				return p, out, true
			}
		}
		if len(catchAll) > 0 {
			c := catchAll[0]
			out := append(append(Params{}, ps...), Param{Key: c.segs[i][1:], Value: strings.Join(pathSegs[i:], "/")})
			return c.pattern, out, true
		}
		return "", nil, false
	}
	return walk(routes, 0, nil)
}

func TestFind_MatchesReferenceMatcher(t *testing.T) {
	rng := rand.New(rand.NewSource(20261116))
	words := []string{"a", "ab", "abc", "b", "n", "new", "next", "x"}
	reqWords := append([]string{"zz", "1", "newer"}, words...)

	for set := 0; set < 1500; set++ {
		r := New()
		var routes []refRoute
		seen := map[string]bool{}
		wildAt := map[string]byte{} // prefix -> ':' or '*': only one wildcard kind per position

		for n := 2 + rng.Intn(10); n > 0; n-- {
			depth := 1 + rng.Intn(4)
			segs := make([]string, 0, depth)
			ok := true
			for d := 0; d < depth && ok; d++ {
				prefix := strings.Join(segs, "/")
				var seg string
				switch k := rng.Intn(10); {
				case k < 6:
					seg = words[rng.Intn(len(words))]
				case k < 9 || d != depth-1:
					seg = ":p" + string(rune('0'+d))
				default:
					seg = "*rest"
				}
				if seg[0] == ':' || seg[0] == '*' {
					if kind, exists := wildAt[prefix]; exists && kind != seg[0] {
						ok = false
						break
					}
				}
				segs = append(segs, seg)
			}
			pattern := "/" + strings.Join(segs, "/")
			if !ok || seen[pattern] {
				continue
			}
			seen[pattern] = true
			for d, seg := range segs {
				if seg[0] == ':' || seg[0] == '*' {
					wildAt[strings.Join(segs[:d], "/")] = seg[0]
				}
			}
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						t.Fatalf("set %d: AddRoute(%q) panicked: %v (routes so far: %v)", set, pattern, rec, routes)
					}
				}()
				r.AddRoute("GET", pattern, pattern)
			}()
			routes = append(routes, refRoute{pattern: pattern, segs: segs})
		}

		buf := make(Params, 0, 8)
		for q := 0; q < 60; q++ {
			depth := 1 + rng.Intn(5)
			pathSegs := make([]string, depth)
			for d := range pathSegs {
				pathSegs[d] = reqWords[rng.Intn(len(reqWords))]
			}
			path := "/" + strings.Join(pathSegs, "/")

			wantPattern, wantParams, wantOK := refMatch(routes, pathSegs)
			res := r.FindEx("GET", path, buf)

			if res.Found != wantOK || res.Pattern != wantPattern {
				t.Fatalf("set %d: %s -> found=%v pattern=%q, want found=%v pattern=%q\nroutes: %v",
					set, path, res.Found, res.Pattern, wantOK, wantPattern, routes)
			}
			if wantOK && res.Handler != wantPattern {
				t.Fatalf("set %d: %s -> handler %v, want %q", set, path, res.Handler, wantPattern)
			}
			if len(res.Params) != len(wantParams) {
				t.Fatalf("set %d: %s -> params %v, want %v\nroutes: %v", set, path, res.Params, wantParams, routes)
			}
			for i := range wantParams {
				if res.Params[i] != wantParams[i] {
					t.Fatalf("set %d: %s -> params %v, want %v\nroutes: %v", set, path, res.Params, wantParams, routes)
				}
			}
		}
	}
}
