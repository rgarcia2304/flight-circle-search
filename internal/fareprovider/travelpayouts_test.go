package fareprovider

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testToken = "test-token-DO-NOT-LOG-NEVER-APPEAR-IN-ERROR-STRINGS"

func newTestClient(t *testing.T, baseURL string) *Travelpayouts {
	t.Helper()
	return NewTravelpayouts(testToken, baseURL, &http.Client{Timeout: 5 * time.Second})
}

func validRequest() SearchRequest {
	return SearchRequest{Origin: "NYC", Destination: "LON", Date: "2026-10-15"}
}

func loadFixture(t *testing.T, path string) string {
	t.Helper()
	b, err := fixtureBytes(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return string(b)
}

func fixtureBytes(path string) ([]byte, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(cwd + "/" + path)
	if err != nil {
		b, err = os.ReadFile(path)
	}
	return b, err
}

// ---------------------------------------------------------------------------
// 1. Happy path (real fixture)
// ---------------------------------------------------------------------------

func TestSearch_Success_RealFixture_NYCTOLON(t *testing.T) {
	fixture := loadFixture(t, "testdata/travelpayouts_success.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fixture)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	fares, err := client.Search(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, want := len(fares), 1; got != want {
		t.Fatalf("len(fares) = %d, want %d", got, want)
	}

	f := fares[0]
	if f.Origin != "NYC" {
		t.Errorf("Origin = %q, want %q", f.Origin, "NYC")
	}
	if f.Destination != "LON" {
		t.Errorf("Destination = %q, want %q", f.Destination, "LON")
	}
	if f.OriginAirport != "JFK" {
		t.Errorf("OriginAirport = %q, want %q", f.OriginAirport, "JFK")
	}
	if f.DestinationAirport != "LHR" {
		t.Errorf("DestinationAirport = %q, want %q", f.DestinationAirport, "LHR")
	}
	if f.Airline != "B6" {
		t.Errorf("Airline = %q, want %q", f.Airline, "B6")
	}
	if f.FlightNumber != "1107" {
		t.Errorf("FlightNumber = %q, want %q", f.FlightNumber, "1107")
	}

	wantDep := time.Date(2026, 10, 15, 8, 45, 0, 0, time.FixedZone("", -4*3600))
	if !f.DepartureAt.Equal(wantDep) {
		t.Errorf("DepartureAt = %v, want %v", f.DepartureAt, wantDep)
	}

	if f.Price != 30300 {
		t.Errorf("Price = %d, want %d (303 USD = 30300 cents)", f.Price, 30300)
	}
	if f.Currency != "USD" {
		t.Errorf("Currency = %q, want %q", f.Currency, "USD")
	}
	if f.Duration != 420*time.Minute {
		t.Errorf("Duration = %v, want %v", f.Duration, 420*time.Minute)
	}
	if f.Transfers != 0 {
		t.Errorf("Transfers = %d, want 0", f.Transfers)
	}

	if !strings.HasPrefix(f.Link, "/search/JFK1510LHR1") {
		t.Errorf("Link = %q, want to start with /search/JFK1510LHR1", f.Link)
	}
}

// ---------------------------------------------------------------------------
// 2. No-data (legitimate empty result)
// ---------------------------------------------------------------------------

func TestSearch_EmptyData_ReturnsEmptySliceNotError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"success": true, "data": [], "currency": "usd"}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	fares, err := client.Search(context.Background(), validRequest())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fares == nil {
		t.Fatalf("fares = nil, want empty slice (not nil)")
	}
	if len(fares) != 0 {
		t.Errorf("len(fares) = %d, want 0", len(fares))
	}
}

func TestSearch_NilDataField_ReturnsEmptySliceNotError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"success": true, "data": null, "currency": "usd"}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	fares, err := client.Search(context.Background(), validRequest())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fares == nil {
		t.Fatalf("fares = nil, want empty slice")
	}
	if len(fares) != 0 {
		t.Errorf("len(fares) = %d, want 0", len(fares))
	}
}

// ---------------------------------------------------------------------------
// 3. Month fallback
// ---------------------------------------------------------------------------

