package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func generateHandler(text string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, text)
	}
}

func getData(handler http.HandlerFunc) (string, error) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler(w, r)
	resp := w.Result()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return string(data), nil
}

func TestRouter(t *testing.T) {
	type testCase struct {
		Name                           string
		RequestPath                    string
		RoutingRulePattern             string
		RequestSuccess                 bool
		RuleRegistrationExpectedResult error
		ReturnData                     string
		ExpectedMatches                map[string]string
	}

	cases := []testCase{
		{
			"simple_should_succeed",
			"/test",
			"/test",
			true,
			nil,
			"/test",
			map[string]string{},
		},
		{
			"trailing_slashes_should_succeed",
			"/test////",
			"/test",
			true,
			ErrPathAlreadyRegistered,
			"/test",
			map[string]string{},
		},
		{
			"simple_regexp_should_succeed",
			"/test/4",
			`/test/:id`,
			true,
			nil,
			"/test4",
			map[string]string{"id": "4"},
		},
		{
			"successive_slashes_no_match_should_fail",
			"/test////-",
			"/test",
			true,
			ErrPathAlreadyRegistered,
			"/test4",
			map[string]string{"id": "-"},
		},
		{
			"simple_should_fail",
			"/testl",
			"/test",
			false,
			ErrPathAlreadyRegistered,
			"",
			map[string]string{},
		},
		{
			"regexp_multiple_match_should_succeed",
			"/foo/4/bar/22/lol/amazon",
			`/foo/:first/bar/:second/lol/:third`,
			true,
			nil,
			"",
			map[string]string{"first": "4", "second": "22", "third": "amazon"},
		},
	}
	router := NewRouter()
	for _, route := range cases {
		if !route.RequestSuccess {
			continue
		}
	}
	for _, route := range cases {
		t.Run(route.Name, func(t *testing.T) {
			err := router.Register(http.MethodGet, route.RoutingRulePattern, generateHandler(route.ReturnData))
			if err != nil && err != route.RuleRegistrationExpectedResult {
				t.Error(err)
				t.FailNow()
			}

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, route.RequestPath, nil)
			handler, matches, err := router.determineHandler(r)
			if err != nil && route.RequestSuccess == true {
				t.Error(err)
				t.FailNow()
				return
			}

			if matches != nil {
				success := true
				if len(matches) != len(route.ExpectedMatches) && len(matches) > 0 {
					t.Log("matches count mismatch")
					t.FailNow()
				}

				if len(route.ExpectedMatches) > 0 {
					for k, match := range matches {
						if match != route.ExpectedMatches[k] {
							success = false
							t.Log("matches expected != matches gotten")
							t.Fail()
						}
					}
				}

				if !success {
					t.FailNow()
				}
			}

			if handler == nil && route.RequestSuccess == false {
				t.Log("route failed as it should have")
				return
			}

			if handler == nil {
				t.Log("no handler found")
				t.FailNow()
				return
			}

			handler(w, r)
			resp := w.Result()
			data, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Error(err)
				t.FailNow()
			}

			defer resp.Body.Close()

			if !route.RequestSuccess && resp.StatusCode != http.StatusOK {
				return
			}

			if string(data) != route.ReturnData {
				t.Log("data != path")
				t.Logf("result '%s' != expected '%s'", data, route.ReturnData)
				t.FailNow()
			}
			t.Logf("rule %s success", route.Name)
		})
	}
}

func BenchmarkSimpleRegister(b *testing.B) {
	router := NewRouter()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		route := fmt.Sprintf("/d_%d", i)
		router.Register(http.MethodGet, route, func(w http.ResponseWriter, r *http.Request) {})
		// router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, route, nil))
	}
}

func BenchmarkSimpleRoute(b *testing.B) {
	router := NewRouter()
	for i := 0; i < 100; i++ {
		route := fmt.Sprintf("/d_%d", i)
		router.Register(http.MethodGet, route, func(w http.ResponseWriter, r *http.Request) {})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// route := fmt.Sprintf("/d_%d", i%100)
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/d_"+fmt.Sprint(i%100), nil))
	}
}

func TestSimpleHTTPServer(t *testing.T) {
	type test struct {
		Name               string
		Path               string
		Method             string
		ExpectedStatusCode int
		ExpectedOutput     string
	}

	testCases := []test{
		{
			"simple_test_should_succeed",
			"/test",
			http.MethodGet,
			http.StatusOK,
			"",
		},
		{
			"multiple_matches_should_succceed",
			"/foo/4/bar/22",
			http.MethodGet,
			http.StatusOK,
			`{"Foo":"4","Bar":"22"}`,
		},
		{
			"simple_should_404",
			"/fdskjgsdf",
			http.MethodGet,
			http.StatusNotFound,
			"",
		},
		{
			"simple_post",
			"/test_post",
			http.MethodPost,
			http.StatusCreated,
			"",
		},
		{
			"simple_post_non_existing_405",
			"/test_post_405",
			http.MethodPost,
			http.StatusMethodNotAllowed,
			"",
		},
	}

	type payload struct {
		Foo string
		Bar string
	}

	answer := payload{}
	router := NewRouter()

	router.Register(http.MethodGet, "/test", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	router.Register(http.MethodPost, "/test_post", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) })
	router.Register(http.MethodGet, "/test_post_405", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) })
	router.Register(http.MethodGet, "/foo/:foo/bar/:bar", func(w http.ResponseWriter, r *http.Request) {
		params := r.Context().Value(ParametersKey)
		if value, ok := params.(map[string]string); ok {
			answer.Foo = value["foo"]
			answer.Bar = value["bar"]

			encoder := json.NewEncoder(w)
			encoder.Encode(answer)
		}
	})

	server := httptest.NewServer(router)
	defer server.Close()

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			var response *http.Response
			var err error

			request, err := http.NewRequest(testCase.Method, server.URL+testCase.Path, nil)
			if err != nil {
				t.Error(err)
				t.Fail()
			}

			response, err = http.DefaultClient.Do(request)

			if err != nil {
				t.Error(err)
				t.Fail()
			}

			if response.StatusCode != testCase.ExpectedStatusCode {
				t.Logf("status code %d != expected status code %d", response.StatusCode, testCase.ExpectedStatusCode)
				t.Fail()
			}

			defer response.Body.Close()
			result, err := ioutil.ReadAll(response.Body)
			if err != nil {
				t.Error(err)
				t.Fail()
			}

			body := strings.Trim(string(result), "\n")

			if body != testCase.ExpectedOutput {
				t.Logf("results != expected results")
				t.Log(body)
				t.Log(testCase.ExpectedOutput)
				t.Fail()
			}
		})
	}
}
