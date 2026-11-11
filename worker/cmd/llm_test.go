package main

import (
 "context"
 "io"
 "net/http"
 "strings"
 "testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)
func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func fakeClient(status int, body string, verify func(*testing.T, *http.Request)) *LLMClient {
 t := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
  // verify is called by the individual test's callback.
  if verify != nil { verify(nil, r) }
  return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
 })}
 return &LLMClient{httpClient:t, apiKey:"test-only-key", model:"test-model"}
}

func TestGenerateSuccess(t *testing.T) {
 client := fakeClient(200, `{"choices":[{"message":{"content":"Hello from ATLAS"}}]}`, nil)
 got, err := client.Generate(context.Background(), "Hello")
 if err != nil { t.Fatal(err) }
 if got != "Hello from ATLAS" { t.Fatalf("got %q",got) }
}

func TestGenerateAPIError(t *testing.T) {
 client := fakeClient(429, `{"error":{"message":"rate limit exceeded"}}`, nil)
 _, err := client.Generate(context.Background(), "Hello")
 if err == nil || !strings.Contains(err.Error(), "429") { t.Fatalf("expected HTTP 429 error, got %v",err) }
}

func TestGenerateMalformedJSON(t *testing.T) {
 client := fakeClient(200, `not-json`, nil)
 _, err := client.Generate(context.Background(), "Hello")
 if err == nil || !strings.Contains(err.Error(), "invalid JSON") { t.Fatalf("expected JSON error, got %v",err) }
}

func TestGenerateEmptyChoices(t *testing.T) {
 client := fakeClient(200, `{"choices":[]}`, nil)
 _, err := client.Generate(context.Background(), "Hello")
 if err == nil || !strings.Contains(err.Error(), "empty completion") { t.Fatalf("expected empty completion error, got %v",err) }
}

func TestGenerateRequest(t *testing.T) {
 client := &LLMClient{apiKey:"test-only-key",model:"test-model"}
 client.httpClient = &http.Client{Transport:roundTripFunc(func(r *http.Request)(*http.Response,error){
  if r.Method != http.MethodPost { t.Errorf("method: %s",r.Method) }
  if r.URL.String() != groqEndpoint { t.Errorf("URL: %s",r.URL) }
  if r.Header.Get("Authorization") != "Bearer test-only-key" { t.Error("missing bearer token") }
  if r.Header.Get("Content-Type") != "application/json" { t.Error("missing JSON content type") }
  b,err:=io.ReadAll(r.Body);if err!=nil { t.Fatal(err) }
  for _,s:=range []string{`"model":"test-model"`,`"role":"user"`,`"content":"test prompt"`} {if !strings.Contains(string(b),s){t.Errorf("request missing %s",s)}}
  return &http.Response{StatusCode:200,Body:io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`)),Header:make(http.Header)},nil
 })}
 if _,err:=client.Generate(context.Background(),"test prompt");err!=nil{t.Fatal(err)}
}

func TestNewLLMClientRequiresKey(t *testing.T) {
 t.Setenv("GROQ_API_KEY", "")
 if _,err:=NewLLMClient();err==nil{t.Fatal("expected missing API key error")}
}