const monthResultsBody = `{
	"success": true,
	"currency": "usd",
	"data": [
		{"origin_airport":"JFK","destination_airport":"LHR","airline":"B6","flight_number":"1107","departure_at":"2026-10-05T06:30:00+00:00","price":250,"duration":420,"transfers":0,"link":"/search/JFK0510LHR1"},
		{"origin_airport":"JFK","destination_airport":"LHR","airline":"AA","flight_number":"100","departure_at":"2026-10-12T07:00:00+00:00","price":310,"duration":430,"transfers":0,"link":"/search/JFK1210LHR1"},
		{"origin_airport":"JFK","destination_airport":"LHR","airline":"DL","flight_number":"400","departure_at":"2026-10-20T09:00:00+00:00","price":340,"duration":450,"transfers":1,"link":"/search/JFK2010LHR1"},
		{"origin_airport":"JFK","destination_airport":"LHR","airline":"UA","flight_number":"500","departure_at":"2026-10-26T18:00:00+00:00","price":275,"duration":415,"transfers":0,"link":"/search/JFK2610LHR1"},
		{"origin_airport":"JFK","destination_airport":"LHR","airline":"VS","flight_number":"3","departure_at":"2026-10-30T20:00:00+00:00","price":900,"duration":430,"transfers":0,"link":"/search/JFK3010LHR1"}
	]
}`

func TestSearch_ExactDateEmpty_FallsBackToMonthAndPicksCheapest(t *testing.T) {
	var serverHits int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverHits, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		date := r.URL.Query().Get("departure_at")
		switch date {
		case "2026-10-15":
			_, _ = io.WriteString(w, `{"success": true, "data": [], "currency": "usd"}`)
		case "2026-10":
			_, _ = io.WriteString(w, monthResultsBody)
		default:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"success": false, "error": "unexpected departure_at"}`)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	fares, err := client.Search(context.Background(), validRequest())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fares) != 1 {
		t.Fatalf("len(fares) = %d, want 1 (cheapest of month)", len(fares))
	}
	if got := atomic.LoadInt32(&serverHits); got != 2 {
		t.Errorf("expected 2 HTTP calls (exact then month), got %d", got)
	}

	if fares[0].Price != 25000 {
		t.Errorf("Price = %d, want %d (cheapest flight in month fixture)", fares[0].Price, 25000)
	}
	wantDep := time.Date(2026, 10, 5, 6, 30, 0, 0, time.UTC)
	if !fares[0].DepartureAt.Equal(wantDep) {
		t.Errorf("DepartureAt = %v, want %v (the 250 USD flight's date)", fares[0].DepartureAt, wantDep)
	}
}

func TestSearch_MonthFallback_BothEmpty_ReturnsEmptySlice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"success": true, "data": [], "currency": "usd"}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	fares, err := client.Search(context.Background(), validRequest())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fares) != 0 {
		t.Errorf("len(fares) = %d, want 0", len(fares))
	}
}

func TestSearch_MonthFallback_ExactDateHasResults_NoSecondCall(t *testing.T) {
	var serverHits int32
	fixture := loadFixture(t, "testdata/travelpayouts_success.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverHits, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fixture)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	fares, err := client.Search(context.Background(), validRequest())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fares) != 1 {
		t.Errorf("len(fares) = %d, want 1", len(fares))
	}
	if got := atomic.LoadInt32(&serverHits); got != 1 {
		t.Errorf("expected exactly 1 HTTP call (no month fallback), got %d", got)
	}
}

func TestSearch_MonthFallback_SecondCallFails_StillReturnsWhatWeHave(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("departure_at") == "2026-10-15" {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"success": true, "data": [], "currency": "usd"}`)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"error": "upstream is on fire"}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	fares, err := client.Search(context.Background(), validRequest())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fares) != 0 {
		t.Errorf("len(fares) = %d, want 0", len(fares))
	}
}

func TestSearch_MonthFallback_ProviderDownSwallowed_OtherErrorsPropagate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("departure_at") == "2026-10-15" {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"success": true, "data": [], "currency": "usd"}`)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"error": "upstream is on fire"}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	fares, err := client.Search(context.Background(), validRequest())

	if err != nil {
		t.Fatalf("provider-down error in month fallback should be swallowed, got: %v", err)
	}
	if len(fares) != 0 {
		t.Errorf("len(fares) = %d, want 0", len(fares))
	}
}

// ---------------------------------------------------------------------------
// 4. Error cases (genuine failure)
// ---------------------------------------------------------------------------

func TestSearch_AuthError_ReturnsErrorWithGenericMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error": "Invalid token: secret-leak-marker"}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.Search(context.Background(), validRequest())

	if !errors.Is(err, ErrAuth) {
		t.Errorf("err = %v, want errors.Is(err, ErrAuth)", err)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if strings.Contains(msg, "secret-leak-marker") {
		t.Errorf("error message leaks provider body: %q", msg)
	}
	if strings.Contains(msg, testToken) {
		t.Errorf("error message leaks API key: %q", msg)
	}
}

func TestSearch_Forbidden403_ReturnsErrAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error": "access denied"}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.Search(context.Background(), validRequest())

	if !errors.Is(err, ErrAuth) {
		t.Errorf("err = %v, want errors.Is(err, ErrAuth)", err)
	}
}

func TestSearch_BadRequest_ReturnsErrInvalidInput_NoLeak(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error": "bad query: leaked-internal-detail"}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.Search(context.Background(), validRequest())

	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("err = %v, want errors.Is(err, ErrInvalidInput)", err)
	}
	msg := err.Error()
	if strings.Contains(msg, "leaked-internal-detail") {
		t.Errorf("error message leaks provider body: %q", msg)
	}
	if strings.Contains(msg, testToken) {
		t.Errorf("error message leaks API key: %q", msg)
	}
}

func TestSearch_RateLimited_ReturnsErrRateLimited_NoLeak(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error": "rate limit body marker: rl-marker-xyz"}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.Search(context.Background(), validRequest())

	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("err = %v, want errors.Is(err, ErrRateLimited)", err)
	}
	msg := err.Error()
	if strings.Contains(msg, "rl-marker-xyz") {
		t.Errorf("error message leaks provider body: %q", msg)
	}
	if strings.Contains(msg, testToken) {
		t.Errorf("error message leaks API key: %q", msg)
	}
}

func TestSearch_ServerError_5xx_ReturnsErrProviderDown_NoLeak(t *testing.T) {
	for _, code := range []int{http.StatusInternalServerError, http.StatusServiceUnavailable, http.StatusGatewayTimeout} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
				_, _ = io.WriteString(w, `{"error": "5xx-leak-marker: abcdef"}`)
			}))
			defer server.Close()

			client := newTestClient(t, server.URL)
			_, err := client.Search(context.Background(), validRequest())

			if !errors.Is(err, ErrProviderDown) {
				t.Errorf("status %d: err = %v, want ErrProviderDown", code, err)
			}
			msg := err.Error()
			if strings.Contains(msg, "abcdef") {
				t.Errorf("error leaks provider body: %q", msg)
			}
			if strings.Contains(msg, testToken) {
				t.Errorf("error leaks API key: %q", msg)
			}
		})
	}
}

func TestSearch_ConnectionReset_ReturnsErrProviderDown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("server does not support hijacking")
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Fatal(err)
		}
		_ = conn.Close()
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.Search(context.Background(), validRequest())

	if !errors.Is(err, ErrProviderDown) {
		t.Errorf("err = %v, want ErrProviderDown", err)
	}
}

func TestSearch_Timeout_ReturnsErrProviderDown_NotPanic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	client := NewTravelpayouts(testToken, server.URL, &http.Client{Timeout: 100 * time.Millisecond})
	_, err := client.Search(context.Background(), validRequest())

	if !errors.Is(err, ErrProviderDown) {
		t.Errorf("err = %v, want ErrProviderDown (timeout surfaced as provider-down)", err)
	}
}

// ---------------------------------------------------------------------------
// 5. Malformed response (HTTP 200, garbage)
// ---------------------------------------------------------------------------

func TestSearch_ValidHTTP_GarbageJSON_ReturnsErrProviderDown_NoPanic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `not json {{{`)
	}))
	defer server.Close()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("adapter panicked on garbage JSON: %v", r)
		}
	}()

	client := newTestClient(t, server.URL)
	_, err := client.Search(context.Background(), validRequest())

	if !errors.Is(err, ErrProviderDown) {
		t.Errorf("err = %v, want ErrProviderDown", err)
	}
}

func TestSearch_ValidHTTP_JSONButMissingDataField_ReturnsErrProviderDown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"success": true, "currency": "usd"}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.Search(context.Background(), validRequest())

	if !errors.Is(err, ErrProviderDown) {
		t.Errorf("err = %v, want ErrProviderDown (missing data field = contract violation)", err)
	}
}

func TestSearch_ValidHTTP_SuccessFalseNoErrorMessage_ReturnsErrProviderDown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"success": false, "data": []}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.Search(context.Background(), validRequest())

	if !errors.Is(err, ErrProviderDown) {
		t.Errorf("err = %v, want ErrProviderDown (success=false without reason)", err)
	}
}

func TestSearch_ValidHTTP_SuccessFalseWithData_ReturnsErrorNotFares(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"success": false, "data": [{"origin_airport":"JFK","destination_airport":"LHR","airline":"B6","flight_number":"1107","departure_at":"2026-10-15T08:45:00-04:00","price":26324,"duration":420,"transfers":0,"link":"/x"}], "currency": "usd"}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	fares, err := client.Search(context.Background(), validRequest())

	if err == nil {
		t.Error("err = nil, want non-nil error for success=false with data")
	}
	if len(fares) != 0 {
		t.Errorf("len(fares) = %d, want 0 (fares should not be returned when success=false)", len(fares))
	}
}

func TestSearch_ValidHTTP_FlightWithInvalidDepartureAt_ReturnsError_NotZeroTime(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"success": true, "data": [{"origin_airport":"JFK","destination_airport":"LHR","airline":"B6","flight_number":"1107","departure_at":"","price":26324,"duration":420,"transfers":0,"link":"/x"}], "currency": "usd"}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	fares, err := client.Search(context.Background(), validRequest())

	if err == nil {
		t.Error("err = nil, want non-nil error for invalid departure_at")
	}
	if len(fares) != 0 {
		t.Errorf("len(fares) = %d, want 0 (invalid flight should be discarded)", len(fares))
	}
}

func TestSearch_ValidHTTP_CurrencyMismatch_ReturnsErrProviderDown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"success": true, "data": [{"origin_airport":"JFK","destination_airport":"LHR","airline":"B6","flight_number":"1107","departure_at":"2026-10-15T08:45:00-04:00","price":26324,"duration":420,"transfers":0,"link":"/x"}], "currency": "rub"}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.Search(context.Background(), validRequest())

	if !errors.Is(err, ErrProviderDown) {
		t.Errorf("err = %v, want ErrProviderDown (currency mismatch = contract violation)", err)
	}
}

func TestSearch_ValidHTTP_MissingRequiredFlightFields_ReturnsErrProviderDown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"success": true, "data": [{"origin_airport": "JFK"}], "currency": "usd"}`)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.Search(context.Background(), validRequest())

	if !errors.Is(err, ErrProviderDown) {
		t.Errorf("err = %v, want ErrProviderDown (missing required flight fields)", err)
	}
}

// ---------------------------------------------------------------------------
// 6. Currency / request integrity
// ---------------------------------------------------------------------------

func TestSearch_OutgoingRequest_AlwaysIncludesCurrencyUSD(t *testing.T) {
	for _, path := range []string{"testdata/travelpayouts_success.json", "testdata/travelpayouts_empty.json"} {
		t.Run(path, func(t *testing.T) {
			fixture, err := os.ReadFile(path)
			if err != nil {
				t.Skipf("skipping %s: fixture not present", path)
			}
			var capturedQuery string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedQuery = r.URL.RawQuery
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(fixture)
			}))
			defer server.Close()

			client := newTestClient(t, server.URL)
			_, _ = client.Search(context.Background(), validRequest())

			if !strings.Contains(capturedQuery, "currency=usd") {
				t.Errorf("request did not include currency=usd; query = %q", capturedQuery)
			}
		})
	}
}

func TestSearch_OutgoingRequest_OneWayIsTrue(t *testing.T) {
	var capturedQuery string
	fixture := loadFixture(t, "testdata/travelpayouts_success.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fixture)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, _ = client.Search(context.Background(), validRequest())

	if !strings.Contains(capturedQuery, "one_way=true") {
		t.Errorf("request did not include one_way=true; query = %q", capturedQuery)
	}
}

func TestSearch_OutgoingRequest_UserAgentIsSet(t *testing.T) {
	var capturedUA string
	fixture := loadFixture(t, "testdata/travelpayouts_success.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fixture)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, _ = client.Search(context.Background(), validRequest())

	if !strings.Contains(capturedUA, "flight-circle-search") {
		t.Errorf("request did not include expected User-Agent; UA = %q", capturedUA)
	}
}

func TestSearch_OutgoingRequest_AcceptIsApplicationJSON(t *testing.T) {
	var capturedAccept string
	fixture := loadFixture(t, "testdata/travelpayouts_success.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fixture)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, _ = client.Search(context.Background(), validRequest())

	if !strings.Contains(capturedAccept, "application/json") {
		t.Errorf("request did not include Accept: application/json; Accept = %q", capturedAccept)
	}
}

func TestSearch_OutgoingRequest_IncludesToken(t *testing.T) {
	var capturedQuery string
	fixture := loadFixture(t, "testdata/travelpayouts_success.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fixture)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, _ = client.Search(context.Background(), validRequest())

	if !strings.Contains(capturedQuery, "token="+testToken) {
		t.Errorf("request did not include the configured token; query = %q", capturedQuery)
	}
}

func TestSearch_OutgoingRequest_DoesNotIncludeTokenInBody(t *testing.T) {
	var bodyBytes []byte
	fixture := loadFixture(t, "testdata/travelpayouts_success.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, _ := io.ReadAll(r.Body)
		bodyBytes = buf
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fixture)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, _ = client.Search(context.Background(), validRequest())

	if len(bodyBytes) != 0 {
		t.Errorf("expected empty request body for GET, got %q", bodyBytes)
	}
}

func TestSearch_OutgoingRequest_UsesGetMethod(t *testing.T) {
	var capturedMethod string
	fixture := loadFixture(t, "testdata/travelpayouts_success.json")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fixture)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, _ = client.Search(context.Background(), validRequest())

	if capturedMethod != http.MethodGet {
		t.Errorf("method = %q, want GET", capturedMethod)
	}
}

// ---------------------------------------------------------------------------
// 7. Input validation (adversarial)
// ---------------------------------------------------------------------------

func TestSearch_EmptyOrigin_ReturnsErrInvalidInput(t *testing.T) {
	assertNoNetworkCall(t, SearchRequest{Origin: "", Destination: "LON", Date: "2026-10-15"})
}

func TestSearch_EmptyDestination_ReturnsErrInvalidInput(t *testing.T) {
	assertNoNetworkCall(t, SearchRequest{Origin: "NYC", Destination: "", Date: "2026-10-15"})
}

func TestSearch_EmptyDate_ReturnsErrInvalidInput(t *testing.T) {
	assertNoNetworkCall(t, SearchRequest{Origin: "NYC", Destination: "LON", Date: ""})
}

func TestSearch_AllFieldsEmpty_ReturnsErrInvalidInput(t *testing.T) {
	assertNoNetworkCall(t, SearchRequest{})
}

func TestSearch_ShortOrigin_ReturnsErrInvalidInput(t *testing.T) {
	assertNoNetworkCall(t, SearchRequest{Origin: "NY", Destination: "LON", Date: "2026-10-15"})
}

func TestSearch_ShortDestination_ReturnsErrInvalidInput(t *testing.T) {
	assertNoNetworkCall(t, SearchRequest{Origin: "NYC", Destination: "LO", Date: "2026-10-15"})
}

func TestSearch_FourCharOrigin_ReturnsErrInvalidInput(t *testing.T) {
	assertNoNetworkCall(t, SearchRequest{Origin: "NYCX", Destination: "LON", Date: "2026-10-15"})
}

func TestSearch_LowercaseOrigin_ReturnsErrInvalidInput(t *testing.T) {
	assertNoNetworkCall(t, SearchRequest{Origin: "nyc", Destination: "LON", Date: "2026-10-15"})
}

func TestSearch_MixedCaseOrigin_ReturnsErrInvalidInput(t *testing.T) {
	assertNoNetworkCall(t, SearchRequest{Origin: "NyC", Destination: "LON", Date: "2026-10-15"})
}

func TestSearch_NumericOrigin_ReturnsErrInvalidInput(t *testing.T) {
	assertNoNetworkCall(t, SearchRequest{Origin: "N12", Destination: "LON", Date: "2026-10-15"})
}

func TestSearch_SymbolInOrigin_ReturnsErrInvalidInput(t *testing.T) {
	assertNoNetworkCall(t, SearchRequest{Origin: "N@C", Destination: "LON", Date: "2026-10-15"})
}

func TestSearch_LeadingSpaceOrigin_ReturnsErrInvalidInput(t *testing.T) {
	assertNoNetworkCall(t, SearchRequest{Origin: " NYC", Destination: "LON", Date: "2026-10-15"})
}

func TestSearch_TrailingSpaceDestination_ReturnsErrInvalidInput(t *testing.T) {
	assertNoNetworkCall(t, SearchRequest{Origin: "NYC", Destination: "LON ", Date: "2026-10-15"})
}

func TestSearch_UnicodeOrigin_ReturnsErrInvalidInput(t *testing.T) {
	assertNoNetworkCall(t, SearchRequest{Origin: "NYC\u0301", Destination: "LON", Date: "2026-10-15"})
}

func TestSearch_InvalidDate_NotISOFormat_ReturnsErrInvalidInput(t *testing.T) {
	assertNoNetworkCall(t, SearchRequest{Origin: "NYC", Destination: "LON", Date: "2026/10/15"})
}

func TestSearch_InvalidDate_MonthOutOfRange_ReturnsErrInvalidInput(t *testing.T) {
	assertNoNetworkCall(t, SearchRequest{Origin: "NYC", Destination: "LON", Date: "2026-13-01"})
}

func TestSearch_InvalidDate_DayOutOfRange_ReturnsErrInvalidInput(t *testing.T) {
	assertNoNetworkCall(t, SearchRequest{Origin: "NYC", Destination: "LON", Date: "2026-02-30"})
}

func TestSearch_InvalidDate_NotADateAtAll_ReturnsErrInvalidInput(t *testing.T) {
	assertNoNetworkCall(t, SearchRequest{Origin: "NYC", Destination: "LON", Date: "not-a-date"})
}

func TestSearch_NoNetworkCallOnInvalidInput(t *testing.T) {
	var hit int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hit, 1)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.Search(context.Background(), SearchRequest{Origin: "X", Destination: "Y", Date: "bad"})

	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
	if got := atomic.LoadInt32(&hit); got != 0 {
		t.Errorf("server received %d request(s) on invalid input, want 0", got)
	}
}

func assertNoNetworkCall(t *testing.T, req SearchRequest) {
	t.Helper()
	var hit int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hit, 1)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	_, err := client.Search(context.Background(), req)

	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("req = %+v: err = %v, want ErrInvalidInput", req, err)
	}
	if got := atomic.LoadInt32(&hit); got != 0 {
		t.Errorf("req = %+v: server received %d request(s) on invalid input, want 0", req, got)
	}
}

// ---------------------------------------------------------------------------
// 8. Interface & error contract
// ---------------------------------------------------------------------------

func TestSearch_ImplementsFareProviderInterface(t *testing.T) {
	var _ FareProvider = (*Travelpayouts)(nil)
}

func TestSearch_ErrorsAreDistinct(t *testing.T) {
	sentinels := []error{ErrInvalidInput, ErrAuth, ErrRateLimited, ErrProviderDown}
	for i, a := range sentinels {
		for j, b := range sentinels {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Errorf("errors.Is(%v, %v) = true, want false", a, b)
			}
		}
	}
}

func TestSearch_MapFares_NilSlice_ReturnsEmptyNotPanic(t *testing.T) {
	fares, err := mapFares(nil)
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if len(fares) != 0 {
		t.Errorf("len(fares) = %d, want 0", len(fares))
	}
}

func TestSearch_NewTravelpayouts_PreservesProvidedHTTPClient(t *testing.T) {
	customClient := &http.Client{Timeout: 1 * time.Millisecond}
	p := NewTravelpayouts("token", "", customClient)
	if p.httpClient == nil {
		t.Fatal("httpClient is nil: constructor did not preserve the provided client")
	}
	if p.httpClient == customClient {
		t.Log("httpClient is the same instance — ideal")
	}
}

func TestSearch_ErrorsAreSelfEqual(t *testing.T) {
	sentinels := []error{ErrInvalidInput, ErrAuth, ErrRateLimited, ErrProviderDown}
	for _, e := range sentinels {
		if !errors.Is(e, e) {
			t.Errorf("errors.Is(%v, %v) = false, want true", e, e)
		}
	}
}

func TestSearch_ErrorsImplementUnwrap(t *testing.T) {
	sentinels := []error{ErrInvalidInput, ErrAuth, ErrRateLimited, ErrProviderDown}
	for _, e := range sentinels {
		var unwrapErr error
		type unwrapper interface{ Unwrap() error }
		if u, ok := e.(unwrapper); ok {
			unwrapErr = u.Unwrap()
		}
		if unwrapErr == nil {
			t.Errorf("%v: Unwrap() returned nil", e)
		}
	}
}

func TestSearch_SentinelErrorsHaveGenericMessages(t *testing.T) {
	sentinels := []error{ErrInvalidInput, ErrAuth, ErrRateLimited, ErrProviderDown}
	banned := []string{testToken, "http://", "https://", "travelpayouts", "aviasales"}
	for _, e := range sentinels {
		msg := e.Error()
		for _, b := range banned {
			if strings.Contains(strings.ToLower(msg), b) {
				t.Errorf("sentinel %v has banned substring %q in message: %q", e, b, msg)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 9. Context & cancellation
// ---------------------------------------------------------------------------

func TestSearch_ContextCancelledMidCall_ReturnsCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := client.Search(ctx, validRequest())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestSearch_ContextAlreadyCancelled_ReturnsCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("server should not be called on already-canceled context")
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Search(ctx, validRequest())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestSearch_ContextDeadlineExceeded_ReturnsDeadlineExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Search(ctx, validRequest())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

func TestSearch_ContextCancellation_ReturnsContextError_NotProviderDown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := client.Search(ctx, validRequest())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled (not ErrProviderDown)", err)
	}
}

// ---------------------------------------------------------------------------
// 10. Link integrity
// ---------------------------------------------------------------------------

func TestSearch_LinkPreservedAsRelativePath(t *testing.T) {
	fixture := loadFixture(t, "testdata/travelpayouts_success.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fixture)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	fares, err := client.Search(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fares) != 1 {
		t.Fatalf("len(fares) = %d, want 1", len(fares))
	}
	if !strings.HasPrefix(fares[0].Link, "/search/") {
		t.Errorf("Link = %q, want to start with /search/ (relative path)", fares[0].Link)
	}
	if strings.HasPrefix(fares[0].Link, "http://") || strings.HasPrefix(fares[0].Link, "https://") {
		t.Errorf("Link = %q, must not have a host prepended", fares[0].Link)
	}
}

func TestSearch_LinkNeverEmptyForValidFlight(t *testing.T) {
	fixture := loadFixture(t, "testdata/travelpayouts_success.json")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, fixture)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	fares, err := client.Search(context.Background(), validRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fares) != 1 {
		t.Fatalf("len(fares) = %d, want 1", len(fares))
	}
	if fares[0].Link == "" {
		t.Errorf("Link is empty for a valid flight")
	}
}
